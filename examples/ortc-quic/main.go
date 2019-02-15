package main

import (
	"flag"
	"fmt"
	"time"

	"github.com/pions/webrtc/pkg/quic"

	"github.com/pions/webrtc"
	"github.com/pions/webrtc/examples/util"
)

const messageSize = 15

func main() {
	isOffer := flag.Bool("offer", false, "Act as the offerer if set")
	flag.Parse()

	// This example shows off the experimental implementation of webrtc-quic.

	// Everything below is the pion-WebRTC (ORTC) API! Thanks for using it ❤️.

	// Create an API object
	api := webrtc.NewAPI()

	// Prepare ICE gathering options
	iceOptions := webrtc.ICEGatherOptions{
		ICEServers: []webrtc.ICEServer{
			{URLs: []string{"stun:stun.l.google.com:19302"}},
		},
	}

	// Create the ICE gatherer
	gatherer, err := api.NewICEGatherer(iceOptions)
	util.Check(err)

	// Construct the ICE transport
	ice := api.NewICETransport(gatherer)

	// Construct the Quic transport
	qt, err := api.NewQUICTransport(ice, nil)
	util.Check(err)

	// Handle incoming streams
	qt.OnBidirectionalStream(func(stream *quic.BidirectionalStream) {
		fmt.Printf("New stream %d\n", stream.StreamID())

		// Handle reading from the stream
		go ReadLoop(stream)

		// Handle writing to the stream
		go WriteLoop(stream)
	})

	// Gather candidates
	err = gatherer.Gather()
	util.Check(err)

	iceCandidates, err := gatherer.GetLocalCandidates()
	util.Check(err)

	iceParams, err := gatherer.GetLocalParameters()
	util.Check(err)

	quicParams := qt.GetLocalParameters()

	signal := Signal{
		ICECandidates:  iceCandidates,
		ICEParameters:  iceParams,
		QuicParameters: quicParams,
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

	// Start the Quic transport
	err = qt.Start(remoteSignal.QuicParameters)
	util.Check(err)

	// Construct the stream as the offerer
	if *isOffer {
		var stream *quic.BidirectionalStream
		stream, err = qt.CreateBidirectionalStream()
		util.Check(err)

		// Handle reading from the stream
		go ReadLoop(stream)

		// Handle writing to the stream
		go WriteLoop(stream)
	}

	select {}
}

// Signal is used to exchange signaling info.
// This is not part of the ORTC spec. You are free
// to exchange this information any way you want.
type Signal struct {
	ICECandidates  []webrtc.ICECandidate `json:"iceCandidates"`
	ICEParameters  webrtc.ICEParameters  `json:"iceParameters"`
	QuicParameters webrtc.QUICParameters `json:"quicParameters"`
}

// ReadLoop reads from the stream
func ReadLoop(s *quic.BidirectionalStream) {
	for {
		buffer := make([]byte, messageSize)
		params, err := s.ReadInto(buffer)
		util.Check(err)

		fmt.Printf("Message from stream '%d': %s\n", s.StreamID(), string(buffer[:params.Amount]))
	}
}

// WriteLoop writes to the stream
func WriteLoop(s *quic.BidirectionalStream) {
	for range time.NewTicker(5 * time.Second).C {
		message := util.RandSeq(messageSize)
		fmt.Printf("Sending %s \n", message)

		data := quic.StreamWriteParameters{
			Data: []byte(message),
		}
		err := s.Write(data)
		util.Check(err)
	}
}
