// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js

// save-to-disk-mpegts records an H.264 WebRTC track into an MPEG-TS file.
package main

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/pion/format/mpegts"
	"github.com/pion/interceptor"
	"github.com/pion/rtp/codecs"
	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media/samplebuilder"
)

const (
	outputFileName = "output.ts"
	videoPID       = 256
)

func main() { //nolint:gocognit,cyclop
	mediaEngine := &webrtc.MediaEngine{}
	if err := mediaEngine.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{
			MimeType:    webrtc.MimeTypeH264,
			ClockRate:   90000,
			SDPFmtpLine: "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42e01f",
		},
		PayloadType: 102,
	}, webrtc.RTPCodecTypeVideo); err != nil {
		panic(err)
	}
	interceptorRegistry := &interceptor.Registry{}
	if err := webrtc.RegisterDefaultInterceptors(mediaEngine, interceptorRegistry); err != nil {
		panic(err)
	}

	api := webrtc.NewAPI(
		webrtc.WithMediaEngine(mediaEngine),
		webrtc.WithInterceptorRegistry(interceptorRegistry),
	)
	peerConnection, err := api.NewPeerConnection(webrtc.Configuration{
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

	if _, err = peerConnection.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo); err != nil {
		panic(err)
	}

	recordingDone := make(chan struct{})
	peerConnection.OnTrack(func(track *webrtc.TrackRemote, _ *webrtc.RTPReceiver) {
		if !strings.EqualFold(track.Codec().MimeType, webrtc.MimeTypeH264) {
			fmt.Printf("Ignoring unsupported track %s\n", track.Codec().MimeType)

			return
		}
		defer close(recordingDone)

		file, createErr := os.Create(outputFileName)
		if createErr != nil {
			panic(createErr)
		}
		writer, createErr := mpegts.NewWriter(file, mpegts.WithH264Track(videoPID))
		if createErr != nil {
			panic(createErr)
		}
		defer func() {
			if closeErr := writer.Close(); closeErr != nil {
				fmt.Printf("cannot close %s: %v\n", outputFileName, closeErr)
			}
		}()

		fmt.Printf("Recording H.264 to %s; close the browser tab to finish.\n", outputFileName)
		builder := samplebuilder.New(50, &codecs.H264Packet{}, track.Codec().ClockRate)
		var firstTimestamp uint32
		haveFirstTimestamp := false

		for {
			packet, _, readErr := track.ReadRTP()
			if readErr != nil {
				fmt.Printf("Finished writing %s: %v\n", outputFileName, readErr)

				return
			}
			builder.Push(packet)
			for sample := builder.Pop(); sample != nil; sample = builder.Pop() {
				if !haveFirstTimestamp {
					firstTimestamp = sample.PacketTimestamp
					haveFirstTimestamp = true
				}
				// Unsigned subtraction also handles the 32-bit RTP timestamp wrap.
				timestamp := int64(sample.PacketTimestamp - firstTimestamp)
				if writeErr := writer.WriteH264(videoPID, timestamp, timestamp, sample.Data); writeErr != nil {
					panic(writeErr)
				}
			}
		}
	})

	peerConnection.OnICEConnectionStateChange(func(state webrtc.ICEConnectionState) {
		fmt.Printf("ICE connection state: %s\n", state)
	})

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

	<-recordingDone
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
