// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js
// +build !js

// repacketize demonstrates how many video codecs can be received, depacketized
// and packetized by Pion over RTP.
package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/pion/interceptor"
	"github.com/pion/rtcp"
	"github.com/pion/rtp"
	"github.com/pion/rtp/codecs"
	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"
	"github.com/pion/webrtc/v4/pkg/media/samplebuilder"
)

//go:embed index.html index.js
var web embed.FS

func main() {
	fs := http.FileServer(http.FS(web))

	// Serve web files
	http.Handle("/", fs)

	// Receive SDP offer from browser and send the answer back. This should ideally
	// be done with WHIP/WHEP but for the purposes of this example, this is good enough.
	// Check out the whip-whep example to see how to do that instead.
	http.HandleFunc("/sdp", func(w http.ResponseWriter, r *http.Request) { //nolint
		w.Header().Set("Access-Control-Allow-Headers", "*")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		switch r.Method {
		case "POST":
			pc := createRTCConn()

			sdp := &webrtc.SessionDescription{}
			if err := json.NewDecoder(r.Body).Decode(sdp); err != nil {
				panic(err)
			}

			if err := pc.SetRemoteDescription(*sdp); err != nil {
				panic(err)
			}

			gather := webrtc.GatheringCompletePromise(pc)

			answer, err := pc.CreateAnswer(nil)
			if err != nil {
				panic(err)
			}
			err = pc.SetLocalDescription(answer)
			if err != nil {
				panic(err)
			}

			<-gather

			resp, err := json.Marshal(pc.LocalDescription())
			if err != nil {
				panic(err)
			}

			w.Header().Set("Content-Type", "application/json")
			if _, err := w.Write(resp); err != nil {
				panic(err)
			}

			return
		case "OPTIONS":
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
	fmt.Println("Open http://localhost:8080 to access this example")
	panic(http.ListenAndServe(":8080", nil)) //nolint:gosec // example
}

func createRTCConn() *webrtc.PeerConnection { //nolint:cyclop
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{URLs: []string{"stun:stun.l.google.com:19302"}},
		},
	}

	mediaEngine := webrtc.MediaEngine{}

	if err := mediaEngine.RegisterDefaultCodecs(); err != nil {
		panic(err)
	}

	// Create a InterceptorRegistry. This is the user configurable RTP/RTCP Pipeline.
	// This provides NACKs, RTCP Reports and other features. If you use `webrtc.NewPeerConnection`
	// this is enabled by default. If you are manually managing You MUST create a InterceptorRegistry
	// for each PeerConnection.
	ir := &interceptor.Registry{}

	// Use the default set of Interceptors
	if err := webrtc.RegisterDefaultInterceptors(&mediaEngine, ir); err != nil {
		panic(err)
	}

	api := webrtc.NewAPI(
		webrtc.WithMediaEngine(&mediaEngine),
		webrtc.WithInterceptorRegistry(ir),
	)
	pc, err := api.NewPeerConnection(config)
	if err != nil {
		panic(err)
	}

	// Create Transceiver that we send and receive video to/from browser
	trans, err := pc.AddTransceiverFromKind(
		webrtc.RTPCodecTypeVideo,
		webrtc.RTPTransceiverInit{Direction: webrtc.RTPTransceiverDirectionSendrecv},
	)
	if err != nil {
		panic(err)
	}

	go func() {
		buf := make([]byte, 0)
		for {
			_, _, err := trans.Sender().Read(buf)
			if err != nil {
				return
			}
		}
	}()

	pc.OnTrack(func(tr *webrtc.TrackRemote, r *webrtc.RTPReceiver) {
		var depacketizer rtp.Depacketizer

		switch tr.Codec().MimeType {
		case webrtc.MimeTypeAV1:
			depacketizer = &codecs.AV1Depacketizer{}
		case webrtc.MimeTypeVP8:
			depacketizer = &codecs.VP8Packet{}
		case webrtc.MimeTypeVP9:
			depacketizer = &codecs.VP9Packet{}
		case webrtc.MimeTypeH264:
			depacketizer = &codecs.H264Packet{}
		case webrtc.MimeTypeH265:
			depacketizer = &codecs.H265Depacketizer{}
		default:
			return
		}

		// Request a new I-frame every 500ms
		go func() {
			t := time.NewTicker(time.Millisecond * 500)
			defer t.Stop()

			for range t.C {
				if err := pc.WriteRTCP([]rtcp.Packet{
					&rtcp.PictureLossIndication{MediaSSRC: uint32(tr.SSRC())},
				}); err != nil {
					panic(err)
				}
			}
		}()

		// New track with the same codec as the received track
		newTrack, err := webrtc.NewTrackLocalStaticSample(
			tr.Codec().RTPCodecCapability,
			"restream",
			"pion",
		)
		if err != nil {
			panic(err)
		}

		// Add it to the transceiver
		if err := trans.Sender().ReplaceTrack(newTrack); err != nil {
			panic(err)
		}

		fmt.Println("New track:", tr.Codec().MimeType)

		// SampleBuilder reorders and depacketizes incoming RTP packets
		sb := samplebuilder.New(100, depacketizer, tr.Codec().ClockRate)

		for rtp, _, readErr := tr.ReadRTP(); readErr == nil; rtp, _, readErr = tr.ReadRTP() {
			sb.Push(rtp)
			for sample := sb.Pop(); sample != nil; sample = sb.Pop() {
				// WriteSample takes sample.Data and packetizes it according to the track's codec
				err := newTrack.WriteSample(media.Sample{
					Data:     sample.Data,
					Duration: sample.Duration,
				})
				if err != nil {
					panic(err)
				}
			}
		}
		fmt.Println("Track ended")
	})

	pc.OnICEConnectionStateChange(func(is webrtc.ICEConnectionState) {
		fmt.Println("ICE connection state changed:", is)
	})

	pc.OnConnectionStateChange(func(pcs webrtc.PeerConnectionState) {
		fmt.Println("Connection state changed:", pcs)
	})

	return pc
}
