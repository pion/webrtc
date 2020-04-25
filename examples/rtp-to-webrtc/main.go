package main

import (
	"fmt"
	"net"

	"github.com/pion/rtp"
	"github.com/pion/webrtc/v2"
	"github.com/pion/webrtc/v2/examples/internal/signal"
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

	// Open a UDP Listener for RTP Packets on port 5004
	listener, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 5004})
	if err != nil {
		panic(err)
	}
	defer func() {
		if err = listener.Close(); err != nil {
			panic(err)
		}
	}()

	fmt.Println("Waiting for RTP Packets, please run GStreamer or ffmpeg now")

	// Listen for a single RTP Packet, we need this to determine the SSRC
	inboundRTPPacket := make([]byte, 4096) // UDP MTU
	n, _, err := listener.ReadFromUDP(inboundRTPPacket)
	if err != nil {
		panic(err)
	}

	// Unmarshal the incoming packet
	packet := &rtp.Packet{}
	if err = packet.Unmarshal(inboundRTPPacket[:n]); err != nil {
		panic(err)
	}

	// Create a video track, using the same SSRC as the incoming RTP Packet
	videoTrack, err := peerConnection.NewTrack(payloadType, packet.SSRC, "video", "pion")
	if err != nil {
		panic(err)
	}
	if _, err = peerConnection.AddTrack(videoTrack); err != nil {
		panic(err)
	}

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

	// Read RTP packets forever and send them to the WebRTC Client
	for {
		n, _, err := listener.ReadFrom(inboundRTPPacket)
		if err != nil {
			fmt.Printf("error during read: %s", err)
			panic(err)
		}

		packet := &rtp.Packet{}
		if err := packet.Unmarshal(inboundRTPPacket[:n]); err != nil {
			panic(err)
		}
		packet.Header.PayloadType = payloadType

		if writeErr := videoTrack.WriteRTP(packet); writeErr != nil {
			panic(writeErr)
		}
	}
}
