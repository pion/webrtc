// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js
// +build !js

// ortc demonstrates Pion WebRTC's ORTC capabilities.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/examples/internal/signal"
	"github.com/pion/webrtc/v4/pkg/media"
	"github.com/pion/webrtc/v4/pkg/media/ivfreader"
)

const (
	videoFileName = "output.ivf"
)

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

	// Use default Codecs
	m := &webrtc.MediaEngine{}
	if err := m.RegisterDefaultCodecs(); err != nil {
		panic(err)
	}

	// Create an API object
	api := webrtc.NewAPI(webrtc.WithMediaEngine(m))

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

	// Create a RTPSender or RTPReceiver
	var (
		rtpReceiver       *webrtc.RTPReceiver
		rtpSendParameters webrtc.RTPSendParameters
	)

	if *isOffer {
		// Open the video file
		file, fileErr := os.Open(videoFileName)
		if fileErr != nil {
			panic(fileErr)
		}

		// Read the header of the video file
		ivf, header, fileErr := ivfreader.NewWith(file)
		if fileErr != nil {
			panic(fileErr)
		}

		trackLocal := fourCCToTrack(header.FourCC)

		// Create RTPSender to send our video file
		rtpSender, fileErr := api.NewRTPSender(trackLocal, dtls)
		if fileErr != nil {
			panic(fileErr)
		}

		rtpSendParameters = rtpSender.GetParameters()

		if fileErr = rtpSender.Send(rtpSendParameters); fileErr != nil {
			panic(fileErr)
		}

		go writeFileToTrack(ivf, header, trackLocal)
	} else {
		if rtpReceiver, err = api.NewRTPReceiver(webrtc.RTPCodecTypeVideo, dtls); err != nil {
			panic(err)
		}
	}

	gatherFinished := make(chan struct{})
	gatherer.OnLocalCandidate(func(i *webrtc.ICECandidate) {
		if i == nil {
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

	s := Signal{
		ICECandidates:     iceCandidates,
		ICEParameters:     iceParams,
		DTLSParameters:    dtlsParams,
		RTPSendParameters: rtpSendParameters,
	}

	iceRole := webrtc.ICERoleControlled

	// Exchange the information
	fmt.Println(signal.Encode(s))
	remoteSignal := Signal{}

	if *isOffer {
		signalingChan := signal.HTTPSDPServer(*port)
		signal.Decode(<-signalingChan, &remoteSignal)

		iceRole = webrtc.ICERoleControlling
	} else {
		signal.Decode(signal.MustReadStdin(), &remoteSignal)
	}

	if err = ice.SetRemoteCandidates(remoteSignal.ICECandidates); err != nil {
		panic(err)
	}

	// Start the ICE transport
	if err = ice.Start(nil, remoteSignal.ICEParameters, &iceRole); err != nil {
		panic(err)
	}

	// Start the DTLS transport
	if err = dtls.Start(remoteSignal.DTLSParameters); err != nil {
		panic(err)
	}

	if !*isOffer {
		if err = rtpReceiver.Receive(webrtc.RTPReceiveParameters{
			Encodings: []webrtc.RTPDecodingParameters{
				{
					RTPCodingParameters: remoteSignal.RTPSendParameters.Encodings[0].RTPCodingParameters,
				},
			},
		}); err != nil {
			panic(err)
		}

		remoteTrack := rtpReceiver.Track()
		pkt, _, err := remoteTrack.ReadRTP()
		if err != nil {
			panic(err)
		}

		fmt.Printf("Got RTP Packet with SSRC %d \n", pkt.SSRC)
	}

	select {}
}

// Given a FourCC value return a Track
func fourCCToTrack(fourCC string) *webrtc.TrackLocalStaticSample {
	// Determine video codec
	var trackCodec string
	switch fourCC {
	case "AV01":
		trackCodec = webrtc.MimeTypeAV1
	case "VP90":
		trackCodec = webrtc.MimeTypeVP9
	case "VP80":
		trackCodec = webrtc.MimeTypeVP8
	default:
		panic(fmt.Sprintf("Unable to handle FourCC %s", fourCC))
	}

	// Create a video Track with the codec of the file
	trackLocal, err := webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{MimeType: trackCodec}, "video", "pion")
	if err != nil {
		panic(err)
	}

	return trackLocal
}

// Write a file to Track
func writeFileToTrack(ivf *ivfreader.IVFReader, header *ivfreader.IVFFileHeader, track *webrtc.TrackLocalStaticSample) {
	ticker := time.NewTicker(time.Millisecond * time.Duration((float32(header.TimebaseNumerator)/float32(header.TimebaseDenominator))*1000))
	for ; true; <-ticker.C {
		frame, _, err := ivf.ParseNextFrame()
		if errors.Is(err, io.EOF) {
			fmt.Printf("All video frames parsed and sent")
			os.Exit(0)
		}

		if err != nil {
			panic(err)
		}

		if err = track.WriteSample(media.Sample{Data: frame, Duration: time.Second}); err != nil {
			panic(err)
		}
	}
}

// Signal is used to exchange signaling info.
// This is not part of the ORTC spec. You are free
// to exchange this information any way you want.
type Signal struct {
	ICECandidates     []webrtc.ICECandidate    `json:"iceCandidates"`
	ICEParameters     webrtc.ICEParameters     `json:"iceParameters"`
	DTLSParameters    webrtc.DTLSParameters    `json:"dtlsParameters"`
	RTPSendParameters webrtc.RTPSendParameters `json:"rtpSendParameters"`
}
