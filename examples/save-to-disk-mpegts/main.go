// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js

// save-to-disk-mpegts records an H.264 WebRTC track into an MPEG-TS file.
package main

import (
	"embed"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/pion/format/mpegts"
	"github.com/pion/interceptor"
	"github.com/pion/interceptor/pkg/intervalpli"
	"github.com/pion/rtp/codecs"
	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media/samplebuilder"
)

const (
	defaultOutputFileName = "output.ts"
	videoPID              = 256
	maxLateVideoPackets   = 50
	stopTimeout           = 5 * time.Second
)

var (
	errRecordingActive = errors.New("a recording is already active")
	errStopTimeout     = errors.New("timed out waiting for recording to finish")
)

//go:embed web/*
var content embed.FS

type timestampState struct {
	first       uint32
	initialized bool
}

type recordingSession struct {
	peerConnection *webrtc.PeerConnection
	answer         *webrtc.SessionDescription
	done           chan struct{}
	doneOnce       sync.Once
	stopOnce       sync.Once
	mu             sync.Mutex
	trackStarted   bool
	stopping       bool
}

type sessionManager struct {
	mu     sync.Mutex
	active *recordingSession
}

func main() { //nolint:cyclop
	addr := flag.String("addr", "localhost:8080", "HTTP listen address")
	output := flag.String("output", defaultOutputFileName, "MPEG-TS output file")
	flag.Parse()

	static, err := fs.Sub(content, "web")
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	sessions := &sessionManager{}
	mux.Handle("/", http.FileServer(http.FS(static)))
	mux.HandleFunc("/whip", func(writer http.ResponseWriter, request *http.Request) {
		switch request.Method {
		case http.MethodDelete:
			if stopErr := sessions.stop(); stopErr != nil {
				http.Error(writer, stopErr.Error(), http.StatusInternalServerError)

				return
			}
			writer.WriteHeader(http.StatusNoContent)

			return
		case http.MethodPost:
		default:
			http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)

			return
		}

		body, readErr := io.ReadAll(io.LimitReader(request.Body, 1<<20))
		if readErr != nil {
			http.Error(writer, "failed to read offer", http.StatusBadRequest)

			return
		}
		rawSDP := string(body)
		if strings.TrimSpace(rawSDP) == "" {
			http.Error(writer, "empty SDP", http.StatusBadRequest)

			return
		}

		session, offerErr := sessions.create(*output, webrtc.SessionDescription{
			Type: webrtc.SDPTypeOffer,
			SDP:  rawSDP,
		}) //nolint:contextcheck // WebRTC signaling API has no context parameter.
		if offerErr != nil {
			log.Printf("handle offer: %v", offerErr)
			status := http.StatusBadRequest
			if errors.Is(offerErr, errRecordingActive) {
				status = http.StatusConflict
			}
			http.Error(writer, offerErr.Error(), status)

			return
		}

		writer.Header().Set("Content-Type", "application/sdp")
		writer.Header().Set("Location", "/whip")
		writer.WriteHeader(http.StatusCreated)
		if _, writeErr := writer.Write([]byte(session.answer.SDP)); writeErr != nil {
			log.Printf("write answer: %v", writeErr)
		}
	})

	log.Printf("Serving UI at http://%s", *addr)
	log.Fatal(http.ListenAndServe(*addr, mux)) //nolint:gosec
}

func (sessions *sessionManager) create(
	output string,
	offer webrtc.SessionDescription,
) (*recordingSession, error) {
	sessions.mu.Lock()
	defer sessions.mu.Unlock()
	if sessions.active != nil {
		return nil, errRecordingActive
	}

	session, err := handleOffer(output, offer) //nolint:contextcheck
	if err != nil {
		return nil, err
	}
	sessions.active = session
	go func() {
		<-session.done
		sessions.mu.Lock()
		if sessions.active == session {
			sessions.active = nil
		}
		sessions.mu.Unlock()
	}()

	return session, nil
}

func (sessions *sessionManager) stop() error {
	sessions.mu.Lock()
	session := sessions.active
	sessions.mu.Unlock()
	if session == nil {
		return nil
	}

	session.stop()
	select {
	case <-session.done:
		return nil
	case <-time.After(stopTimeout):
		return errStopTimeout
	}
}

