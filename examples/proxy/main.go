package main

import (
	"fmt"
	"os"
	"sync"

	"bufio"
	"encoding/base64"

	"github.com/pions/webrtc"
	"github.com/pions/webrtc/pkg/media"
	"github.com/pions/webrtc/pkg/media/samplebuilder"
)

var peerConnectionConfig = webrtc.RTCConfiguration{
	IceServers: []webrtc.RTCIceServer{
		{
			URLs: []string{"stun:stun.l.google.com:19302"},
		},
	},
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}

func mustReadStdin(reader *bufio.Reader) string {
	rawSd, err := reader.ReadString('\n')
	check(err)

	fmt.Println("")
	sd, err := base64.StdEncoding.DecodeString(rawSd)
	check(err)

	return string(sd)
}

func main() {
	reader := bufio.NewReader(os.Stdin)
	sd := mustReadStdin(reader)
	fmt.Println("")

	/* Everything below is the pion-WebRTC API, thanks for using it! */

	// Only support VP8, this makes our proxying code simpler
	webrtc.RegisterCodec(webrtc.NewRTCRtpVP8Codec(webrtc.DefaultPayloadTypeVP8, 90000))

	// Create a new RTCPeerConnection
	peerConnection, err := webrtc.New(peerConnectionConfig)
	check(err)

	outboundSamples := []chan<- media.RTCSample{}
	var outboundSamplesLock sync.RWMutex
	// Set a handler for when a new remote track starts, this just distributes all our packets
	// to connected peers
	peerConnection.Ontrack = func(track *webrtc.RTCTrack) {
		builder := samplebuilder.New(256)
		for {
			builder.Push(<-track.Packets)
			outboundSamplesLock.RLock()
			for s := builder.Pop(); s != nil; s = builder.Pop() {
				for _, outChan := range outboundSamples {
					outChan <- *s
				}
			}
			outboundSamplesLock.RUnlock()
		}
	}

	// Set the remote SessionDescription
	check(peerConnection.SetRemoteDescription(webrtc.RTCSessionDescription{
		Type: webrtc.RTCSdpTypeOffer,
		Sdp:  string(sd),
	}))

	// Sets the LocalDescription, and starts our UDP listeners
	answer, err := peerConnection.CreateAnswer(nil)
	check(err)

	// Get the LocalDescription and take it to base64 so we can paste in browser
	fmt.Println(base64.StdEncoding.EncodeToString([]byte(answer.Sdp)))

	for {
		fmt.Println("")
		fmt.Println("Paste an SDP to start sendonly peer connection")
		recvOnlyOffer := mustReadStdin(reader)

		// Create a new RTCPeerConnection
		peerConnection, err := webrtc.New(peerConnectionConfig)
		check(err)

		// Create a single VP8 Track to send videa
		vp8Track, err := peerConnection.NewRTCTrack(webrtc.DefaultPayloadTypeVP8, "video", "pion2")
		check(err)

		_, err = peerConnection.AddTrack(vp8Track)
		check(err)

		outboundSamplesLock.Lock()
		outboundSamples = append(outboundSamples, vp8Track.Samples)
		outboundSamplesLock.Unlock()

		// Set the remote SessionDescription
		check(peerConnection.SetRemoteDescription(webrtc.RTCSessionDescription{
			Type: webrtc.RTCSdpTypeOffer,
			Sdp:  string(recvOnlyOffer),
		}))

		// Sets the LocalDescription, and starts our UDP listeners
		answer, err := peerConnection.CreateAnswer(nil)
		check(err)

		// Get the LocalDescription and take it to base64 so we can paste in browser
		fmt.Println(base64.StdEncoding.EncodeToString([]byte(answer.Sdp)))
	}
}
