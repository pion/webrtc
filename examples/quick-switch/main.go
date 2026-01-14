// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js
// +build !js

// quick-switch demonstrates Pion WebRTC's ability to quickly switch between videos.
package main

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"
	"github.com/pion/webrtc/v4/pkg/media/ivfreader"
)

// nolint: gochecknoglobals
var (
	tracksLock sync.RWMutex
	tracks     []*webrtc.TrackLocalStaticSample

	videoFiles     []string
	videoFileIndex atomic.Int32
)

func nextVideo() {
	newIndex := videoFileIndex.Load() + 1
	if int(newIndex) >= len(videoFiles) {
		newIndex = 0
	}

	videoFileIndex.Store(newIndex)
}

// nolint: cyclop
func doWHIP(res http.ResponseWriter, req *http.Request) {
	peerConnection, err := webrtc.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		panic(err)
	}

	peerConnection.OnDataChannel(func(d *webrtc.DataChannel) {
		d.OnMessage(func(_ webrtc.DataChannelMessage) {
			nextVideo()
		})
	})

	// One Track is used for PeerConnection. All video streams are written to one Track
	videoTrack, err := webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{
		MimeType: webrtc.MimeTypeAV1,
	}, "video", "video")
	if err != nil {
		panic(err)
	}

	if _, err = peerConnection.AddTrack(videoTrack); err != nil {
		panic(err)
	}

	tracksLock.Lock()
	tracks = append(tracks, videoTrack)
	tracksLock.Unlock()

	// Set the handler for ICE connection state
	// This will notify you when the peer has connected/disconnected
	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		fmt.Printf("ICE Connection State has changed: %s\n", connectionState.String())
		if connectionState == webrtc.ICEConnectionStateClosed || connectionState == webrtc.ICEConnectionStateFailed {
			if closeErr := peerConnection.Close(); closeErr != nil {
				panic(closeErr)
			}

			tracksLock.Lock()
			tracks = slices.DeleteFunc(tracks, func(x *webrtc.TrackLocalStaticSample) bool { return x == videoTrack })
			tracksLock.Unlock()
		}
	})

	offer, err := io.ReadAll(req.Body)
	if err != nil {
		panic(err)
	}

	if err = peerConnection.SetRemoteDescription(webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  string(offer),
	}); err != nil {
		panic(err)
	}

	// Create channel that is blocked until ICE Gathering is complete
	gatherComplete := webrtc.GatheringCompletePromise(peerConnection)

	answer, err := peerConnection.CreateAnswer(nil)
	if err != nil {
		panic(err)
	} else if err = peerConnection.SetLocalDescription(answer); err != nil {
		panic(err)
	}

	// Block until ICE Gathering is complete, disabling trickle ICE
	// we do this because we only can exchange one signaling message
	// in a production application you should exchange ICE Candidates via OnICECandidate
	<-gatherComplete

	res.Header().Set("Content-Type", "application/sdp")
	if _, err := res.Write([]byte(peerConnection.LocalDescription().SDP)); err != nil {
		panic(err)
	}
}

func playFile(fileIndex int32) {
	file, err := os.Open(videoFiles[fileIndex])
	if err != nil {
		panic(err)
	}
	defer file.Close() // nolint: errcheck

	ivf, header, err := ivfreader.NewWith(file)
	if err != nil {
		panic(err)
	}

	frameDuration := time.Duration(header.TimebaseNumerator) * time.Second / time.Duration(header.TimebaseDenominator)
	ticker := time.NewTicker(frameDuration)
	defer ticker.Stop()

	for {
		if fileIndex != videoFileIndex.Load() {
			return
		}

		frame, _, err := ivf.ParseNextFrame()
		if errors.Is(err, io.EOF) {
			nextVideo()

			return
		} else if err != nil {
			panic(err)
		}

		tracksLock.RLock()
		for _, t := range tracks {
			if err = t.WriteSample(media.Sample{Data: frame, Duration: frameDuration}); err != nil {
				tracksLock.RUnlock()
				panic(err)
			}
		}
		tracksLock.RUnlock()
		<-ticker.C
	}
}

func main() {
	http.Handle("/", http.FileServer(http.Dir(".")))
	http.HandleFunc("/whip", doWHIP)

	// Switch between all ivf files in current directory
	go func() {
		files, err := filepath.Glob("*.ivf")
		if err != nil {
			panic(err)
		}
		for _, p := range files {
			videoFiles = append(videoFiles, filepath.Base(p))
		}
		if len(videoFiles) == 0 {
			panic("no .ivf files found in the working directory")
		}

		for {
			playFile(videoFileIndex.Load())
		}
	}()

	fmt.Println("Open http://localhost:8080 to access this demo")
	// nolint: gosec
	panic(http.ListenAndServe(":8080", nil))
}