func handleOffer(output string, offer webrtc.SessionDescription) (*recordingSession, error) { //nolint:cyclop
	mediaEngine := &webrtc.MediaEngine{}
	if err := mediaEngine.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{
			MimeType:    webrtc.MimeTypeH264,
			ClockRate:   90000,
			SDPFmtpLine: "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42e01f",
		},
		PayloadType: 102,
	}, webrtc.RTPCodecTypeVideo); err != nil {
		return nil, fmt.Errorf("register H.264: %w", err)
	}
	interceptorRegistry := &interceptor.Registry{}
	intervalPLIFactory, err := intervalpli.NewReceiverInterceptor()
	if err != nil {
		return nil, fmt.Errorf("create interval PLI interceptor: %w", err)
	}
	interceptorRegistry.Add(intervalPLIFactory)
	if err = webrtc.RegisterDefaultInterceptors(mediaEngine, interceptorRegistry); err != nil {
		return nil, fmt.Errorf("register interceptors: %w", err)
	}

	api := webrtc.NewAPI(
		webrtc.WithMediaEngine(mediaEngine),
		webrtc.WithInterceptorRegistry(interceptorRegistry),
	)
	peerConnection, err := api.NewPeerConnection(webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{{URLs: []string{"stun:stun.l.google.com:19302"}}},
	})
	if err != nil {
		return nil, fmt.Errorf("create PeerConnection: %w", err)
	}
	session := &recordingSession{
		peerConnection: peerConnection,
		done:           make(chan struct{}),
	}

	setupComplete := false
	defer func() {
		if !setupComplete {
			_ = peerConnection.Close()
		}
	}()

	if _, err = peerConnection.AddTransceiverFromKind(
		webrtc.RTPCodecTypeVideo,
		webrtc.RTPTransceiverInit{Direction: webrtc.RTPTransceiverDirectionRecvonly},
	); err != nil {
		return nil, fmt.Errorf("add video transceiver: %w", err)
	}

	peerConnection.OnTrack(func(track *webrtc.TrackRemote, _ *webrtc.RTPReceiver) {
		if !strings.EqualFold(track.Codec().MimeType, webrtc.MimeTypeH264) {
			log.Printf("Ignoring unsupported track %s", track.Codec().MimeType)

			return
		}
		if !session.beginTrack() {
			return
		}

		if recordErr := recordTrack(output, track); recordErr != nil {
			log.Printf("record track: %v", recordErr)
		}
		session.stop()
		session.finish()
	})

	peerConnection.OnICEConnectionStateChange(func(state webrtc.ICEConnectionState) {
		log.Printf("ICE connection state: %s", state)
	})
	peerConnection.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		log.Printf("Peer connection state: %s", state)
		if state == webrtc.PeerConnectionStateFailed {
			session.stop()
		}
	})

	//nolint:contextcheck // WebRTC signaling API has no context parameter.
	if err = peerConnection.SetRemoteDescription(offer); err != nil {
		return nil, fmt.Errorf("set remote description: %w", err)
	}
	answer, err := peerConnection.CreateAnswer(nil)
	if err != nil {
		return nil, fmt.Errorf("create answer: %w", err)
	}
	gatherComplete := webrtc.GatheringCompletePromise(peerConnection)
	if err = peerConnection.SetLocalDescription(answer); err != nil {
		return nil, fmt.Errorf("set local description: %w", err)
	}
	<-gatherComplete
	session.answer = peerConnection.LocalDescription()
	setupComplete = true

	return session, nil
}

func (session *recordingSession) beginTrack() bool {
	session.mu.Lock()
	defer session.mu.Unlock()
	if session.stopping || session.trackStarted {
		return false
	}
	session.trackStarted = true

	return true
}

func (session *recordingSession) stop() {
	session.mu.Lock()
	session.stopping = true
	trackStarted := session.trackStarted
	session.mu.Unlock()

	session.stopOnce.Do(func() {
		if closeErr := session.peerConnection.Close(); closeErr != nil {
			log.Printf("close PeerConnection: %v", closeErr)
		}
	})
	if !trackStarted {
		session.finish()
	}
}

func (session *recordingSession) finish() {
	session.doneOnce.Do(func() { close(session.done) })
}

func recordTrack(output string, track *webrtc.TrackRemote) error { //nolint:cyclop
	file, err := os.Create(output) //nolint:gosec // The local CLI intentionally accepts a user-selected output path.
	if err != nil {
		return fmt.Errorf("create %s: %w", output, err)
	}
	writer, err := mpegts.NewWriter(file, mpegts.WithH264Track(videoPID))
	if err != nil {
		_ = file.Close()

		return fmt.Errorf("create MPEG-TS writer: %w", err)
	}
	writerClosed := false
	defer func() {
		if !writerClosed {
			if closeErr := writer.Close(); closeErr != nil {
				log.Printf("close %s: %v", output, closeErr)
			}
		}
	}()

	log.Printf("Recording H.264 to %s", output)
	builder := samplebuilder.New(maxLateVideoPackets, &codecs.H264Packet{}, track.Codec().ClockRate)
	timestamps := &timestampState{}

	for {
		packet, _, readErr := track.ReadRTP()
		if readErr != nil {
			builder.Flush()
			if writeErr := writeSamples(builder, writer, timestamps); writeErr != nil {
				return writeErr
			}
			closeErr := writer.Close()
			writerClosed = true
			if closeErr != nil {
				return fmt.Errorf("close %s: %w", output, closeErr)
			}
			if !errors.Is(readErr, io.EOF) && !errors.Is(readErr, io.ErrClosedPipe) {
				return fmt.Errorf("read RTP: %w", readErr)
			}
			log.Printf("Finished writing %s", output)

			return nil
		}
		builder.Push(packet)
		if err = writeSamples(builder, writer, timestamps); err != nil {
			return err
		}
	}
}

func writeSamples(builder *samplebuilder.SampleBuilder, writer *mpegts.Writer, timestamps *timestampState) error {
	for sample := builder.Pop(); sample != nil; sample = builder.Pop() {
		if !timestamps.initialized {
			timestamps.first = sample.PacketTimestamp
			timestamps.initialized = true
		}
		// Unsigned subtraction also handles the 32-bit RTP timestamp wrap.
		timestamp := int64(sample.PacketTimestamp - timestamps.first)
		if err := writer.WriteH264(videoPID, timestamp, timestamp, sample.Data); err != nil {
			return fmt.Errorf("write H.264 access unit: %w", err)
		}
	}

	return nil
}
