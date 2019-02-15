package main

import (
	"fmt"
	"log"

	janus "github.com/notedit/janus-go"
	"github.com/pions/webrtc"
	"github.com/pions/webrtc/examples/util"
	gst "github.com/pions/webrtc/examples/util/gstreamer-src"
	"github.com/pions/webrtc/pkg/ice"
)

func watchHandle(handle *janus.Handle) {
	// wait for event
	for {
		msg := <-handle.Events
		switch msg := msg.(type) {
		case *janus.SlowLinkMsg:
			log.Println("SlowLinkMsg type ", handle.Id)
		case *janus.MediaMsg:
			log.Println("MediaEvent type", msg.Type, " receiving ", msg.Receiving)
		case *janus.WebRTCUpMsg:
			log.Println("WebRTCUp type ", handle.Id)
		case *janus.HangupMsg:
			log.Println("HangupEvent type ", handle.Id)
		case *janus.EventMsg:
			log.Printf("EventMsg %+v", msg.Plugindata.Data)
		}

	}

}

func main() {
	// Everything below is the pion-WebRTC API! Thanks for using it ❤️.

	// Setup the codecs you want to use.
	// We'll use the default ones but you can also define your own
	webrtc.RegisterDefaultCodecs()

	// Prepare the configuration
	config := webrtc.RTCConfiguration{
		IceServers: []webrtc.RTCIceServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	}

	// Create a new RTCPeerConnection
	peerConnection, err := webrtc.NewRTCPeerConnection(config)
	util.Check(err)

	peerConnection.OnICEConnectionStateChange(func(connectionState ice.ConnectionState) {
		fmt.Printf("Connection State has changed %s \n", connectionState.String())
	})

	// Create a audio track
	opusTrack, err := peerConnection.NewRTCSampleTrack(webrtc.DefaultPayloadTypeOpus, "audio", "pion1")
	util.Check(err)
	_, err = peerConnection.AddTrack(opusTrack)
	util.Check(err)

	// Create a video track
	vp8Track, err := peerConnection.NewRTCSampleTrack(webrtc.DefaultPayloadTypeVP8, "video", "pion2")
	util.Check(err)
	_, err = peerConnection.AddTrack(vp8Track)
	util.Check(err)

	offer, err := peerConnection.CreateOffer(nil)
	util.Check(err)

	err = peerConnection.SetLocalDescription(offer)
	util.Check(err)

	gateway, err := janus.Connect("ws://localhost:8188/janus")
	util.Check(err)

	session, err := gateway.Create()
	util.Check(err)

	handle, err := session.Attach("janus.plugin.videoroom")
	util.Check(err)

	go watchHandle(handle)

	_, err = handle.Message(map[string]interface{}{
		"request": "join",
		"ptype":   "publisher",
		"room":    1234,
		"id":      1,
	}, nil)
	util.Check(err)

	msg, err := handle.Message(map[string]interface{}{
		"request": "publish",
		"audio":   true,
		"video":   true,
		"data":    false,
	}, map[string]interface{}{
		"type":    "offer",
		"sdp":     offer.Sdp,
		"trickle": false,
	})
	util.Check(err)

	if msg.Jsep != nil {
		err = peerConnection.SetRemoteDescription(webrtc.RTCSessionDescription{
			Type: webrtc.RTCSdpTypeAnswer,
			Sdp:  msg.Jsep["sdp"].(string),
		})
		util.Check(err)

		// Start pushing buffers on these tracks
		gst.CreatePipeline(webrtc.Opus, opusTrack.Samples, "audiotestsrc").Start()
		gst.CreatePipeline(webrtc.VP8, vp8Track.Samples, "videotestsrc").Start()
	}

	select {}

}
