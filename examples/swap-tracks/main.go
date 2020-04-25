package main

import (
	"fmt"
	"io"
	"math/rand"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v2"
	"github.com/pion/webrtc/v2/examples/internal/signal"
)

func main() {
	// Everything below is the Pion WebRTC API! Thanks for using it ❤️.

	// Wait for the offer to be pasted
	offer := webrtc.SessionDescription{}
	signal.Decode(signal.MustReadStdin(), &offer)

	// We make our own mediaEngine so we can place the sender's codecs in it. Since we are echoing their RTP packet
	// back to them we are actually codec agnostic - we can accept all their codecs. This also ensures that we use the
	// dynamic media type from the sender in our answer.
	mediaEngine := webrtc.MediaEngine{}

	// Add codecs to the mediaEngine. Note that even though we are only going to echo back the sender's video we also
	// add audio codecs. This is because createAnswer will create an audioTransceiver and associated SDP and we currently
	// cannot tell it not to. The audio SDP must match the sender's codecs too...
	err := mediaEngine.PopulateFromSDP(offer)
	if err != nil {
		panic(err)
	}

	videoCodecs := mediaEngine.GetCodecsByKind(webrtc.RTPCodecTypeVideo)
	if len(videoCodecs) == 0 {
		panic("Offer contained no video codecs")
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

	// Create Track that we send video back to browser on
	outputTrack, err := peerConnection.NewTrack(videoCodecs[0].PayloadType, rand.Uint32(), "video", "pion")
	if err != nil {
		panic(err)
	}

	// Add this newly created track to the PeerConnection
	if _, err = peerConnection.AddTrack(outputTrack); err != nil {
		panic(err)
	}

	// In addition to the implicit transceiver added by the track, we add two more
	// for the other tracks
	_, err = peerConnection.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo,
		webrtc.RtpTransceiverInit{Direction: webrtc.RTPTransceiverDirectionRecvonly})
	if err != nil {
		panic(err)
	}
	_, err = peerConnection.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo,
		webrtc.RtpTransceiverInit{Direction: webrtc.RTPTransceiverDirectionRecvonly})
	if err != nil {
		panic(err)
	}

	// Set the remote SessionDescription
	err = peerConnection.SetRemoteDescription(offer)
	if err != nil {
		panic(err)
	}

	// Which track is currently being handled
	currTrack := 0
	// The total number of tracks
	trackCount := 0
	// The channel of packets with a bit of buffer
	packets := make(chan *rtp.Packet, 60)

	// Set a handler for when a new remote track starts
	peerConnection.OnTrack(func(track *webrtc.Track, receiver *webrtc.RTPReceiver) {
		fmt.Printf("Track has started, of type %d: %s \n", track.PayloadType(), track.Codec().Name)
		trackNum := trackCount
		trackCount++
		// The last timestamp so that we can change the packet to only be the delta
		var lastTimestamp uint32

		// Whether this track is the one currently sending to the channel (on change
		// of this we send a PLI to have the entire picture updated)
		var isCurrTrack bool
		for {
			// Read RTP packets being sent to Pion
			rtp, readErr := track.ReadRTP()
			if readErr != nil {
				panic(readErr)
			}

			// Change the timestamp to only be the delta
			oldTimestamp := rtp.Timestamp
			if lastTimestamp == 0 {
				rtp.Timestamp = 0
			} else {
				rtp.Timestamp -= lastTimestamp
			}
			lastTimestamp = oldTimestamp

			// Check if this is the current track
			if currTrack == trackNum {
				// If just switched to this track, send PLI to get picture refresh
				if !isCurrTrack {
					isCurrTrack = true
					if writeErr := peerConnection.WriteRTCP([]rtcp.Packet{&rtcp.PictureLossIndication{MediaSSRC: track.SSRC()}}); writeErr != nil {
						fmt.Println(writeErr)
					}
				}
				packets <- rtp
			} else {
				isCurrTrack = false
			}
		}
	})
	// Set the handler for ICE connection state and update chan if connected
	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		fmt.Printf("Connection State has changed %s \n", connectionState.String())
	})

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
	fmt.Printf("Paste below base64 in browser:\n%v\n", signal.Encode(answer))

	// Asynchronously take all packets in the channel and write them out to our
	// track
	go func() {
		var currTimestamp uint32
		for i := uint16(0); ; i++ {
			packet := <-packets
			// Timestamp on the packet is really a diff, so add it to current
			currTimestamp += packet.Timestamp
			packet.Timestamp = currTimestamp
			// Set the output SSRC
			packet.SSRC = outputTrack.SSRC()
			// Keep an increasing sequence number
			packet.SequenceNumber = i
			// Write out the packet, ignoring closed pipe if nobody is listening
			if err := outputTrack.WriteRTP(packet); err != nil && err != io.ErrClosedPipe {
				panic(err)
			}
		}
	}()

	// Wait for connection, then rotate the track every 5s
	fmt.Printf("Waiting for connection\n")
	for {
		// We haven't gotten any tracks yet
		if trackCount == 0 {
			continue
		}

		fmt.Printf("Waiting 5 seconds then changing...\n")
		time.Sleep(5 * time.Second)
		if currTrack == trackCount-1 {
			currTrack = 0
		} else {
			currTrack++
		}
		fmt.Printf("Switched to track #%v\n", currTrack+1)
	}
}
