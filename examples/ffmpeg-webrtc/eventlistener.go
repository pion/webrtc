package main

import (
	"encoding/json"
	"fmt"
	"math/rand"

	"github.com/pion/webrtc/v2"
)

var (
	offer        = "1"
	answer       = "2"
	off          = "3"
	iceCandidate = "4"
)

func (c *Camera) EventListener(done chan bool, room *Room) {
	for {
		select {
		case <-done:
			if c.on {
				c.Stop()
			}
			return
		case m := <-room.Inbound:
			switch m.Type {
			case offer:

				remoteSdp := webrtc.SessionDescription{}

				remoteSDP, err := json.Marshal(m.Message)
				if err != nil {
					fmt.Println("could not marshal offer. ", err)
					continue
				}

				if err := json.Unmarshal(remoteSDP, &remoteSdp); err != nil {
					fmt.Println("could not unmarshal sdp offer. ", err)
					continue
				}

				fmt.Printf("%+v", remoteSdp)

				mediaEngine := webrtc.MediaEngine{}
				if err := mediaEngine.PopulateFromSDP(remoteSdp); err != nil {
					fmt.Println("webrtc could not create media engine.", err)
					continue
				}

				var h264PayloadType uint8
				for _, videoCodec := range mediaEngine.GetCodecsByKind(webrtc.RTPCodecTypeVideo) {
					if videoCodec.Name == "H264" {
						h264PayloadType = videoCodec.PayloadType
						break
					}
				}

				if h264PayloadType == 0 {
					fmt.Println("Remote peer does not support H264")
					continue
				}

				fmt.Println("received offer")
				peerConnection, err := c.NewPeerConnection(mediaEngine)
				if err != nil {
					fmt.Println(err)
					continue
				}

				peerConnection.OnICECandidate(func(ice *webrtc.ICECandidate) {
					if ice == nil {
						return
					}

					fmt.Println("trickle ice candidate: ", ice)
					room.Outbound <- &Message{
						Type:    iceCandidate,
						Message: ice.ToJSON(),
					}
				})

				peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
					fmt.Printf("connection state has changed %s \n", connectionState.String())
				})

				if err := peerConnection.SetRemoteDescription(remoteSdp); err != nil {
					fmt.Println("could not set remote description.", err)
				}

				videoTrack, err := peerConnection.NewTrack(h264PayloadType, rand.Uint32(), "usb_cam", c.Name)
				if err != nil {
					fmt.Println("webrtc could not create video track.", err)
					continue
				}

				_, err = peerConnection.AddTrack(videoTrack)
				if err != nil {
					fmt.Println("webrtc could not add video track.", err)
					continue
				}

				direction := webrtc.RtpTransceiverInit{
					Direction: webrtc.RTPTransceiverDirectionSendonly,
				}

				_, err = peerConnection.AddTransceiverFromTrack(videoTrack, direction)
				if err != nil {
					fmt.Println("webrtc could not set transceiver direction. ", err)
					continue
				}

				answr, err := peerConnection.CreateAnswer(nil)
				if err != nil {
					fmt.Println("webrtc could not create answer. ", err)
					continue
				}

				if err = peerConnection.SetLocalDescription(answr); err != nil {
					fmt.Println("webrtc could not set local description")
					continue
				}

				room.Outbound <- &Message{
					Type:    answer,
					Message: answr,
				}

				c.Stream(videoTrack)
			case iceCandidate:
				fmt.Println("received ice candidate")

				candidate := webrtc.ICECandidateInit{}

				bytes, err := json.Marshal(m.Message)
				if err != nil {
					fmt.Println("could not marshal ice candidate. ", err)
					continue
				}

				if err := json.Unmarshal(bytes, &candidate); err != nil {
					fmt.Println("could not unmarshal ice candidate.", err)
				}

				fmt.Println("candidate:", candidate)

				c.AddICECandidate(candidate)
			case off:
				c.Stop()
			}
		}
	}
}
