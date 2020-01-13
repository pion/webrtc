package main

import (
	"fmt"
	"math/rand"

	"github.com/pion/webrtc/v2"
	"github.com/pion/webrtc/v2/pkg/media"

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

	// Search for H264 Payload type. If the offer doesn't support H264 exit since
	// since they won't be able to decode anything we send them
	var payloadType uint8
	for _, videoCodec := range mediaEngine.GetCodecsByKind(webrtc.RTPCodecTypeVideo) {
		if videoCodec.Name == "H264" {
			payloadType = videoCodec.PayloadType
			break
		}
	}
	if payloadType == 0 {
		panic("Remote peer does not support H264")
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

	// Create a video track
	videoTrack, err := peerConnection.NewTrack(payloadType, rand.Uint32(), "video", "pion")
	if err != nil {
		panic(err)
	}
	if _, err = peerConnection.AddTrack(videoTrack); err != nil {
		panic(err)
	}

	go func() {
		var nalStream = make(chan Nal)
		fps := uint32(25)

		// to generate a h264 video sample please use sample.sh
		go loadFile("output.h264", nalStream, fps)

		for {
			nal := <-nalStream
			Samples := uint32(0)
			if nal.UnitType == CodedSliceNonIdr || nal.UnitType == CodedSliceIdr {
				Samples = 90000 / fps
			}

			// send only Idr, NonIdr, SPS, PPS
			if nal.UnitType == CodedSliceNonIdr || nal.UnitType == CodedSliceIdr ||
				nal.UnitType == SPS || nal.UnitType == PPS {
				frame := nal.Data
				// prepend 0x00_00_00_01 prefix if it doesn't not exist
				if !((frame[0] == 0x00 && frame[1] == 0x00 && frame[2] == 0x01) ||
					(frame[0] == 0x00 && frame[1] == 0x00 && frame[2] == 0x00 && frame[3] == 0x01)) {
					frame = append([]byte{0x00, 0x00, 0x00, 0x01}, frame...)
				}
				// fmt.Println("nal: ", NalUnitTypeStr(nal.UnitType))
				if ivfErr := videoTrack.WriteSample(media.Sample{Data: frame, Samples: Samples}); ivfErr != nil {
					panic(ivfErr)
				}
			}
		}
	}()

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

	// Block forever
	select {}
}
