package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/pions/webrtc"
	"github.com/pions/webrtc/pkg/datachannel"
	"github.com/pions/webrtc/pkg/ice"
)

func buildPeerConnection() *webrtc.RTCPeerConnection {
	peerConnection, err := webrtc.New(webrtc.RTCConfiguration{
		ICEServers: []webrtc.RTCICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	})
	if err != nil {
		panic(err)
	}

	d, err := peerConnection.CreateDataChannel("data", nil)
	if err != nil {
		panic(err)
	}

	d.Lock()
	d.Onmessage = func(payload datachannel.Payload) {
		switch p := payload.(type) {
		case *datachannel.PayloadString:
			fmt.Printf("Message '%s' from DataChannel '%s' payload '%s'\n", p.PayloadType().String(), d.Label, string(p.Data))
		case *datachannel.PayloadBinary:
			fmt.Printf("Message '%s' from DataChannel '%s' payload '% 02x'\n", p.PayloadType().String(), d.Label, p.Data)
		default:
			fmt.Printf("Message '%s' from DataChannel '%s' no payload \n", p.PayloadType().String(), d.Label)
		}
	}
	d.Unlock()

	peerConnection.OnICEConnectionStateChange = func(connectionState ice.ConnectionState) {
		fmt.Printf("Connection State has changed %s \n", connectionState.String())
		if connectionState == ice.ConnectionStateConnected {
			fmt.Println("sending openchannel")
			err := d.SendOpenChannelMessage()
			if err != nil {
				fmt.Println("faild to send openchannel", err)
			}
		}
	}

	return peerConnection
}

func main() {
	ln, err := net.Listen("unix", "../pion.sock")
	if err != nil {
		log.Fatal("Failed to listen: ", err)
	}

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, os.Interrupt, syscall.SIGTERM)
	go func(ln net.Listener, c chan os.Signal) {
		sig := <-c
		log.Printf("Caught signal %s: shutting down.", sig)
		ln.Close()
		os.Exit(0)
	}(ln, sigc)

	fmt.Println("Ready for offer")
	fd, err := ln.Accept()
	if err != nil {
		log.Fatal("Accept error: ", err)
	}

	buf := make([]byte, 5000)
	n, err := fd.Read(buf)
	if err != nil {
		return
	}

	peerConnection := buildPeerConnection()

	if err := peerConnection.SetRemoteDescription(webrtc.RTCSessionDescription{
		Type: webrtc.RTCSdpTypeOffer,
		Sdp:  string(buf[0:n]),
	}); err != nil {
		panic(err)
	}

	answer, err := peerConnection.CreateAnswer(nil)
	if err != nil {
		panic(err)
	}

	if _, err := fd.Write([]byte(answer.Sdp)); err != nil {
		log.Fatal("Writing client error: ", err)
	}
	select {}
}
