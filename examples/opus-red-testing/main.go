// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build opusred && !js

// opus-red-testing controls individual Opus RED qualification trials.
package main

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"time"
)

//go:embed index.html
var assets embed.FS

type outcome struct {
	evidence       json.RawMessage
	fileName       string
	referenceAudio string
	receivedAudio  string
}

type trialRunner func(context.Context, string) (outcome, error)

type controller struct {
	mu          sync.Mutex
	runner      trialRunner
	blocked     string
	busy        bool
	results     map[string]outcome
	artifactDir string
}

func main() {
	listen := flag.String("listen", "127.0.0.1:8088", "loopback HTTP address")
	artifactDir := flag.String("artifacts", ".opus-red-artifacts", "trial evidence and audio directory")
	flag.Parse()
	if err := serve(*listen, *artifactDir); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func serve(address, artifactDir string) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil || net.ParseIP(host) == nil || !net.ParseIP(host).IsLoopback() {
		return fmt.Errorf("listen address must be a loopback IP and port: %s", address)
	}
	absoluteDir, err := filepath.Abs(artifactDir)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(absoluteDir, 0o700); err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	setupCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	runner, setupErr := newTrialRunner(setupCtx, absoluteDir)
	cancel()
	c := &controller{runner: runner, results: make(map[string]outcome), artifactDir: absoluteDir}
	if setupErr != nil {
		c.blocked = setupErr.Error()
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		page, readErr := assets.ReadFile("index.html")
		if readErr != nil {
			http.Error(w, readErr.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(page)
	})
	mux.HandleFunc("GET /api/status", c.status)
	mux.HandleFunc("POST /api/run", c.run)
	mux.HandleFunc("POST /api/reset", c.reset)
	mux.HandleFunc("GET /api/export", c.export)
	mux.HandleFunc("GET /api/audio", c.audio)
	server := &http.Server{
		Addr: address, Handler: sameOrigin(mux), ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout: 5 * time.Second, WriteTimeout: 45 * time.Second,
		BaseContext: func(net.Listener) context.Context { return ctx },
	}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return err
	}
	shutdownDone := make(chan struct{})
	go func() {
		defer close(shutdownDone)
		<-ctx.Done()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		_ = server.Shutdown(shutdownCtx)
	}()
	fmt.Printf("Opus RED Test 01: http://%s\nEvidence: %s\n", listener.Addr(), absoluteDir)
	if c.blocked != "" {
		fmt.Printf("Blocked: %s\n", c.blocked)
	}
	err = server.Serve(listener)
	stop()
	<-shutdownDone
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func sameOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); origin != "" {
			parsed, err := url.Parse(origin)
			if err != nil || parsed.Host != r.Host || parsed.Scheme != "http" {
				http.Error(w, "origin must match the local controller", http.StatusForbidden)
				return
			}
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func (c *controller) status(w http.ResponseWriter, _ *http.Request) {
	c.mu.Lock()
	defer c.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{
		"ready": c.runner != nil && c.blocked == "", "blocked": c.blocked, "busy": c.busy,
		"case": "test01-no-loss", "next": "test02-single-loss", "artifacts": c.artifactDir,
	})
}

func (c *controller) run(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Mode string `json:"mode"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "expected mode: plain or red"})
		return
	}
	if request.Mode != "plain" && request.Mode != "red" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "only Test 01 plain and red are implemented"})
		return
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "expected one JSON request"})
		return
	}
	c.mu.Lock()
	if c.runner == nil || c.blocked != "" {
		message := c.blocked
		c.mu.Unlock()
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "BLOCKED", "error": message})
		return
	}
	if c.busy {
		c.mu.Unlock()
		writeJSON(w, http.StatusConflict, map[string]string{"error": "a trial is already running"})
		return
	}
	c.busy = true
	c.mu.Unlock()
	defer func() { c.mu.Lock(); c.busy = false; c.mu.Unlock() }()
	ctx, cancel := context.WithTimeout(r.Context(), 35*time.Second)
	defer cancel()
	result, runErr := c.runner(ctx, request.Mode)
	if len(result.evidence) > 0 {
		if saveErr := os.WriteFile(filepath.Join(c.artifactDir, result.fileName), result.evidence, 0o600); saveErr != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"status": "FAIL", "error": saveErr.Error()})
			return
		}
		c.mu.Lock()
		c.results[request.Mode] = result
		c.mu.Unlock()
		writeJSON(w, http.StatusOK, result.evidence)
		return
	}
	message := "trial produced no evidence"
	if runErr != nil {
		message = runErr.Error()
	}
	writeJSON(w, http.StatusInternalServerError, map[string]string{"status": "FAIL", "error": message})
}

func (c *controller) reset(w http.ResponseWriter, _ *http.Request) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.busy {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "wait for the current trial to finish"})
		return
	}
	c.results = make(map[string]outcome)
	writeJSON(w, http.StatusOK, map[string]string{"status": "reset"})
}

func (c *controller) latest(mode string) (outcome, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	result, ok := c.results[mode]
	return result, ok
}

func (c *controller) export(w http.ResponseWriter, r *http.Request) {
	result, ok := c.latest(r.URL.Query().Get("mode"))
	if !ok {
		http.Error(w, "run the selected trial first", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", result.fileName))
	writeJSON(w, http.StatusOK, result.evidence)
}

func (c *controller) audio(w http.ResponseWriter, r *http.Request) {
	result, ok := c.latest(r.URL.Query().Get("mode"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	var path string
	switch r.URL.Query().Get("kind") {
	case "reference":
		path = result.referenceAudio
	case "received":
		path = result.receivedAudio
	default:
		http.Error(w, "unknown audio kind", http.StatusBadRequest)
		return
	}
	if path == "" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "audio/wav")
	http.ServeFile(w, r, path)
}
