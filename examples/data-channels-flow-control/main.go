package main

import (
	"encoding/json"
	"log"
	"sync/atomic"
	"time"

	"github.com/pion/webrtc/v3"
)

const (
	bufferedAmountLowThreshold uint64 = 512 * 1024  // 512 KB
	maxBufferedAmount          uint64 = 1024 * 1024 // 1 MB
)

func check(err error) {
	if err != nil {
		panic(err)
	}
}

func setRemoteDescription(pc *webrtc.PeerConnection, sdp []byte) {
	var desc webrtc.SessionDescription
	err := json.Unmarshal(sdp, &desc)
	check(err)

	// Apply the desc as the remote description
	err = pc.SetRemoteDescription(desc)
	check(err)
}

func createOfferer() *webrtc.PeerConnection {
	// Prepare the configuration
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{},
	}

	// Create a new PeerConnection
	pc, err := webrtc.NewPeerConnection(config)
	check(err)

	buf := make([]byte, 1024)

	ordered := false
	maxRetransmits := uint16(0)

	options := &webrtc.DataChannelInit{
		Ordered:        &ordered,
		MaxRetransmits: &maxRetransmits,
	}

	sendMoreCh := make(chan struct{})

	// Create a datachannel with label 'data'
	dc, err := pc.CreateDataChannel("data", options)
	check(err)

	// Register channel opening handling
	dc.OnOpen(func() {
		log.Printf("OnOpen: %s-%d. Start sending a series of 1024-byte packets as fast as it can\n", dc.Label(), dc.ID())

		for {
			err2 := dc.Send(buf)
			check(err2)

			if dc.BufferedAmount()+uint64(len(buf)) > maxBufferedAmount {
				// Wait until the bufferedAmount becomes lower than the threshold
				<-sendMoreCh
			}
		}
	})

	// Set bufferedAmountLowThreshold so that we can get notified when
	// we can send more
	dc.SetBufferedAmountLowThreshold(bufferedAmountLowThreshold)

	// This callback is made when the current bufferedAmount becomes lower than the threadshold
	dc.OnBufferedAmountLow(func() {
		sendMoreCh <- struct{}{}
	})

	return pc
}

func createAnswerer() *webrtc.PeerConnection {
	// Prepare the configuration
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{},
	}

	// Create a new PeerConnection
	pc, err := webrtc.NewPeerConnection(config)
	check(err)

	pc.OnDataChannel(func(dc *webrtc.DataChannel) {
		var totalBytesReceived uint64

		// Register channel opening handling
		dc.OnOpen(func() {
			log.Printf("OnOpen: %s-%d. Start receiving data", dc.Label(), dc.ID())
			since := time.Now()

			// Start printing out the observed throughput
			for range time.NewTicker(1000 * time.Millisecond).C {
				bps := float64(atomic.LoadUint64(&totalBytesReceived)*8) / time.Since(since).Seconds()
				log.Printf("Throughput: %.03f Mbps", bps/1024/1024)
			}
		})

		// Register the OnMessage to handle incoming messages
		dc.OnMessage(func(dcMsg webrtc.DataChannelMessage) {
			n := len(dcMsg.Data)
			atomic.AddUint64(&totalBytesReceived, uint64(n))
		})
	})

	return pc
}

func main() {
	offerPC := createOfferer()
	answerPC := createAnswerer()

	// Set ICE Candidate handler. As soon as a PeerConnection has gathered a candidate
	// send it to the other peer
	answerPC.OnICECandidate(func(i *webrtc.ICECandidate) {
		if i != nil {
			check(offerPC.AddICECandidate(i.ToJSON()))
		}
	})

	// Set ICE Candidate handler. As soon as a PeerConnection has gathered a candidate
	// send it to the other peer
	offerPC.OnICECandidate(func(i *webrtc.ICECandidate) {
		if i != nil {
			check(answerPC.AddICECandidate(i.ToJSON()))
		}
	})

	// Now, create an offer
	offer, err := offerPC.CreateOffer(nil)
	check(err)
	check(offerPC.SetLocalDescription(offer))
	desc, err := json.Marshal(offer)
	check(err)

	setRemoteDescription(answerPC, desc)

	answer, err := answerPC.CreateAnswer(nil)
	check(err)
	check(answerPC.SetLocalDescription(answer))
	desc2, err := json.Marshal(answer)
	check(err)

	setRemoteDescription(offerPC, desc2)

	// Block forever
	select {}
}
