package main

import (
	"log"
	"net"
	"sync/atomic"
	"time"

	"github.com/pion/logging"
	"github.com/pion/transport/vnet"
	"github.com/pion/webrtc/v3"
)

/* VNet Configuration
+ - - - - - - - - - - - - - - - - - - - - - - - +
                      VNet
| +-------------------------------------------+ |
  |              wan:vnet.Router              |
| +---------+----------------------+----------+ |
            |                      |
| +---------+----------+ +---------+----------+ |
  | offerVNet:vnet.Net | |answerVNet:vnet.Net |
| +---------+----------+ +---------+----------+ |
            |                      |
+ - - - - - + - - - - - - - - - - -+- - - - - - +
            |                      |
  +---------+----------+ +---------+----------+
  |offerPeerConnection | |answerPeerConnection|
  +--------------------+ +--------------------+
*/

func main() {
	var inboundBytes int32  // for offerPeerConnection
	var outboundBytes int32 // for offerPeerConnection

	// Create a root router
	wan, err := vnet.NewRouter(&vnet.RouterConfig{
		CIDR:          "1.2.3.0/24",
		LoggerFactory: logging.NewDefaultLoggerFactory(),
	})
	panicIfError(err)

	// Add a filter that monitors the traffic on the router
	wan.AddChunkFilter(func(c vnet.Chunk) bool {
		netType := c.SourceAddr().Network()
		if netType == "udp" {
			dstAddr := c.DestinationAddr().String()
			host, _, err2 := net.SplitHostPort(dstAddr)
			panicIfError(err2)
			if host == "1.2.3.4" {
				// c.UserData() returns a []byte of UDP payload
				atomic.AddInt32(&inboundBytes, int32(len(c.UserData())))
			}
			srcAddr := c.SourceAddr().String()
			host, _, err2 = net.SplitHostPort(srcAddr)
			panicIfError(err2)
			if host == "1.2.3.4" {
				// c.UserData() returns a []byte of UDP payload
				atomic.AddInt32(&outboundBytes, int32(len(c.UserData())))
			}
		}
		return true
	})

	// Log throughput every 3 seconds
	go func() {
		duration := 2 * time.Second
		for {
			time.Sleep(duration)

			inBytes := atomic.SwapInt32(&inboundBytes, 0)   // read & reset
			outBytes := atomic.SwapInt32(&outboundBytes, 0) // read & reset
			inboundThroughput := float64(inBytes) / duration.Seconds()
			outboundThroughput := float64(outBytes) / duration.Seconds()
			log.Printf("inbound throughput : %.01f [Byte/s]\n", inboundThroughput)
			log.Printf("outbound throughput: %.01f [Byte/s]\n", outboundThroughput)
		}
	}()

	// Create a network interface for offerer
	offerVNet := vnet.NewNet(&vnet.NetConfig{
		StaticIPs: []string{"1.2.3.4"},
	})
	// Add the network interface to the router
	panicIfError(wan.AddNet(offerVNet))

	offerSettingEngine := webrtc.SettingEngine{}
	offerSettingEngine.SetVNet(offerVNet)
	offerAPI := webrtc.NewAPI(webrtc.WithSettingEngine(offerSettingEngine))

	// Create a network interface for answerer
	answerVNet := vnet.NewNet(&vnet.NetConfig{
		StaticIPs: []string{"1.2.3.5"},
	})
	// Add the network interface to the router
	panicIfError(wan.AddNet(answerVNet))

	answerSettingEngine := webrtc.SettingEngine{}
	answerSettingEngine.SetVNet(answerVNet)
	answerAPI := webrtc.NewAPI(webrtc.WithSettingEngine(answerSettingEngine))

	// Start the virtual network by calling Start() on the root router
	panicIfError(wan.Start())

	offerPeerConnection, err := offerAPI.NewPeerConnection(webrtc.Configuration{})
	panicIfError(err)

	answerPeerConnection, err := answerAPI.NewPeerConnection(webrtc.Configuration{})
	panicIfError(err)

	// Set ICE Candidate handler. As soon as a PeerConnection has gathered a candidate
	// send it to the other peer
	answerPeerConnection.OnICECandidate(func(i *webrtc.ICECandidate) {
		if i != nil {
			panicIfError(offerPeerConnection.AddICECandidate(i.ToJSON()))
		}
	})

	// Set ICE Candidate handler. As soon as a PeerConnection has gathered a candidate
	// send it to the other peer
	offerPeerConnection.OnICECandidate(func(i *webrtc.ICECandidate) {
		if i != nil {
			panicIfError(answerPeerConnection.AddICECandidate(i.ToJSON()))
		}
	})

	offerDataChannel, err := offerPeerConnection.CreateDataChannel("label", nil)
	panicIfError(err)

	msgSendLoop := func(dc *webrtc.DataChannel, interval time.Duration) {
		for {
			time.Sleep(interval)
			panicIfError(dc.SendText("My DataChannel Message"))
		}
	}

	offerDataChannel.OnOpen(func() {
		// Send test from offerer every 100 msec
		msgSendLoop(offerDataChannel, 100*time.Millisecond)
	})

	answerPeerConnection.OnDataChannel(func(answerDataChannel *webrtc.DataChannel) {
		answerDataChannel.OnOpen(func() {
			// Send test from answerer every 200 msec
			msgSendLoop(answerDataChannel, 200*time.Millisecond)
		})
	})

	offer, err := offerPeerConnection.CreateOffer(nil)
	panicIfError(err)
	panicIfError(offerPeerConnection.SetLocalDescription(offer))
	panicIfError(answerPeerConnection.SetRemoteDescription(offer))

	answer, err := answerPeerConnection.CreateAnswer(nil)
	panicIfError(err)
	panicIfError(answerPeerConnection.SetLocalDescription(answer))
	panicIfError(offerPeerConnection.SetRemoteDescription(answer))

	select {}
}

func panicIfError(err error) {
	if err != nil {
		panic(err)
	}
}
