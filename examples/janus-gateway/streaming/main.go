package main

import (
	"fmt"
	"time"

	janus "github.com/notedit/janus-go"
	"github.com/pions/webrtc"
	"github.com/pions/webrtc/examples/util"
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
	// Everything below is the pion-WebRTC API! Thanks for using it ❤️.

	// Setup the codecs you want to use.
	// We'll use the default ones but you can also define your own
	webrtc.RegisterDefaultCodecs()

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
	util.Check(err)

	peerConnection.OnICEConnectionStateChange(func(connectionState ice.ConnectionState) {
		fmt.Printf("Connection State has changed %s \n", connectionState.String())
	})

	peerConnection.OnTrack(func(track *webrtc.Track) {
		if track.Codec.Name == webrtc.Opus {
			return
		}

		fmt.Println("Got VP8 track, saving to disk as output.ivf")
		i, err := ivfwriter.New("output.ivf")
		util.Check(err)
		for {
			err = i.AddPacket(<-track.Packets)
			util.Check(err)
		}
	})

	// Janus
	gateway, err := janus.Connect("ws://localhost:8188/")
	util.Check(err)

	// Create session
	session, err := gateway.Create()
	util.Check(err)

	// Create handle
	handle, err := session.Attach("janus.plugin.streaming")
	util.Check(err)

	go watchHandle(handle)

	// Get streaming list
	_, err = handle.Request(map[string]interface{}{
		"request": "list",
	})
	util.Check(err)

	// Watch the second stream
	msg, err := handle.Message(map[string]interface{}{
		"request": "watch",
		"id":      1,
	}, nil)
	util.Check(err)

	if msg.Jsep != nil {
		err = peerConnection.SetRemoteDescription(webrtc.SessionDescription{
			Type: webrtc.SDPTypeOffer,
			SDP:  msg.Jsep["sdp"].(string),
		})
		util.Check(err)

		answer, err := peerConnection.CreateAnswer(nil)
		util.Check(err)

		err = peerConnection.SetLocalDescription(answer)
		util.Check(err)

		// now we start
		_, err = handle.Message(map[string]interface{}{
			"request": "start",
		}, map[string]interface{}{
			"type":    "answer",
			"sdp":     answer.SDP,
			"trickle": false,
		})
		util.Check(err)
	}
	for {
		_, err = session.KeepAlive()
		util.Check(err)

		time.Sleep(5 * time.Second)
	}
}
