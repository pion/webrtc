package main

import (
	"fmt"
	"time"

	janus "github.com/notedit/janus-go"
	"github.com/pions/webrtc"
	"github.com/pions/webrtc/pkg/media"
	"github.com/pions/webrtc/pkg/media/ivfwriter"
	"github.com/pions/webrtc/pkg/media/opuswriter"
)

func saveToDisk(i media.Writer, track *webrtc.Track) {
	defer func() {
		if err := i.Close(); err != nil {
			panic(err)
		}
	}()

	for {
		packet, err := track.ReadRTP()
		if err != nil {
			panic(err)
		}

		if err := i.AddPacket(packet); err != nil {
			panic(err)
		}
	}
}

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

	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		fmt.Printf("Connection State has changed %s \n", connectionState.String())
	})

	peerConnection.OnTrack(func(track *webrtc.Track, receiver *webrtc.RTPReceiver) {
		codec := track.Codec()
		if codec.Name == webrtc.Opus {
			fmt.Println("Got Opus track, saving to disk as output.opus")
			i, opusNewErr := opuswriter.New("output.opus", codec.ClockRate, codec.Channels)
			if opusNewErr != nil {
				panic(opusNewErr)
			}
			saveToDisk(i, track)
		} else if codec.Name == webrtc.VP8 {
			fmt.Println("Got VP8 track, saving to disk as output.ivf")
			i, ivfNewErr := ivfwriter.New("output.ivf")
			if ivfNewErr != nil {
				panic(ivfNewErr)
			}
			saveToDisk(i, track)
		}
	})

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
	if err != nil {
		panic(err)
	}

	go watchHandle(handle)

	// Get streaming list
	_, err = handle.Request(map[string]interface{}{
		"request": "list",
	})
	if err != nil {
		panic(err)
	}

	// Watch the second stream
	msg, err := handle.Message(map[string]interface{}{
		"request": "watch",
		"id":      1,
	}, nil)
	if err != nil {
		panic(err)
	}

	if msg.Jsep != nil {
		err = peerConnection.SetRemoteDescription(webrtc.SessionDescription{
			Type: webrtc.SDPTypeOffer,
			SDP:  msg.Jsep["sdp"].(string),
		})
		if err != nil {
			panic(err)
		}

		answer, answerErr := peerConnection.CreateAnswer(nil)
		if answerErr != nil {
			panic(answerErr)
		}

		err = peerConnection.SetLocalDescription(answer)
		if err != nil {
			panic(err)
		}

		// now we start
		_, err = handle.Message(map[string]interface{}{
			"request": "start",
		}, map[string]interface{}{
			"type":    "answer",
			"sdp":     answer.SDP,
			"trickle": false,
		})
		if err != nil {
			panic(err)
		}
	}
	for {
		_, err = session.KeepAlive()
		if err != nil {
			panic(err)
		}

		time.Sleep(5 * time.Second)
	}
}
