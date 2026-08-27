// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js

// play-from-disk-mpegts sends H.264 or H.265 video from an MPEG-TS file over WebRTC.
package main

import (
	"crypto/rand"
	"embed"
	"encoding/binary"
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
	"github.com/pion/rtp"
	"github.com/pion/rtp/codecs"
	"github.com/pion/webrtc/v4"
)

const (
	defaultInputFileName = "output.ts"
	videoClockRate       = 90000
	videoMTU             = 1200
	connectionTimeout    = 30 * time.Second
)

var (
	errNoVideoTrack      = errors.New("MPEG-TS file contains no supported video track")
	errConnectionTimeout = errors.New("timed out waiting for ICE connection")
)

//go:embed web/*
var content embed.FS

type mediaSource struct {
	file      *os.File
	reader    *mpegts.Reader
	track     *mpegts.Track
	mimeType  string
	payloader rtp.Payloader
}

func main() {
	addr := flag.String("addr", "localhost:8080", "HTTP listen address")
	input := flag.String("input", defaultInputFileName, "MPEG-TS file to play")
	flag.Parse()

	source, err := openMediaSource(*input)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Loaded %s (%s, PID %d)", *input, source.track.Codec, source.track.PID)
	if err = source.close(); err != nil {
		log.Printf("close input after probe: %v", err)
	}

	static, err := fs.Sub(content, "web")
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(static)))
	mux.HandleFunc("/whep", func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
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

		answer, offerErr := handleOffer(*input, webrtc.SessionDescription{
			Type: webrtc.SDPTypeOffer,
			SDP:  rawSDP,
		}) //nolint:contextcheck // WebRTC signaling API has no context parameter.
		if offerErr != nil {
			log.Printf("handle offer: %v", offerErr)
			http.Error(writer, offerErr.Error(), http.StatusBadRequest)

			return
		}

		writer.Header().Set("Content-Type", "application/sdp")
		if _, writeErr := writer.Write([]byte(answer.SDP)); writeErr != nil {
			log.Printf("write answer: %v", writeErr)
		}
	})

	log.Printf("Serving UI at http://%s", *addr)
	log.Fatal(http.ListenAndServe(*addr, mux)) //nolint:gosec
}

func handleOffer(input string, offer webrtc.SessionDescription) (*webrtc.SessionDescription, error) { //nolint:cyclop
	source, err := openMediaSource(input)
	if err != nil {
		return nil, err
	}

	peerConnection, err := webrtc.NewPeerConnection(webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{{URLs: []string{"stun:stun.l.google.com:19302"}}},
	})
	if err != nil {
		_ = source.close()

		return nil, fmt.Errorf("create PeerConnection: %w", err)
	}

	setupComplete := false
	defer func() {
		if !setupComplete {
			_ = peerConnection.Close()
			_ = source.close()
		}
	}()

	videoTrack, err := webrtc.NewTrackLocalStaticRTP(
		webrtc.RTPCodecCapability{MimeType: source.mimeType, ClockRate: videoClockRate},
		"video",
		"pion",
	)
	if err != nil {
		return nil, fmt.Errorf("create video track: %w", err)
	}
	rtpSender, err := peerConnection.AddTrack(videoTrack)
	if err != nil {
		return nil, fmt.Errorf("add video track: %w", err)
	}
	go drainRTCP(rtpSender)

	connected := make(chan struct{})
	disconnected := make(chan struct{})
	var connectedOnce, disconnectedOnce sync.Once
	peerConnection.OnICEConnectionStateChange(func(state webrtc.ICEConnectionState) {
		log.Printf("ICE connection state: %s", state)
		if state == webrtc.ICEConnectionStateConnected {
			connectedOnce.Do(func() { close(connected) })
		}
	})
	peerConnection.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		log.Printf("Peer connection state: %s", state)
		if state == webrtc.PeerConnectionStateFailed || state == webrtc.PeerConnectionStateClosed {
			disconnectedOnce.Do(func() { close(disconnected) })
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

	go func() {
		defer func() {
			if closeErr := peerConnection.Close(); closeErr != nil {
				log.Printf("close PeerConnection: %v", closeErr)
			}
			if closeErr := source.close(); closeErr != nil {
				log.Printf("close input: %v", closeErr)
			}
		}()

		if playErr := playMPEGTS(connected, disconnected, source, videoTrack); playErr != nil {
			log.Printf("playback stopped: %v", playErr)
		}
	}()

	setupComplete = true

	return peerConnection.LocalDescription(), nil
}

