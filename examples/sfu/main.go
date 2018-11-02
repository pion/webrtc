package main

import (
	"crypto/rand"
	"fmt"
	"os"
	"time"

	"encoding/base64"
	"encoding/binary"

	"golang.org/x/crypto/ssh/terminal"

	"github.com/pions/webrtc"
	"github.com/pions/webrtc/pkg/rtcp"
	"github.com/pions/webrtc/pkg/sfu"
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

func mustReadStdin(reader *terminal.Terminal) string {
	// Set stty to raw mode in order to read > 1024 chars from stdin
	old, err := terminal.MakeRaw(0)
	check(err)
	defer terminal.Restore(0, old)

	rawSd, err := reader.ReadLine()
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
	reader := terminal.NewTerminal(os.Stdin, "SDP: ")
	sd := mustReadStdin(reader)
	fmt.Println("")

	/* Everything below is the pion-WebRTC API, thanks for using it! */

	sfu := sfu.New()

	// Only support VP8, this makes our proxying code simpler
	webrtc.RegisterCodec(webrtc.NewRTCRtpVP8Codec(webrtc.DefaultPayloadTypeVP8, 90000))

	// Create a new RTCPeerConnection
	peerConnection, err := webrtc.New(peerConnectionConfig)
	check(err)

	// Set a handler for when a new remote track starts, this just distributes all our packets
	// to connected peers
	peerConnection.OnTrack = func(track *webrtc.RTCTrack) {
		// Send a PLI on an interval so that the publisher is pushing a keyframe every rtcpPLIInterval
		// This is a temporary fix until we implement incoming RTCP events, then we would push a PLI only when a viewer requests it
		go func() {
			ticker := time.NewTicker(rtcpPLIInterval)
			for {
				select {
				case <-ticker.C:
					err := peerConnection.SendRTCP(&rtcp.PictureLossIndication{MediaSSRC: track.Ssrc})
					if err != nil {
						fmt.Println(err)
					}
				}
			}
		}()

		sfu.AddSource(track.Packets)
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

		// Generate a random ssrc for each sink track
		buf := make([]byte, 4)
		_, err = rand.Read(buf)
		check(err)
		ssrc := binary.LittleEndian.Uint32(buf)

		// Create a single VP8 Track to send videa
		vp8Track, err := peerConnection.NewRawRTPTrack(webrtc.DefaultPayloadTypeVP8, ssrc, "video", "pion2")
		check(err)

		_, err = peerConnection.AddTrack(vp8Track)
		check(err)

		sfu.AddSink(vp8Track.RawRTP)

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
