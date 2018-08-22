package main

import (
	"fmt"

	janus "github.com/notedit/janus-go"
	"github.com/pions/webrtc"
	"github.com/pions/webrtc/pkg/ice"
	"github.com/pions/webrtc/pkg/media/ivfwriter"
)

func watchHandle(handle *janus.Handle) {
	// wait for event
	for {
		msg := <-handle.Events
		switch msg := msg.(type) {
		case *janus.SlowLinkMsg:
			fmt.Print("SlowLinkMsg type ", handle.Id)
		case *janus.MediaMsg:
			fmt.Print("MediaEvent type", msg.Type, " receiving ", msg.Receiving)
		case *janus.WebRTCUpMsg:
			fmt.Print("WebRTCUp type ", handle.Id)
		case *janus.HangupMsg:
			fmt.Print("HangupEvent type ", handle.Id)
		case *janus.EventMsg:
			fmt.Printf("EventMsg %+v", msg.Plugindata.Data)
		}

	}

}

func main() {
	webrtc.RegisterDefaultCodecs()

	// Create a new RTCPeerConnection
	peerConnection, err := webrtc.New(webrtc.RTCConfiguration{})
	if err != nil {
		panic(err)
	}

	peerConnection.OnICEConnectionStateChange = func(connectionState ice.ConnectionState) {
		fmt.Printf("Connection State has changed %s \n", connectionState.String())
	}

	peerConnection.OnTrack = func(track *webrtc.RTCTrack) {
		if track.Codec.Name == webrtc.Opus {
			return
		}

		fmt.Println("Got VP8 track, saving to disk as output.ivf")
		i, err := ivfwriter.New("output.ivf")
		if err != nil {
			panic(err)
		}
		for {
			if err := i.AddPacket(<-track.Packets); err != nil {
				panic(err)
			}
		}
	}

	// Janus
	gateway, err := janus.Connect("ws://localhost:8188/")

	if err != nil {
		panic(err)
	}

	// Create session
	session, err := gateway.Create()

	if err != nil {
		panic(err)
	}

	// Create handle
	handle, err := session.Attach("janus.plugin.streaming")

	go watchHandle(handle)

	// Get streaming list
	if _, err := handle.Request(map[string]interface{}{
		"request": "list",
	}); err != nil {
		panic(err)
	}

	// Watch the second stream
	msg, err := handle.Message(map[string]interface{}{
		"request": "watch",
		"id":      1,
	}, nil)

	if err != nil {
		fmt.Print("message", msg)
		panic(err)
	}

	if msg.Jsep != nil {
		if err := peerConnection.SetRemoteDescription(webrtc.RTCSessionDescription{
			Type: webrtc.RTCSdpTypeOffer,
			Sdp:  msg.Jsep["sdp"].(string),
		}); err != nil {
			panic(err)
		}

		answer, err := peerConnection.CreateAnswer(nil)
		if err != nil {
			panic(err)
		}

		// now we start
		if _, err := handle.Message(map[string]interface{}{
			"request": "start",
		}, map[string]interface{}{
			"type":    "answer",
			"sdp":     answer.Sdp,
			"trickle": false,
		}); err != nil {
			panic(err)
		}
	}
	select {}
}
