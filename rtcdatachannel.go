package webrtc

import (
	"sync"

	"github.com/pions/webrtc/pkg/datachannel"
	"github.com/pions/webrtc/pkg/rtcerr"
)

// RTCDataChannel represents a WebRTC DataChannel
// The RTCDataChannel interface represents a network channel
// which can be used for bidirectional peer-to-peer transfers of arbitrary data
type RTCDataChannel struct {
	sync.RWMutex

	Onmessage func(datachannel.Payload)
	ID        uint16
	Label     string

	rtcPeerConnection *RTCPeerConnection
}

// SendOpenChannelMessage is a test to send OpenChannel manually
func (d *RTCDataChannel) SendOpenChannelMessage() error {
	if err := d.rtcPeerConnection.networkManager.SendOpenChannelMessage(d.ID, d.Label); err != nil {
		return &rtcerr.UnknownError{Err: err}
	}
	return nil

}

// Send sends the passed message to the DataChannel peer
func (d *RTCDataChannel) Send(p datachannel.Payload) error {
	if err := d.rtcPeerConnection.networkManager.SendDataChannelMessage(p, d.ID); err != nil {
		return &rtcerr.UnknownError{Err: err}
	}
	return nil
}
