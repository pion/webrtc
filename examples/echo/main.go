package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/sdp/v2"
	"github.com/pion/webrtc/v2"
	"github.com/pion/webrtc/v2/examples/internal/signal"
)

func main() {
	// Everything below is the Pion WebRTC API! Thanks for using it ❤️.

	// Wait for the offer to be pasted
	offer := webrtc.SessionDescription{}
	signal.Decode(signal.MustReadStdin(), &offer)
	var offerSD sdp.SessionDescription
	err := offerSD.Unmarshal([]byte(offer.SDP))
	if err != nil {
		panic(err)
	}
	// We make our own mediaEngine so we can place the sender's codecs in it. Since we are echoing their RTP packet
	// back to them we are actually codec agnostic - we can accept all their codecs. This also ensures that we use the
	// dynamic media type from the sender in our answer.
	mediaEngine := webrtc.MediaEngine{}

	// Add codecs to the mediaEngine. Note that even though we are only going to echo back the sender's video we also
	// add audio codecs. This is because createAnswer will create an audioTransceiver and associated SDP and we currently
	// cannot tell it not to. The audio SDP must match the sender's codecs too...
	err = setCodecsFromOffer(offerSD, &mediaEngine)
	if err != nil {
		panic(err)
	}
	api := webrtc.NewAPI(webrtc.WithMediaEngine(mediaEngine))

	// Prepare the configuration
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	}
	// Create a new RTCPeerConnection
	peerConnection, err := api.NewPeerConnection(config)
	if err != nil {
		panic(err)
	}

	// Set the remote SessionDescription
	err = peerConnection.SetRemoteDescription(offer)
	if err != nil {
		panic(err)
	}

	// Transceivers for audio and video are created by CreateAnswer if not present.
	// We create the video transceiver here so we can get its sender's SSRC
	transceiver, err := peerConnection.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo)
	if err != nil {
		panic(err)
	}

	// Create an answer
	answer, err := peerConnection.CreateAnswer(nil)
	if err != nil {
		panic(err)
	}

	// Sets the LocalDescription, and starts our UDP listeners
	err = peerConnection.SetLocalDescription(answer)
	if err != nil {
		panic(err)
	}

	// Output the answer in base64 so we can paste it in browser
	fmt.Println(signal.Encode(answer))

	outputTrack := transceiver.Sender.Track()
	// Set a handler for when a new remote track starts, this handler copies inbound RTP packets,
	// replaces the SSRC and sends them back
	peerConnection.OnTrack(func(track *webrtc.Track, receiver *webrtc.RTPReceiver) {
		// Send a PLI on an interval so that the publisher is pushing a keyframe every rtcpPLIInterval
		// This is a temporary fix until we implement incoming RTCP events, then we would push a PLI only when a viewer requests it
		go func() {
			ticker := time.NewTicker(time.Second * 3)
			for range ticker.C {
				errSend := peerConnection.WriteRTCP([]rtcp.Packet{&rtcp.PictureLossIndication{MediaSSRC: track.SSRC()}})
				if errSend != nil {
					fmt.Println(errSend)
				}
			}
		}()

		fmt.Printf("Track has started, of type %d: %s \n", track.PayloadType(), track.Codec().Name)
		for {
			// Read RTP packets being sent to Pion
			rtp, readErr := track.ReadRTP()
			if readErr != nil {
				panic(readErr)
			}

			// Replace the SSRC with the SSRC of the outbound track.
			// The only change we are making replacing the SSRC, the RTP packets are unchanged otherwise
			rtp.SSRC = outputTrack.SSRC()

			if writeErr := outputTrack.WriteRTP(rtp); writeErr != nil {
				panic(writeErr)
			}
		}
	})
	// Set the handler for ICE connection state
	// This will notify you when the peer has connected/disconnected
	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		fmt.Printf("Connection State has changed %s \n", connectionState.String())
	})

	// Block forever
	select {}
	}

// setCodecsFromOffer finds all codecs in a session description and adds them to a MediaEngine, using dynamic
// payload types and parameters from the sdp
func setCodecsFromOffer(offerSD sdp.SessionDescription, mediaEngine *webrtc.MediaEngine) error {
	for _, md := range offerSD.MediaDescriptions {
		formats, err := md.MediaFormats()
	if err != nil {
			return err
	}
		for _, format := range formats {
			if format.MediaType == "video" || format.MediaType == "audio" {
				splits := strings.Split(format.EncodingName, "/")
				if len(splits) < 2 {
					return fmt.Errorf("unexpected encoding name %s", format.EncodingName)
				}
				codecName := splits[0]
				cr, err := strconv.Atoi(splits[1])
	if err != nil {
					return fmt.Errorf("couldn't extract integer clock rate from encoding name %s", format.EncodingName)
	}
				clockRate := uint32(cr)
				payloadType := uint8(format.PayloadType)
				var codec *webrtc.RTPCodec
				switch codecName {
				case webrtc.G722:
					codec = webrtc.NewRTPG722Codec(payloadType, clockRate)
				case webrtc.Opus:
					codec = webrtc.NewRTPOpusCodec(payloadType, clockRate)
				case webrtc.VP8:
					codec = webrtc.NewRTPVP8Codec(payloadType, clockRate)
					codec.SDPFmtpLine = format.Parameters
				case webrtc.VP9:
					codec = webrtc.NewRTPVP9Codec(payloadType, clockRate)
					codec.SDPFmtpLine = format.Parameters
				case webrtc.H264:
					codec = webrtc.NewRTPH264Codec(payloadType, clockRate)
					codec.SDPFmtpLine = format.Parameters
				default:
					//fmt.Printf("ignoring offer codec %s\n", codecName)
					continue
				}
				mediaEngine.RegisterCodec(codec)
			}
		}
	}
	return nil
}
