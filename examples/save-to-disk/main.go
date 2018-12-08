package main

import (
	"fmt"

	"github.com/pions/webrtc"
	"github.com/pions/webrtc/examples/util"
	"github.com/pions/webrtc/pkg/ice"
	"github.com/pions/webrtc/pkg/media/ivfwriter"
)

func main() {
	// Everything below is the pion-WebRTC API! Thanks for using it ❤️.

	// Setup the codecs you want to use.
	// We'll use a VP8 codec but you can also define your own
	webrtc.RegisterCodec(webrtc.NewRTCRtpOpusCodec(webrtc.DefaultPayloadTypeOpus, 48000, 2))
	webrtc.RegisterCodec(webrtc.NewRTCRtpVP8Codec(webrtc.DefaultPayloadTypeVP8, 90000))

	// Prepare the configuration
	config := webrtc.RTCConfiguration{
		IceServers: []webrtc.RTCIceServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	}

	// Create a new RTCPeerConnection
	peerConnection, err := webrtc.New(config)
	util.Check(err)

	// Set a handler for when a new remote track starts, this handler saves buffers to disk as
	// an ivf file, since we could have multiple video tracks we provide a counter.
	// In your application this is where you would handle/process video
	peerConnection.OnTrack(func(track *webrtc.RTCTrack) {
		if track.Codec.Name == webrtc.VP8 {
			fmt.Println("Got VP8 track, saving to disk as output.ivf")
			i, err := ivfwriter.New("output.ivf")
			util.Check(err)
			for {
				err = i.AddPacket(<-track.Packets)
				util.Check(err)
			}
		}
	})

	// Set the handler for ICE connection state
	// This will notify you when the peer has connected/disconnected
	peerConnection.OnICEConnectionStateChange(func(connectionState ice.ConnectionState) {
		fmt.Printf("Connection State has changed %s \n", connectionState.String())
	})

	// Wait for the offer to be pasted
	offer := util.Decode(util.MustReadStdin())

	// Set the remote SessionDescription
	err = peerConnection.SetRemoteDescription(offer)
	util.Check(err)

	// Sets the LocalDescription, and starts our UDP listeners
	answer, err := peerConnection.CreateAnswer(nil)
	util.Check(err)

	// Output the answer in base64 so we can paste it in browser
	fmt.Println(util.Encode(answer))

	// Block forever
	select {}
}
