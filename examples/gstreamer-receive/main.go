package main

import (
	"fmt"
	"runtime"
	"time"

	"github.com/pions/rtcp"
	"github.com/pions/webrtc"

	gst "github.com/pions/webrtc/examples/internal/gstreamer-sink"
	"github.com/pions/webrtc/examples/internal/signal"
)

// gstreamerReceiveMain is launched in a goroutine because the main thread is needed
// for Glib's main loop (Gstreamer uses Glib)
func gstreamerReceiveMain() {
	// Everything below is the pion-WebRTC API! Thanks for using it ❤️.

	// Prepare the configuration
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	}

	// Create a new RTCPeerConnection
	peerConnection, err := webrtc.NewPeerConnection(config)
	if err != nil {
		panic(err)
	}

	// Set a handler for when a new remote track starts, this handler creates a gstreamer pipeline
	// for the given codec
	peerConnection.OnTrack(func(track *webrtc.Track, receiver *webrtc.RTPReceiver) {
		// Send a PLI on an interval so that the publisher is pushing a keyframe every rtcpPLIInterval
		// This is a temporary fix until we implement incoming RTCP events, then we would push a PLI only when a viewer requests it
		go func() {
			ticker := time.NewTicker(time.Second * 3)
			for range ticker.C {
				err := peerConnection.SendRTCP(&rtcp.PictureLossIndication{MediaSSRC: track.SSRC()})
				if err != nil {
					fmt.Println(err)
				}
			}
		}()

		codec := track.Codec()
		fmt.Printf("Track has started, of type %d: %s \n", track.PayloadType(), codec.Name)
		pipeline := gst.CreatePipeline(codec.Name)
		pipeline.Start()
		buf := make([]byte, 1400)
		for {
			i, err := track.Read(buf)
			if err != nil {
				panic(err)
			}

			pipeline.Push(buf[:i])
		}
	})

	// Set the handler for ICE connection state
	// This will notify you when the peer has connected/disconnected
	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		fmt.Printf("Connection State has changed %s \n", connectionState.String())
	})

	// Wait for the offer to be pasted
	offer := webrtc.SessionDescription{}
	signal.Decode(signal.MustReadStdin(), &offer)

	// Set the remote SessionDescription
	err = peerConnection.SetRemoteDescription(offer)
	if err != nil {
		panic(err)
	}

	// Create an answer
	answer, err := peerConnection.CreateAnswer(nil)
	if err != nil {
		panic(err)
	}

	// Sets the LocalDescription, and starts our UDP listeners
	err = peerConnection.SetLocalDescription(answer)
	if err != nil {
		panic(err)
	}

	// Output the answer in base64 so we can paste it in browser
	fmt.Println(signal.Encode(answer))

	// Block forever
	select {}
}

func init() {
	// This example uses Gstreamer's autovideosink element to display the received video
	// This element, along with some others, sometimes require that the process' main thread is used
	runtime.LockOSThread()
}

func main() {
	// Start a new thread to do the actual work for this application
	go gstreamerReceiveMain()
	// Use this goroutine (which has been runtime.LockOSThread'd to he the main thread) to run the Glib loop that Gstreamer requires
	gst.StartMainLoop()
}
