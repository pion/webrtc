// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js

// play-from-disk-mpegts sends H.264 or H.265 video from an MPEG-TS file over WebRTC.
package main

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/pion/format/mpegts"
	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"
)

const inputFileName = "output.ts"

func main() { //nolint:gocognit,cyclop
	file, err := os.Open(inputFileName)
	if err != nil {
		panic(fmt.Sprintf("open %s: %v", inputFileName, err))
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			fmt.Printf("cannot close %s: %v\n", inputFileName, closeErr)
		}
	}()

	reader, err := mpegts.NewReader(file)
	if err != nil {
		panic(err)
	}
	if len(reader.Tracks()) == 0 {
		panic("MPEG-TS file contains no supported video track")
	}
	containerTrack := reader.Tracks()[0]
	var mimeType string
	switch containerTrack.Codec {
	case mpegts.CodecH264:
		mimeType = webrtc.MimeTypeH264
	case mpegts.CodecH265:
		mimeType = webrtc.MimeTypeH265
	default:
		panic(fmt.Sprintf("unsupported MPEG-TS codec %s", containerTrack.Codec))
	}

	peerConnection, err := webrtc.NewPeerConnection(webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{{URLs: []string{"stun:stun.l.google.com:19302"}}},
	})
	if err != nil {
		panic(err)
	}
	defer func() {
		if closeErr := peerConnection.Close(); closeErr != nil {
			fmt.Printf("cannot close peer connection: %v\n", closeErr)
		}
	}()

	videoTrack, err := webrtc.NewTrackLocalStaticSample(
		webrtc.RTPCodecCapability{MimeType: mimeType, ClockRate: 90000}, "video", "pion",
	)
	if err != nil {
		panic(err)
	}
	rtpSender, err := peerConnection.AddTrack(videoTrack)
	if err != nil {
		panic(err)
	}
	go func() {
		buffer := make([]byte, 1500)
		for {
			if _, _, readErr := rtpSender.Read(buffer); readErr != nil {
				return
			}
		}
	}()

	connected, markConnected := context.WithCancel(context.Background())
	peerConnection.OnICEConnectionStateChange(func(state webrtc.ICEConnectionState) {
		fmt.Printf("ICE connection state: %s\n", state)
		if state == webrtc.ICEConnectionStateConnected {
			markConnected()
		}
	})

	playbackDone := make(chan struct{})
	go func() {
		defer close(playbackDone)
		<-connected.Done()
		fmt.Printf("Playing %s (%s, PID %d)\n", inputFileName, containerTrack.Codec, containerTrack.PID)

		current, readErr := reader.NextAccessUnit()
		if readErr != nil {
			panic(readErr)
		}
		for {
			// NextAccessUnit reuses its output buffer, so retain this unit before reading ahead.
			data := append([]byte(nil), current.Data...)
			next, nextErr := reader.NextAccessUnit()
			duration := time.Second / 30
			if nextErr == nil {
				ticks := next.DTS - current.DTS
				if ticks > 0 {
					duration = time.Duration(ticks) * time.Second / 90000
				}
			} else if !errors.Is(nextErr, io.EOF) {
				panic(nextErr)
			}

			if writeErr := videoTrack.WriteSample(media.Sample{Data: data, Duration: duration}); writeErr != nil {
				panic(writeErr)
			}
			if errors.Is(nextErr, io.EOF) {
				fmt.Println("All MPEG-TS access units sent")

				return
			}
			time.Sleep(duration)
			current = next
		}
	}()

	offer := webrtc.SessionDescription{}
	decode(readUntilNewline(), &offer)
	if err = peerConnection.SetRemoteDescription(offer); err != nil {
		panic(err)
	}
	answer, err := peerConnection.CreateAnswer(nil)
	if err != nil {
		panic(err)
	}
	gatherComplete := webrtc.GatheringCompletePromise(peerConnection)
	if err = peerConnection.SetLocalDescription(answer); err != nil {
		panic(err)
	}
	<-gatherComplete
	fmt.Println(encode(peerConnection.LocalDescription()))

	<-playbackDone
}

func readUntilNewline() string {
	reader := bufio.NewReader(os.Stdin)
	for {
		line, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			panic(err)
		}
		if line = strings.TrimSpace(line); line != "" {
			return line
		}
	}
}

func encode(description *webrtc.SessionDescription) string {
	data, err := json.Marshal(description)
	if err != nil {
		panic(err)
	}

	return base64.StdEncoding.EncodeToString(data)
}

func decode(input string, description *webrtc.SessionDescription) {
	data, err := base64.StdEncoding.DecodeString(input)
	if err != nil {
		panic(err)
	}
	if err = json.Unmarshal(data, description); err != nil {
		panic(err)
	}
}
