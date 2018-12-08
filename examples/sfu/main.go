package main

import (
	"fmt"
	"os"
	"sync"
	"time"

	"bufio"
	"encoding/base64"

	"github.com/pions/webrtc"
	"github.com/pions/webrtc/examples/util"
	"github.com/pions/webrtc/pkg/media"
	"github.com/pions/webrtc/pkg/media/samplebuilder"
	"github.com/pions/webrtc/pkg/rtcp"
	"github.com/pions/webrtc/pkg/rtp/codecs"
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

const (
	rtcpPLIInterval = time.Second * 3
)

func main() {
	reader := bufio.NewReader(os.Stdin)
	offer := util.Decode(mustReadStdin(reader))
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
	peerConnection.OnTrack(func(track *webrtc.RTCTrack) {
		// Send a PLI on an interval so that the publisher is pushing a keyframe every rtcpPLIInterval
		// This is a temporary fix until we implement incoming RTCP events, then we would push a PLI only when a viewer requests it
		go func() {
			ticker := time.NewTicker(rtcpPLIInterval)
			for range ticker.C {
				if err := peerConnection.SendRTCP(&rtcp.PictureLossIndication{MediaSSRC: track.Ssrc}); err != nil {
					fmt.Println(err)
				}
			}
		}()

		// Transform RTP packets into samples, allowing us to distribute incoming packets from the publisher to everyone who has joined the broadcast
		builder := samplebuilder.New(256, &codecs.VP8Packet{})
		for {
			outboundSamplesLock.RLock()
			builder.Push(<-track.Packets)
			for s := builder.Pop(); s != nil; s = builder.Pop() {
				for _, outChan := range outboundSamples {
					outChan <- *s
				}
			}
			outboundSamplesLock.RUnlock()
		}
	})

	// Set the remote SessionDescription
	check(peerConnection.SetRemoteDescription(offer))

	// Sets the LocalDescription, and starts our UDP listeners
	answer, err := peerConnection.CreateAnswer(nil)
	check(err)

	// Get the LocalDescription and take it to base64 so we can paste in browser
	fmt.Println(util.Encode(answer))

	for {
		fmt.Println("")
		fmt.Println("Paste an SDP to start sendonly peer connection")
		recvOnlyOffer := util.Decode(mustReadStdin(reader))

		// Create a new RTCPeerConnection
		peerConnection, err := webrtc.New(peerConnectionConfig)
		check(err)

		// Create a single VP8 Track to send videa
		vp8Track, err := peerConnection.NewRTCSampleTrack(webrtc.DefaultPayloadTypeVP8, "video", "pion2")
		check(err)

		_, err = peerConnection.AddTrack(vp8Track)
		check(err)

		outboundSamplesLock.Lock()
		outboundSamples = append(outboundSamples, vp8Track.Samples)
		outboundSamplesLock.Unlock()

		// Set the remote SessionDescription
		err = peerConnection.SetRemoteDescription(recvOnlyOffer)
		check(err)

		// Sets the LocalDescription, and starts our UDP listeners
		answer, err := peerConnection.CreateAnswer(nil)
		check(err)

		// Get the LocalDescription and take it to base64 so we can paste in browser
		fmt.Println(util.Encode(answer))
	}
}