func openMediaSource(input string) (*mediaSource, error) {
	file, err := os.Open(input) //nolint:gosec // The local CLI intentionally accepts a user-selected media path.
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", input, err)
	}
	reader, err := mpegts.NewReader(file)
	if err != nil {
		_ = file.Close()

		return nil, fmt.Errorf("open MPEG-TS reader: %w", err)
	}
	if len(reader.Tracks()) == 0 {
		_ = file.Close()

		return nil, errNoVideoTrack
	}

	track := reader.Tracks()[0]
	source := &mediaSource{file: file, reader: reader, track: track}
	switch track.Codec {
	case mpegts.CodecH264:
		source.mimeType = webrtc.MimeTypeH264
		source.payloader = &codecs.H264Payloader{}
	case mpegts.CodecH265:
		source.mimeType = webrtc.MimeTypeH265
		source.payloader = &codecs.H265Payloader{}
	default:
		_ = file.Close()

		return nil, fmt.Errorf("unsupported MPEG-TS codec %s", track.Codec) //nolint:err113
	}

	return source, nil
}

func (source *mediaSource) close() error {
	return source.file.Close()
}

func playMPEGTS( //nolint:cyclop
	connected <-chan struct{},
	disconnected <-chan struct{},
	source *mediaSource,
	videoTrack *webrtc.TrackLocalStaticRTP,
) error {
	select {
	case <-connected:
		log.Printf("Playing %s (PID %d)", source.track.Codec, source.track.PID)
	case <-disconnected:
		return nil
	case <-time.After(connectionTimeout):
		return errConnectionTimeout
	}

	current, err := nextAccessUnit(source.reader, source.track.PID)
	if err != nil {
		return fmt.Errorf("read first access unit: %w", err)
	}
	firstDTS := current.DTS
	playbackStart := time.Now()
	var timestampSeed [4]byte
	if _, err = rand.Read(timestampSeed[:]); err != nil {
		return fmt.Errorf("generate RTP timestamp: %w", err)
	}
	timestampBase := binary.BigEndian.Uint32(timestampSeed[:])
	packetizer := rtp.NewPacketizer(
		videoMTU, 0, 0, source.payloader, rtp.NewRandomSequencer(), videoClockRate,
	)

	for {
		// NextAccessUnit reuses its output buffer, so retain this unit before reading ahead.
		data := append([]byte(nil), current.Data...)
		next, nextErr := nextAccessUnit(source.reader, source.track.PID)
		duration := time.Second / 30
		if nextErr == nil {
			ticks := next.DTS - current.DTS
			if ticks > 0 {
				nextOffset := time.Duration(next.DTS-firstDTS) * time.Second / videoClockRate
				duration = max(time.Until(playbackStart.Add(nextOffset)), 0)
			}
		} else if !errors.Is(nextErr, io.EOF) {
			return fmt.Errorf("read access unit: %w", nextErr)
		}

		// MPEG-TS access units are in decode order, while RTP timestamps carry
		// presentation time. Preserving PTS is required for streams with B-frames.
		timestamp := timestampBase + uint32(current.PTS-firstDTS) //nolint:gosec
		packets := packetizer.Packetize(data, 0)
		for _, packet := range packets {
			packet.Timestamp = timestamp
			if writeErr := videoTrack.WriteRTP(packet); writeErr != nil {
				return fmt.Errorf("write RTP: %w", writeErr)
			}
		}

		timer := time.NewTimer(duration)
		select {
		case <-timer.C:
		case <-disconnected:
			timer.Stop()

			return nil
		}
		if errors.Is(nextErr, io.EOF) {
			log.Println("All MPEG-TS access units sent")

			return nil
		}
		current = next
	}
}

func nextAccessUnit(reader *mpegts.Reader, pid uint16) (*mpegts.AccessUnit, error) {
	for {
		accessUnit, err := reader.NextAccessUnit()
		if err != nil {
			return nil, err
		}
		if accessUnit.Track.PID == pid {
			return accessUnit, nil
		}
	}
}

func drainRTCP(sender *webrtc.RTPSender) {
	buffer := make([]byte, 1500)
	for {
		if _, _, err := sender.Read(buffer); err != nil {
			return
		}
	}
}
