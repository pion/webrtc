package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/pion/webrtc/v3"
	"github.com/pion/webrtc/v3/pkg/media"
	"github.com/pion/webrtc/v3/pkg/media/ivfreader"
)

var pc *webrtc.PeerConnection      // nolint
var signal = make(chan []byte, 1)  // nolint
var channels []*webrtc.DataChannel // nolint

type message struct {
	Type        string                    `json:"type"`
	Candidate   webrtc.ICECandidateInit   `json:"candidate"`
	Description webrtc.SessionDescription `json:"description"`
}

// Type message
const (
	MessageTypeICE           = "ice-candidate"
	MessageTypeOffer         = "offer"
	MessageTypeAnswer        = "answer"
	MessageTypeNewTrack      = "add-track"
	MessageTypeRemoveTrack   = "remove-track"
	MessageTypeAddChannel    = "add-channel"
	MessageTypeRemoveChannel = "remove-channel"
)

func check(err error) {
	if err != nil {
		panic(err)
	}
}

func writeVideoToTrack(track *webrtc.Track) {
	// Open a IVF file and start reading using our IVFReader
	file, err := os.Open("output.ivf")
	check(err)

	ivf, header, err := ivfreader.NewWith(file)
	check(err)

	// Send our video file frame at a time. Pace our sending so we send it at the same speed it should be played back as.
	// This isn't required since the video is timestamped, but we will such much higher loss if we send all at once.
	sleepTime := time.Millisecond * time.Duration((float32(header.TimebaseNumerator)/float32(header.TimebaseDenominator))*1000)
	for {
		frame, _, err := ivf.ParseNextFrame()
		if err != nil {
			go writeVideoToTrack(track)
			return
		}

		time.Sleep(sleepTime)
		if err = track.WriteSample(media.Sample{Data: frame, Samples: 90000}); err != nil {
			fmt.Printf("Finish writing video track: %s ", err)
			return
		}
	}
}

func clientSay(w http.ResponseWriter, r *http.Request) {
	var msg message
	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		panic(err)
	}

	switch msg.Type {
	case MessageTypeICE:
		err := pc.AddICECandidate(msg.Candidate)
		check(err)
	case MessageTypeOffer:
		var err error
		err = pc.SetRemoteDescription(msg.Description)
		check(err)
		answer, err := pc.CreateAnswer(nil)
		check(err)

		err = pc.SetLocalDescription(answer)
		check(err)

		b, err := json.Marshal(message{Type: MessageTypeAnswer, Description: answer})
		check(err)
		signal <- b
	case MessageTypeAnswer:
		err := pc.SetRemoteDescription(msg.Description)
		check(err)
	case MessageTypeRemoveTrack:
		senders := pc.GetSenders()
		if len(senders) == 0 {
			fmt.Print("nothing more to remove")
			break
		}

		err := pc.RemoveTrack(senders[0])
		check(err)
	case MessageTypeNewTrack:
		track, err := pc.NewTrack(webrtc.DefaultPayloadTypeVP8, rand.Uint32(), fmt.Sprintf("video-%d", rand.Uint32()), fmt.Sprintf("video-%d", rand.Uint32()))
		check(err)

		_, err = pc.AddTrack(track)
		check(err)

		go writeVideoToTrack(track)
	case MessageTypeAddChannel:
		channel, err := pc.CreateDataChannel(fmt.Sprintf("channel-server-%d", rand.Uint32()), nil)
		check(err)
		channels = append(channels, channel)
	case MessageTypeRemoveChannel:
		if len(channels) == 0 {
			return
		}
		// pop first channel
		var channel *webrtc.DataChannel
		channel, channels = channels[0], channels[1:]
		check(channel.Close())
	default:
		fmt.Printf("Unknown message type: %s ", msg.Type)
	}
	w.WriteHeader(http.StatusFound)
}

func serverSay(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("content-type", "application/json")
	_, err := w.Write(<-signal)
	if err != nil {
		fmt.Println(err)
	}
}

func main() {
	var err error
	pc, err = webrtc.NewPeerConnection(
		webrtc.Configuration{
			ICEServers: []webrtc.ICEServer{
				{
					URLs: []string{"stun:stun.l.google.com:19302"},
				},
			},
		},
	)
	check(err)

	pc.OnNegotiationNeeded(func() {
		// Dont use this, see
		// https://developer.mozilla.org/en-US/docs/Web/API/WebRTC_API/Perfect_negotiation
		offer, err := pc.CreateOffer(nil)
		check(err)
		err = pc.SetLocalDescription(offer)
		check(err)
		msg := message{
			Type:        MessageTypeOffer,
			Description: offer,
		}
		b, err := json.Marshal(msg)
		check(err)
		signal <- b
	})

	pc.OnICECandidate(func(ice *webrtc.ICECandidate) {
		if ice == nil {
			return
		}
		msg := message{
			Type:      MessageTypeICE,
			Candidate: ice.ToJSON(),
		}
		b, err := json.Marshal(msg)
		check(err)
		signal <- b
	})

	pc.OnTrack(func(track *webrtc.Track, _ *webrtc.RTPReceiver) {
		for {
			if _, err := track.ReadRTP(); err != nil {
				fmt.Println("remote track send error =>", err)
				return
			}
		}
	})

	http.Handle("/", http.FileServer(http.Dir(".")))
	http.HandleFunc("/client-say", clientSay)
	http.HandleFunc("/server-say", serverSay)
	// http.Handle("/ws", websocket.Handler(handleWS))

	if err := http.ListenAndServe(":8080", nil); err != nil {
		fmt.Println(err)
	}
}
