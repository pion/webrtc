package main

import (
	"net/http"
	"sync"

	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pions/rtcp"
	"github.com/pions/webrtc"
)

// Peer config
var peerConnectionConfig = webrtc.Configuration{
	ICEServers: []webrtc.ICEServer{
		{
			URLs: []string{"stun:stun.l.google.com:19302"},
		},
	},
}

var (
	// Media engine
	m webrtc.MediaEngine

	// API object
	api *webrtc.API

	// Publisher Peer
	pubCount    int32
	pubReceiver *webrtc.PeerConnection

	// Local track
	videoTrack     *webrtc.Track
	audioTrack     *webrtc.Track
	videoTrackLock = sync.RWMutex{}
	audioTrackLock = sync.RWMutex{}

	// Websocket upgrader
	upgrader = websocket.Upgrader{}

	// Broadcast channels
	broadcastHub = newHub()
)

const (
	rtcpPLIInterval = time.Second * 3
)

func room(w http.ResponseWriter, r *http.Request) {

	// Websocket client
	c, err := upgrader.Upgrade(w, r, nil)
	checkError(err)
	defer func() {
		checkError(c.Close())
	}()

	// Read sdp from websocket
	mt, msg, err := c.ReadMessage()
	checkError(err)

	if atomic.LoadInt32(&pubCount) == 0 {
		atomic.AddInt32(&pubCount, 1)

		// Create a new RTCPeerConnection
		pubReceiver, err = api.NewPeerConnection(peerConnectionConfig)
		checkError(err)

		pubReceiver.OnTrack(func(remoteTrack *webrtc.Track, receiver *webrtc.RTPReceiver) {
			if remoteTrack.PayloadType() == webrtc.DefaultPayloadTypeVP8 || remoteTrack.PayloadType() == webrtc.DefaultPayloadTypeVP9 || remoteTrack.PayloadType() == webrtc.DefaultPayloadTypeH264 {

				// Create a local video track, all our SFU clients will be fed via this track
				var err error
				videoTrackLock.Lock()
				videoTrack, err = pubReceiver.NewTrack(remoteTrack.PayloadType(), remoteTrack.SSRC(), "video", "pion")
				videoTrackLock.Unlock()
				checkError(err)

				// Send a PLI on an interval so that the publisher is pushing a keyframe every rtcpPLIInterval
				go func() {
					ticker := time.NewTicker(rtcpPLIInterval)
					for range ticker.C {
						checkError(pubReceiver.SendRTCP(&rtcp.PictureLossIndication{MediaSSRC: videoTrack.SSRC()}))
					}
				}()

				rtpBuf := make([]byte, 1400)
				for {
					i, err := remoteTrack.Read(rtpBuf)
					checkError(err)
					videoTrackLock.RLock()
					_, err = videoTrack.Write(rtpBuf[:i])
					videoTrackLock.RUnlock()
					checkError(err)
				}

			} else {

				// Create a local audio track, all our SFU clients will be fed via this track
				var err error
				audioTrackLock.Lock()
				audioTrack, err = pubReceiver.NewTrack(remoteTrack.PayloadType(), remoteTrack.SSRC(), "audio", "pion")
				audioTrackLock.Unlock()
				checkError(err)

				rtpBuf := make([]byte, 1400)
				for {
					i, err := remoteTrack.Read(rtpBuf)
					checkError(err)
					audioTrackLock.RLock()
					_, err = audioTrack.Write(rtpBuf[:i])
					audioTrackLock.RUnlock()
					checkError(err)
				}
			}
		})

		// Set the remote SessionDescription
		checkError(pubReceiver.SetRemoteDescription(
			webrtc.SessionDescription{
				SDP:  string(msg),
				Type: webrtc.SDPTypeOffer,
			}))

		// Create answer
		answer, err := pubReceiver.CreateAnswer(nil)
		checkError(err)

		// Sets the LocalDescription, and starts our UDP listeners
		checkError(pubReceiver.SetLocalDescription(answer))

		// Send server sdp to publisher
		checkError(c.WriteMessage(mt, []byte(answer.SDP)))

		// Register incoming channel
		pubReceiver.OnDataChannel(func(d *webrtc.DataChannel) {
			d.OnMessage(func(msg webrtc.DataChannelMessage) {
				// Broadcast the data to subSenders
				broadcastHub.broadcastChannel <- msg.Data
			})
		})
	} else {

		// Create a new PeerConnection
		subSender, err := api.NewPeerConnection(peerConnectionConfig)
		checkError(err)

		// Register data channel creation handling
		subSender.OnDataChannel(func(d *webrtc.DataChannel) {
			broadcastHub.addListener(d)
		})

		// Waiting for publisher track finish
		for {
			videoTrackLock.RLock()
			if videoTrack == nil {
				videoTrackLock.RUnlock()
				//if videoTrack == nil, waiting..
				time.Sleep(100 * time.Millisecond)
			} else {
				videoTrackLock.RUnlock()
				break
			}
		}

		// Add local video track
		videoTrackLock.RLock()
		_, err = subSender.AddTrack(videoTrack)
		videoTrackLock.RUnlock()
		checkError(err)

		// Add local audio track
		audioTrackLock.RLock()
		_, err = subSender.AddTrack(audioTrack)
		audioTrackLock.RUnlock()
		checkError(err)

		// Set the remote SessionDescription
		checkError(subSender.SetRemoteDescription(
			webrtc.SessionDescription{
				SDP:  string(msg),
				Type: webrtc.SDPTypeOffer,
			}))

		// Create answer
		answer, err := subSender.CreateAnswer(nil)
		checkError(err)

		// Sets the LocalDescription, and starts our UDP listeners
		checkError(subSender.SetLocalDescription(answer))

		// Send server sdp to subscriber
		checkError(c.WriteMessage(mt, []byte(answer.SDP)))
	}
}
