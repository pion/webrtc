package main

import (
	"fmt"
	"log"
	"net"

	"github.com/pions/webrtc"
	"github.com/pions/webrtc/pkg/datachannel"
	"github.com/pions/webrtc/pkg/ice"
)

func buildPeerConnection() *webrtc.RTCPeerConnection {
	peerConnection, err := webrtc.New(webrtc.RTCConfiguration{
		IceServers: []webrtc.RTCIceServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	})
	if err != nil {
		panic(err)
	}

	peerConnection.OnIceConnectionStateChange = func(connectionState ice.ConnectionState) {
		fmt.Printf("Connection State has changed %s \n", connectionState.String())
	}

	peerConnection.OnDataChannel = func(d *webrtc.RTCDataChannel) {
		fmt.Printf("New DataChannel %s %d\n", d.Label, d.ID)

		d.Lock()
		defer d.Unlock()
		d.OnMessage = func(payload datachannel.Payload) {
			switch p := payload.(type) {
			case *datachannel.PayloadString:
				fmt.Printf("Message '%s' from DataChannel '%s' payload '%s'\n", p.PayloadType().String(), d.Label, string(p.Data))
			case *datachannel.PayloadBinary:
				fmt.Printf("Message '%s' from DataChannel '%s' payload '% 02x'\n", p.PayloadType().String(), d.Label, p.Data)
			default:
				fmt.Printf("Message '%s' from DataChannel '%s' no payload \n", p.PayloadType().String(), d.Label)
			}
		}
	}

	return peerConnection
}

func main() {
	c, err := net.Dial("unix", "../pion.sock")
	if err != nil {
		log.Fatal("Dial error", err)
	}
	defer c.Close()

	peerConnection := buildPeerConnection()
	offer, err := peerConnection.CreateOffer(nil)
	if err != nil {
		panic(err)
	}

	if _, err = c.Write([]byte(offer.Sdp)); err != nil {
		log.Fatal("Write error:", err)
	}

	buf := make([]byte, 5000)
	n, err := c.Read(buf[:])
	if err != nil {
		return
	}

	if err := peerConnection.SetRemoteDescription(webrtc.RTCSessionDescription{
		Type: webrtc.RTCSdpTypeAnswer,
		Sdp:  string(buf[0:n]),
	}); err != nil {
		panic(err)
	}

	select {}
}
