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

	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"
	"github.com/pion/webrtc/v4/pkg/media/ivfreader"
)

const (
	videoFileName = "output.ivf"
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

	// Use default Codecs
	mediaEngine := &webrtc.MediaEngine{}
	if err := mediaEngine.RegisterDefaultCodecs(); err != nil {
		panic(err)
	}

	// Create an API object
	api := webrtc.NewAPI(webrtc.WithMediaEngine(mediaEngine))

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

	if *isOffer { //nolint:nestif
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

	signal := Signal{
		ICECandidates:     iceCandidates,
		ICEParameters:     iceParams,
		DTLSParameters:    dtlsParams,
		RTPSendParameters: rtpSendParameters,
	}

	iceRole := webrtc.ICERoleControlled

	// Exchange the information
	fmt.Println(encode(&signal))
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

// Given a FourCC value return a Track.
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

// Write a file to Track.
func writeFileToTrack(ivf *ivfreader.IVFReader, header *ivfreader.IVFFileHeader, track *webrtc.TrackLocalStaticSample) {
	ticker := time.NewTicker(
		time.Millisecond * time.Duration((float32(header.TimebaseNumerator)/float32(header.TimebaseDenominator))*1000),
	)
	defer ticker.Stop()
	for ; true; <-ticker.C {
		frame, _, err := ivf.ParseNextFrame()
		if errors.Is(err, io.EOF) {
			fmt.Printf("All video frames parsed and sent")
			os.Exit(0) //nolint: gocritic
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
func encode(obj *Signal) string {
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
