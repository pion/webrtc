package main

import (
	"fmt"
	"github.com/pion/webrtc/v2"
	"github.com/pion/webrtc/v2/examples/internal/signal"
	"github.com/pion/webrtc/v2/pkg/media"
	"math/rand"
	"os/exec"
	"time"
)

func main() {
	// Wait for the offer to be pasted
	offer := webrtc.SessionDescription{}
	signal.Decode(signal.MustReadStdin(), &offer)

	// We make our own mediaEngine so we can place the sender's codecs in it.  This because we must use the
	// dynamic media type from the sender in our answer. This is not required if we are the offerer
	mediaEngine := webrtc.MediaEngine{}
	err := mediaEngine.PopulateFromSDP(offer)
	if err != nil {
		panic(err)
	}

	// Search for VP8 Payload type. If the offer doesn't support VP8 exit since
	// since they won't be able to decode anything we send them
	var payloadType uint8
	for _, videoCodec := range mediaEngine.GetCodecsByKind(webrtc.RTPCodecTypeVideo) {
		if videoCodec.Name == "VP8" {
			payloadType = videoCodec.PayloadType
			break
		}
	}
	if payloadType == 0 {
		panic("Remote peer does not support VP8")
	}

	// Create a new RTCPeerConnection
	api := webrtc.NewAPI(webrtc.WithMediaEngine(mediaEngine))
	peerConnection, err := api.NewPeerConnection(webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	})
	if err != nil {
		panic(err)
	}

	// Create a video track
	videoTrack, err := peerConnection.NewTrack(payloadType, rand.Uint32(), "video", "pion")
	if err != nil {
		panic(err)
	}
	if _, err = peerConnection.AddTrack(videoTrack); err != nil {
		panic(err)
	}

	// start gstreamer v4l2 video
	go func() {
		cmd := exec.Command(
			"gst-launch-1.0",
			// select device e.g.: "v4l2src", "device=/dev/video1"
			"v4l2src",
			// config video width, height and framerate
			"!", "video/x-raw,width=320,height=240,framerate=15/1",
			// encode to vp8
			"!", "vp8enc",
			// output to cmd StdoutPipe
			"!", "filesink", "location=/dev/stdout",
		)

		// we will read the video stream from the stdout reader pipe
		out, err := cmd.StdoutPipe()
		if err != nil {
			panic(err)
		}

		go func() {
			// start the command
			if err := cmd.Start(); err != nil {
				panic(err)
			}
		}()

		buf := make([]byte, 1024*512)
		for {
			// log the start time to calc samples
			start := time.Now()

			// read stdout pipe data
			n, err := out.Read(buf)
			if err != nil {
				panic(err)
			}

			// get this time duration
			duration := time.Since(start)

			// vp8 clock rate is 9kHz, calc with this time duration
			samples := uint32(90000 / 1000 * duration.Milliseconds())

			// output to videoTrack
			if err := videoTrack.WriteSample(media.Sample{Data: buf[:n], Samples: samples}); err != nil {
				panic(err)
			}
		}
	}()

	// Create an opus audio track
	audioTrack, err := peerConnection.NewTrack(webrtc.DefaultPayloadTypeOpus, rand.Uint32(), "audio", "pion")
	if err != nil {
		panic(err)
	}
	if _, err = peerConnection.AddTrack(audioTrack); err != nil {
		panic(err)
	}

	// start gstreamer alsa audio
	go func() {
		cmd := exec.Command(
			"gst-launch-1.0",
			// you should use your device or use default
			"alsasrc", "device=hw:2",
			// my alsa device use this config
			"!", "audio/x-raw,format=S16LE,rate=16000,channels=2",
			// make audio resample
			// if your alsa source is 48000 2channels, you might not need this resample part
			"!", "audioresample",
			"!", "audio/x-raw,format=S16LE,rate=48000,channels=2",
			// encode to opus, use frame-size=10, it's relate with the samples
			"!", "opusenc", "frame-size=10", "audio-type=generic",
			// output to cmd StdoutPipe
			"!", "filesink", "location=/dev/stdout",
		)

		// we will read the audio stream from the stdout reader pipe
		out, err := cmd.StdoutPipe()
		if err != nil {
			panic(err)
		}

		go func() {
			// start the command
			if err := cmd.Start(); err != nil {
				panic(err)
			}
		}()

		buf := make([]byte, 1024)
		for {
			// read stdout pipe data
			n, err := out.Read(buf)
			if err != nil {
				panic(err)
			}

			// calc audio samples
			// 48000(sample-rate) / 1000(1sec) * 10(frame-size)
			samples := uint32(48000 / 1000 * 10)

			// output to audioTrack
			if err := audioTrack.WriteSample(media.Sample{Data: buf[:n], Samples: samples}); err != nil {
				panic(err)
			}
		}
	}()

	// Set the handler for ICE connection state
	// This will notify you when the peer has connected/disconnected
	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		fmt.Printf("Connection State has changed %s \n", connectionState.String())
	})

	// Set the remote SessionDescription
	if err = peerConnection.SetRemoteDescription(offer); err != nil {
		panic(err)
	}

	// Create answer
	answer, err := peerConnection.CreateAnswer(nil)
	if err != nil {
		panic(err)
	}

	// Sets the LocalDescription, and starts our UDP listeners
	if err = peerConnection.SetLocalDescription(answer); err != nil {
		panic(err)
	}

	// Output the answer in base64 so we can paste it in browser
	fmt.Println(signal.Encode(answer))

	// Block forever
	select {}
}
