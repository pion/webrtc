package webrtc

import "github.com/pions/webrtc/pkg/datachannel"

// RTCDataChannel represents a WebRTC DataChannel
// The RTCDataChannel interface represents a network channel
// which can be used for bidirectional peer-to-peer transfers of arbitrary data
type RTCDataChannel struct {
	Onmessage func(datachannel.Payload)
	ID        uint16
	Label     string

	rtcPeerConnection *RTCPeerConnection
}

// Send sends the passed message to the DataChannel peer
func (r *RTCDataChannel) Send(p datachannel.Payload) error {
	if err := r.rtcPeerConnection.networkManager.SendDataChannelMessage(p, r.ID); err != nil {
		return &UnknownError{Err: err}
	}
	return nil
}
