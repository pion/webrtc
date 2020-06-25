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

var peerConnection *webrtc.PeerConnection //nolint

// doSignaling exchanges all state of the local PeerConnection and is called
// every time a video is added or removed
func doSignaling(w http.ResponseWriter, r *http.Request) {
	var offer webrtc.SessionDescription
	if err := json.NewDecoder(r.Body).Decode(&offer); err != nil {
		panic(err)
	}

	if err := peerConnection.SetRemoteDescription(offer); err != nil {
		panic(err)
	}

	// Create channel that is blocked until ICE Gathering is complete
	gatherComplete := webrtc.GatheringCompletePromise(peerConnection)

	answer, err := peerConnection.CreateAnswer(nil)
	if err != nil {
		panic(err)
	} else if err = peerConnection.SetLocalDescription(answer); err != nil {
		panic(err)
	}

	// Block until ICE Gathering is complete, disabling trickle ICE
	// we do this because we only can exchange one signaling message
	// in a production application you should exchange ICE Candidates via OnICECandidate
	<-gatherComplete

	response, err := json.Marshal(*peerConnection.LocalDescription())
	if err != nil {
		panic(err)
	}

	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(response); err != nil {
		panic(err)
	}
}

// Add a single video track
func createPeerConnection(w http.ResponseWriter, r *http.Request) {
	if peerConnection.ConnectionState() != webrtc.PeerConnectionStateNew {
		panic(fmt.Sprintf("createPeerConnection called in non-new state (%s)", peerConnection.ConnectionState()))
	}

	doSignaling(w, r)
	fmt.Println("PeerConnection has been created")
}

// Add a single video track
func addVideo(w http.ResponseWriter, r *http.Request) {
	videoTrack, err := peerConnection.NewTrack(webrtc.DefaultPayloadTypeVP8, rand.Uint32(), fmt.Sprintf("video-%d", rand.Uint32()), fmt.Sprintf("video-%d", rand.Uint32()))
	if err != nil {
		panic(err)
	}
	if _, err = peerConnection.AddTrack(videoTrack); err != nil {
		panic(err)
	}

	go writeVideoToTrack(videoTrack)
	doSignaling(w, r)
	fmt.Println("Video track has been added")
}

// Remove a single sender
func removeVideo(w http.ResponseWriter, r *http.Request) {
	if senders := peerConnection.GetSenders(); len(senders) != 0 {
		if err := peerConnection.RemoveTrack(senders[0]); err != nil {
			panic(err)
		}
	}

	doSignaling(w, r)
	fmt.Println("Video track has been removed")
}

func main() {
	rand.Seed(time.Now().UTC().UnixNano())

	var err error
	if peerConnection, err = webrtc.NewPeerConnection(webrtc.Configuration{}); err != nil {
		panic(err)
	}

	// Set the handler for ICE connection state
	// This will notify you when the peer has connected/disconnected
	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		fmt.Printf("ICE Connection State has changed: %s\n", connectionState.String())
	})

	http.Handle("/", http.FileServer(http.Dir(".")))
	http.HandleFunc("/createPeerConnection", createPeerConnection)
	http.HandleFunc("/addVideo", addVideo)
	http.HandleFunc("/removeVideo", removeVideo)

	fmt.Println("Open http://localhost:8080 to access this demo")
	panic(http.ListenAndServe(":8080", nil))
}

// Read a video file from disk and write it to a webrtc.Track
// When the video has been completely read this exits without error
func writeVideoToTrack(t *webrtc.Track) {
	// Open a IVF file and start reading using our IVFReader
	file, err := os.Open("output.ivf")
	if err != nil {
		panic(err)
	}

	ivf, header, err := ivfreader.NewWith(file)
	if err != nil {
		panic(err)
	}

	// Send our video file frame at a time. Pace our sending so we send it at the same speed it should be played back as.
	// This isn't required since the video is timestamped, but we will such much higher loss if we send all at once.
	sleepTime := time.Millisecond * time.Duration((float32(header.TimebaseNumerator)/float32(header.TimebaseDenominator))*1000)
	for {
		frame, _, err := ivf.ParseNextFrame()
		if err != nil {
			fmt.Printf("Finish writing video track: %s ", err)
			return
		}

		time.Sleep(sleepTime)
		if err = t.WriteSample(media.Sample{Data: frame, Samples: 90000}); err != nil {
			fmt.Printf("Finish writing video track: %s ", err)
			return
		}
	}
}
