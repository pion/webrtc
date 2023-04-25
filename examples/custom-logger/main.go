// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js
// +build !js

// custom-logger is an example of how the Pion API provides an customizable logging API
package main

import (
	"fmt"
	"os"

	"github.com/pion/logging"
	"github.com/pion/webrtc/v3"
)

// Everything below is the Pion WebRTC API! Thanks for using it ❤️.

// customLogger satisfies the interface logging.LeveledLogger
// a logger is created per subsystem in Pion, so you can have custom
// behavior per subsystem (ICE, DTLS, SCTP...)
type customLogger struct{}

// Print all messages except trace
func (c customLogger) Trace(string)                  {}
func (c customLogger) Tracef(string, ...interface{}) {}

func (c customLogger) Debug(msg string) { fmt.Printf("customLogger Debug: %s\n", msg) }
func (c customLogger) Debugf(format string, args ...interface{}) {
	c.Debug(fmt.Sprintf(format, args...))
}
func (c customLogger) Info(msg string) { fmt.Printf("customLogger Info: %s\n", msg) }
func (c customLogger) Infof(format string, args ...interface{}) {
	c.Trace(fmt.Sprintf(format, args...))
}
func (c customLogger) Warn(msg string) { fmt.Printf("customLogger Warn: %s\n", msg) }
func (c customLogger) Warnf(format string, args ...interface{}) {
	c.Warn(fmt.Sprintf(format, args...))
}
func (c customLogger) Error(msg string) { fmt.Printf("customLogger Error: %s\n", msg) }
func (c customLogger) Errorf(format string, args ...interface{}) {
	c.Error(fmt.Sprintf(format, args...))
}

// customLoggerFactory satisfies the interface logging.LoggerFactory
// This allows us to create different loggers per subsystem. So we can
// add custom behavior
type customLoggerFactory struct{}

func (c customLoggerFactory) NewLogger(subsystem string) logging.LeveledLogger {
	fmt.Printf("Creating logger for %s \n", subsystem)
	return customLogger{}
}

func main() {
	// Create a new API with a custom logger
	// This SettingEngine allows non-standard WebRTC behavior
	s := webrtc.SettingEngine{
		LoggerFactory: customLoggerFactory{},
	}
	api := webrtc.NewAPI(webrtc.WithSettingEngine(s))

	// Create a new RTCPeerConnection
	offerPeerConnection, err := api.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		panic(err)
	}
	defer func() {
		if cErr := offerPeerConnection.Close(); cErr != nil {
			fmt.Printf("cannot close offerPeerConnection: %v\n", cErr)
		}
	}()

	// We need a DataChannel so we can have ICE Candidates
	if _, err = offerPeerConnection.CreateDataChannel("custom-logger", nil); err != nil {
		panic(err)
	}

	// Create a new RTCPeerConnection
	answerPeerConnection, err := api.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		panic(err)
	}
	defer func() {
		if cErr := answerPeerConnection.Close(); cErr != nil {
			fmt.Printf("cannot close answerPeerConnection: %v\n", cErr)
		}
	}()

	// Set the handler for Peer connection state
	// This will notify you when the peer has connected/disconnected
	offerPeerConnection.OnConnectionStateChange(func(s webrtc.PeerConnectionState) {
		fmt.Printf("Peer Connection State has changed: %s (offerer)\n", s.String())

		if s == webrtc.PeerConnectionStateFailed {
			// Wait until PeerConnection has had no network activity for 30 seconds or another failure. It may be reconnected using an ICE Restart.
			// Use webrtc.PeerConnectionStateDisconnected if you are interested in detecting faster timeout.
			// Note that the PeerConnection may come back from PeerConnectionStateDisconnected.
			fmt.Println("Peer Connection has gone to failed exiting")
			os.Exit(0)
		}
	})

	// Set the handler for Peer connection state
	// This will notify you when the peer has connected/disconnected
	answerPeerConnection.OnConnectionStateChange(func(s webrtc.PeerConnectionState) {
		fmt.Printf("Peer Connection State has changed: %s (answerer)\n", s.String())

		if s == webrtc.PeerConnectionStateFailed {
			// Wait until PeerConnection has had no network activity for 30 seconds or another failure. It may be reconnected using an ICE Restart.
			// Use webrtc.PeerConnectionStateDisconnected if you are interested in detecting faster timeout.
			// Note that the PeerConnection may come back from PeerConnectionStateDisconnected.
			fmt.Println("Peer Connection has gone to failed exiting")
			os.Exit(0)
		}
	})

	// Set ICE Candidate handler. As soon as a PeerConnection has gathered a candidate
	// send it to the other peer
	answerPeerConnection.OnICECandidate(func(i *webrtc.ICECandidate) {
		if i != nil {
			if iceErr := offerPeerConnection.AddICECandidate(i.ToJSON()); iceErr != nil {
				panic(iceErr)
			}
		}
	})

	// Set ICE Candidate handler. As soon as a PeerConnection has gathered a candidate
	// send it to the other peer
	offerPeerConnection.OnICECandidate(func(i *webrtc.ICECandidate) {
		if i != nil {
			if iceErr := answerPeerConnection.AddICECandidate(i.ToJSON()); iceErr != nil {
				panic(iceErr)
			}
		}
	})

	// Create an offer for the other PeerConnection
	offer, err := offerPeerConnection.CreateOffer(nil)
	if err != nil {
		panic(err)
	}

	// SetLocalDescription, needed before remote gets offer
	if err = offerPeerConnection.SetLocalDescription(offer); err != nil {
		panic(err)
	}

	// Take offer from remote, answerPeerConnection is now able to contact
	// the other PeerConnection
	if err = answerPeerConnection.SetRemoteDescription(offer); err != nil {
		panic(err)
	}

	// Create an Answer to send back to our originating PeerConnection
	answer, err := answerPeerConnection.CreateAnswer(nil)
	if err != nil {
		panic(err)
	}

	// Set the answerer's LocalDescription
	if err = answerPeerConnection.SetLocalDescription(answer); err != nil {
		panic(err)
	}

	// SetRemoteDescription on original PeerConnection, this finishes our signaling
	// bother PeerConnections should be able to communicate with each other now
	if err = offerPeerConnection.SetRemoteDescription(answer); err != nil {
		panic(err)
	}

	// Block forever
	select {}
}
