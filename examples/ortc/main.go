package main

import (
	"flag"
	"fmt"
	"time"

	"github.com/pions/webrtc"
	"github.com/pions/webrtc/examples/util"
	"github.com/pions/webrtc/pkg/datachannel"
)

func main() {
	isOffer := flag.Bool("offer", false, "Act as the offerer if set")
	flag.Parse()

	// Everything below is the pion-WebRTC (ORTC) API! Thanks for using it ❤️.

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
	util.Check(err)

	// Construct the ICE transport
	ice := api.NewICETransport(gatherer)

	// Construct the DTLS transport
	dtls, err := api.NewDTLSTransport(ice, nil)
	util.Check(err)

	// Construct the SCTP transport
	sctp := api.NewSCTPTransport(dtls)

	// Handle incoming data channels
	sctp.OnDataChannel(func(channel *webrtc.DataChannel) {
		fmt.Printf("New DataChannel %s %d\n", channel.Label, channel.ID)

		// Register the handlers
		channel.OnOpen(handleOnOpen(channel))
		channel.OnMessage(handleMessage(channel))
	})

	// Gather candidates
	err = gatherer.Gather()
	util.Check(err)

	iceCandidates, err := gatherer.GetLocalCandidates()
	util.Check(err)

	iceParams, err := gatherer.GetLocalParameters()
	util.Check(err)

	dtlsParams := dtls.GetLocalParameters()

	sctpCapabilities := sctp.GetCapabilities()

	signal := Signal{
		ICECandidates:    iceCandidates,
		ICEParameters:    iceParams,
		DTLSParameters:   dtlsParams,
		SCTPCapabilities: sctpCapabilities,
	}

	// Exchange the information
	fmt.Println(util.Encode(signal))
	remoteSignal := Signal{}
	util.Decode(util.MustReadStdin(), &remoteSignal)

	iceRole := webrtc.ICERoleControlled
	if *isOffer {
		iceRole = webrtc.ICERoleControlling
	}

	err = ice.SetRemoteCandidates(remoteSignal.ICECandidates)
	util.Check(err)

	// Start the ICE transport
	err = ice.Start(nil, remoteSignal.ICEParameters, &iceRole)
	util.Check(err)

	// Start the DTLS transport
	err = dtls.Start(remoteSignal.DTLSParameters)
	util.Check(err)

	// Start the SCTP transport
	err = sctp.Start(remoteSignal.SCTPCapabilities)
	util.Check(err)

	// Construct the data channel as the offerer
	if *isOffer {
		dcParams := &webrtc.DataChannelParameters{
			Label: "Foo",
			ID:    1,
		}
		var channel *webrtc.DataChannel
		channel, err = api.NewDataChannel(sctp, dcParams)
		util.Check(err)

		// Register the handlers
		// channel.OnOpen(handleOnOpen(channel)) // TODO: OnOpen on handle ChannelAck
		go handleOnOpen(channel)() // Temporary alternative
		channel.OnMessage(handleMessage(channel))
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
		fmt.Printf("Data channel '%s'-'%d' open. Random messages will now be sent to any connected DataChannels every 5 seconds\n", channel.Label, channel.ID)

		for range time.NewTicker(5 * time.Second).C {
			message := util.RandSeq(15)
			fmt.Printf("Sending %s \n", message)

			err := channel.Send(datachannel.PayloadString{Data: []byte(message)})
			util.Check(err)
		}
	}
}

func handleMessage(channel *webrtc.DataChannel) func(datachannel.Payload) {
	return func(payload datachannel.Payload) {
		switch p := payload.(type) {
		case *datachannel.PayloadString:
			fmt.Printf("Message '%s' from DataChannel '%s' payload '%s'\n", p.PayloadType().String(), channel.Label, string(p.Data))
		case *datachannel.PayloadBinary:
			fmt.Printf("Message '%s' from DataChannel '%s' payload '% 02x'\n", p.PayloadType().String(), channel.Label, p.Data)
		default:
			fmt.Printf("Message '%s' from DataChannel '%s' no payload \n", p.PayloadType().String(), channel.Label)
		}
	}
}
