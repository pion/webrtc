// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build opusred && !js

package webrtc_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pion/webrtc/v4/internal/opusredlab"
	"github.com/stretchr/testify/require"
)

// The standalone child embeds dependency build metadata, unlike Go test binaries.
// Build it with -race even for a normal test invocation so the actual RTP trial
// is instrumented, not just the HTTP client driving it.
func TestOpusRED_NoLoss(t *testing.T) {
	repoRoot, err := os.Getwd()
	require.NoError(t, err)
	binary := filepath.Join(t.TempDir(), "opus-red-testing")
	buildCtx, buildCancel := context.WithTimeout(context.Background(), 90*time.Second)
	build := exec.CommandContext(buildCtx, "go", "build", "-race", "-modfile=go.red.mod", "-tags=opusred", "-o", binary, "./examples/opus-red-testing")
	build.Dir = repoRoot
	buildOutput, buildErr := build.CombinedOutput()
	buildCancel()
	require.NoError(t, buildErr, "%s", buildOutput)
	artifactBase := filepath.Join(repoRoot, ".opus-red-artifacts")
	require.NoError(t, os.MkdirAll(artifactBase, 0o700))
	artifactDir, err := os.MkdirTemp(artifactBase, "test01-")
	require.NoError(t, err)
	t.Logf("Trial evidence and audio: %s", artifactDir)
	processCtx, processCancel := context.WithTimeout(context.Background(), 120*time.Second)
	process := exec.CommandContext(processCtx, binary, "-listen=127.0.0.1:0", "-artifacts="+artifactDir)
	process.Dir = repoRoot
	var logs lockedLog
	process.Stderr = &logs
	stdout, err := process.StdoutPipe()
	require.NoError(t, err)
	require.NoError(t, process.Start())
	ready := make(chan string, 1)
	scanDone := make(chan struct{})
	go func() {
		defer close(scanDone)
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			_, _ = fmt.Fprintln(&logs, line)
			if address, ok := strings.CutPrefix(line, "Opus RED Test 01: "); ok {
				select {
				case ready <- address:
				default:
				}
			}
		}
	}()
	processDone := make(chan error, 1)
	go func() { processDone <- process.Wait() }()
	t.Cleanup(func() {
		defer processCancel()
		_ = process.Process.Signal(os.Interrupt)
		select {
		case processErr := <-processDone:
			require.NoError(t, processErr, "controller exit: %s", logs.String())
		case <-time.After(6 * time.Second):
			_ = process.Process.Kill()
			<-processDone
			t.Errorf("controller did not shut down: %s", logs.String())
		}
		<-scanDone
	})
	var address string
	select {
	case address = <-ready:
	case <-time.After(35 * time.Second):
		t.Fatalf("controller did not start: %s", logs.String())
	}
	client := &http.Client{Timeout: 35 * time.Second}
	statusRaw := readResponse(t, client, http.MethodGet, address+"/api/status", nil)
	var status struct {
		Ready   bool
		Blocked string
	}
	require.NoError(t, json.Unmarshal(statusRaw, &status))
	require.True(t, status.Ready, "qualification prerequisite blocked: %s", status.Blocked)
	var fixtureSHA string
	for _, trial := range []struct{ name, mode string }{{"Plain", "plain"}, {"RED", "red"}} {
		t.Run(trial.name, func(t *testing.T) {
			body := strings.NewReader(`{"mode":"` + trial.mode + `"}`)
			raw := readResponse(t, client, http.MethodPost, address+"/api/run", body)
			var result opusredlab.Result
			require.NoError(t, json.Unmarshal(raw, &result))
			t.Logf("%s evidence: %s", trial.name, result.ArtifactDir)
			require.Equal(t, "PASS", result.Status, "%s", raw)
			require.Empty(t, result.Errors)
			require.NotEmpty(t, result.Assertions)
			for _, assertion := range result.Assertions {
				require.True(t, assertion.Passed, "%s: %s", assertion.Name, assertion.Detail)
			}
			require.Len(t, result.SenderEvidence.Original, 300)
			require.Len(t, result.SenderEvidence.Sent, 300)
			require.Len(t, result.ReceiverEvidence.Received, 300)
			require.Len(t, result.Output, 300)
			require.True(t, result.Teardown.Success)
			require.Equal(t, result.Audio.Reference.PCMSHA256, result.Audio.Received.PCMSHA256)
			require.Equal(t, result.Audio.Reference.DurationSeconds, result.Audio.Received.DurationSeconds)
			require.Greater(t, result.Audio.Received.RMS, 0.0)
			if fixtureSHA == "" {
				fixtureSHA = result.FixtureSHA
			}
			require.Equal(t, fixtureSHA, result.FixtureSHA, "both trials must use identical encoded source packets")
			exported := readResponse(t, client, http.MethodGet, address+"/api/export?mode="+trial.mode, nil)
			var exportedResult opusredlab.Result
			require.NoError(t, json.Unmarshal(exported, &exportedResult))
			require.Equal(t, result, exportedResult, "export must contain this trial's exact evidence")
			for _, kind := range []string{"reference", "received"} {
				wav := readResponse(t, client, http.MethodGet, address+"/api/audio?mode="+trial.mode+"&kind="+kind, nil)
				require.Greater(t, len(wav), 44)
				require.Equal(t, "RIFF", string(wav[:4]))
			}
		})
	}
}

func readResponse(t *testing.T, client *http.Client, method, address string, body io.Reader) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, method, address, body)
	require.NoError(t, err)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := client.Do(request)
	require.NoError(t, err)
	defer response.Body.Close()
	raw, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.StatusCode, "%s", raw)
	return raw
}

type lockedLog struct {
	mu   sync.Mutex
	data bytes.Buffer
}

func (log *lockedLog) Write(data []byte) (int, error) {
	log.mu.Lock()
	defer log.mu.Unlock()
	return log.data.Write(data)
}

func (log *lockedLog) String() string {
	log.mu.Lock()
	defer log.mu.Unlock()
	return log.data.String()
}
