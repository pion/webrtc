// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js
// +build !js

// ortc demonstrates Pion WebRTC's ORTC capabilities.
package main

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/pion/randutil"
	"github.com/pion/webrtc/v4"
)

// nolint:cyclop
func main() {
	isOffer := flag.Bool("offer", false, "Act as the offerer if set")
	port := flag.Int("port", 8080, "http server port")
	flag.Parse()

	// Everything below is the Pion WebRTC (ORTC) API! Thanks for using it ❤️.

	// Prepare ICE gathering options
	iceOptions := webrtc.ICEGatherOptions{
		ICEServers: []webrtc.ICEServer{
			{URLs: []string{"stun:stun.l.google.com:19302"}},
		},
	}

	// Create an API object
	api := webrtc.NewAPI()

	// Create the ICE gatherer
	gatherer, err := api.NewICEGatherer(iceOptions)
	if err != nil {
		panic(err)
	}

	// Construct the ICE transport
	ice := api.NewICETransport(gatherer)

	// Construct the DTLS transport
	dtls, err := api.NewDTLSTransport(ice, nil)
	if err != nil {
		panic(err)
	}

	// Construct the SCTP transport
	sctp := api.NewSCTPTransport(dtls)

	// Handle incoming data channels
	sctp.OnDataChannel(func(channel *webrtc.DataChannel) {
		fmt.Printf("New DataChannel %s %d\n", channel.Label(), channel.ID())

		// Register the handlers
		channel.OnOpen(handleOnOpen(channel))
		channel.OnMessage(func(msg webrtc.DataChannelMessage) {
			fmt.Printf("Message from DataChannel '%s': '%s'\n", channel.Label(), string(msg.Data))
		})
	})

	gatherFinished := make(chan struct{})
	gatherer.OnLocalCandidate(func(candidate *webrtc.ICECandidate) {
		if candidate == nil {
			close(gatherFinished)
		}
	})

	// Gather candidates
	if err = gatherer.Gather(); err != nil {
		panic(err)
	}

	<-gatherFinished

	iceCandidates, err := gatherer.GetLocalCandidates()
	if err != nil {
		panic(err)
	}

	iceParams, err := gatherer.GetLocalParameters()
	if err != nil {
		panic(err)
	}

	dtlsParams, err := dtls.GetLocalParameters()
	if err != nil {
		panic(err)
	}

	sctpCapabilities := sctp.GetCapabilities()

	s := Signal{
		ICECandidates:    iceCandidates,
		ICEParameters:    iceParams,
		DTLSParameters:   dtlsParams,
		SCTPCapabilities: sctpCapabilities,
	}

	iceRole := webrtc.ICERoleControlled

	// Exchange the information
	fmt.Println(encode(s))
	remoteSignal := Signal{}

	if *isOffer {
		signalingChan := httpSDPServer(*port)
		decode(<-signalingChan, &remoteSignal)

		iceRole = webrtc.ICERoleControlling
	} else {
		decode(readUntilNewline(), &remoteSignal)
	}

	if err = ice.SetRemoteCandidates(remoteSignal.ICECandidates); err != nil {
		panic(err)
	}

	// Start the ICE transport
	err = ice.Start(nil, remoteSignal.ICEParameters, &iceRole)
	if err != nil {
		panic(err)
	}

	// Start the DTLS transport
	if err = dtls.Start(remoteSignal.DTLSParameters); err != nil {
		panic(err)
	}

	// Start the SCTP transport
	if err = sctp.Start(remoteSignal.SCTPCapabilities); err != nil {
		panic(err)
	}

	// Construct the data channel as the offerer
	if *isOffer {
		var id uint16 = 1

		dcParams := &webrtc.DataChannelParameters{
			Label: "Foo",
			ID:    &id,
		}
		var channel *webrtc.DataChannel
		channel, err = api.NewDataChannel(sctp, dcParams)
		if err != nil {
			panic(err)
		}

		// Register the handlers
		// channel.OnOpen(handleOnOpen(channel)) // TODO: OnOpen on handle ChannelAck
		go handleOnOpen(channel)() // Temporary alternative
		channel.OnMessage(func(msg webrtc.DataChannelMessage) {
			fmt.Printf("Message from DataChannel '%s': '%s'\n", channel.Label(), string(msg.Data))
		})
	}

	select {}
}

// Signal is used to exchange signaling info.
// This is not part of the ORTC spec. You are free
// to exchange this information any way you want.
type Signal struct {
	ICECandidates    []webrtc.ICECandidate   `json:"iceCandidates"`
	ICEParameters    webrtc.ICEParameters    `json:"iceParameters"`
	DTLSParameters   webrtc.DTLSParameters   `json:"dtlsParameters"`
	SCTPCapabilities webrtc.SCTPCapabilities `json:"sctpCapabilities"`
}

func handleOnOpen(channel *webrtc.DataChannel) func() {
	return func() {
		fmt.Printf(
			"Data channel '%s'-'%d' open. Random messages will now be sent to any connected DataChannels every 5 seconds\n",
			channel.Label(), channel.ID(),
		)

		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			message, err := randutil.GenerateCryptoRandomString(15, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
			if err != nil {
				panic(err)
			}

			fmt.Printf("Sending %s \n", message)
			if err := channel.SendText(message); err != nil {
				panic(err)
			}
		}
	}
}

// Read from stdin until we get a newline.
func readUntilNewline() (in string) {
	var err error

	r := bufio.NewReader(os.Stdin)
	for {
		in, err = r.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			panic(err)
		}

		if in = strings.TrimSpace(in); len(in) > 0 {
			break
		}
	}

	fmt.Println("")

	return
}

// JSON encode + base64 a SessionDescription.
func encode(obj Signal) string {
	b, err := json.Marshal(obj)
	if err != nil {
		panic(err)
	}

	return base64.StdEncoding.EncodeToString(b)
}

// Decode a base64 and unmarshal JSON into a SessionDescription.
func decode(in string, obj *Signal) {
	b, err := base64.StdEncoding.DecodeString(in)
	if err != nil {
		panic(err)
	}

	if err = json.Unmarshal(b, obj); err != nil {
		panic(err)
	}
}

// httpSDPServer starts a HTTP Server that consumes SDPs.
func httpSDPServer(port int) chan string {
	sdpChan := make(chan string)
	http.HandleFunc("/", func(res http.ResponseWriter, req *http.Request) {
		body, _ := io.ReadAll(req.Body)
		fmt.Fprintf(res, "done") //nolint: errcheck
		sdpChan <- string(body)
	})

	go func() {
		// nolint: gosec
		panic(http.ListenAndServe(":"+strconv.Itoa(port), nil))
	}()

	return sdpChan
}
