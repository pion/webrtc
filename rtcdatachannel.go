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

	// Transport represents the associated underlying data transport that is
	// used to transport actual data to the other peer.
	Transport *RTCSctpTransport

	// Label represents a label that can be used to distinguish this
	// RTCDataChannel object from other RTCDataChannel objects. Scripts are
	// allowed to create multiple RTCDataChannel objects with the same label.
	Label string

	// Ordered represents if the RTCDataChannel is ordered, and false if
	// out-of-order delivery is allowed.
	Ordered bool

	// MaxPacketLifeTime represents the length of the time window (msec) during
	// which transmissions and retransmissions may occur in unreliable mode.
	MaxPacketLifeTime *uint16

	// MaxRetransmits represents the maximum number of retransmissions that are
	// attempted in unreliable mode.
	MaxRetransmits *uint16

	// Protocol represents the name of the sub-protocol used with this
	// RTCDataChannel.
	Protocol string

	// Negotiated represents whether this RTCDataChannel was negotiated by the
	// application (true), or not (false).
	Negotiated bool

	// ID represents the ID for this RTCDataChannel. The value is initially
	// null, which is what will be returned if the ID was not provided at
	// channel creation time, and the DTLS role of the SCTP transport has not
	// yet been negotiated. Otherwise, it will return the ID that was either
	// selected by the script or generated. After the ID is set to a non-null
	// value, it will not change.
	ID *uint16

	// Priority represents the priority for this RTCDataChannel. The priority is
	// assigned at channel creation time.
	Priority RTCPriorityType

	// ReadyState represents the state of the RTCDataChannel object.
	ReadyState RTCDataChannelState

	// BufferedAmount represents the number of bytes of application data
	// (UTF-8 text and binary data) that have been queued using send(). Even
	// though the data transmission can occur in parallel, the returned value
	// MUST NOT be decreased before the current task yielded back to the event
	// loop to prevent race conditions. The value does not include framing
	// overhead incurred by the protocol, or buffering done by the operating
	// system or network hardware. The value of BufferedAmount slot will only
	// increase with each call to the send() method as long as the ReadyState is
	// open; however, BufferedAmount does not reset to zero once the channel
	// closes.
	BufferedAmount uint64

	// BufferedAmountLowThreshold represents the threshold at which the
	// bufferedAmount is considered to be low. When the bufferedAmount decreases
	// from above this threshold to equal or below it, the bufferedamountlow
	// event fires. BufferedAmountLowThreshold is initially zero on each new
	// RTCDataChannel, but the application may change its value at any time.
	BufferedAmountLowThreshold uint64

	// The binaryType represents attribute MUST, on getting, return the value to
	// which it was last set. On setting, if the new value is either the string
	// "blob" or the string "arraybuffer", then set the IDL attribute to this
	// new value. Otherwise, throw a SyntaxError. When an RTCDataChannel object
	// is created, the binaryType attribute MUST be initialized to the string
	// "blob". This attribute controls how binary data is exposed to scripts.
	// binaryType                 string

	// OnOpen              func()
	// OnBufferedAmountLow func()
	// OnError             func()
	// OnClose             func()

	// Onmessage designates an event handler which is invoked on a message
	// arrival over the sctp transport from a remote peer.
	//
	// Deprecated: use OnMessage instead.
	Onmessage func(datachannel.Payload)

	// OnMessage designates an event handler which is invoked on a message
	// arrival over the sctp transport from a remote peer.
	OnMessage func(datachannel.Payload)

	// Deprecated: Will be removed when networkManager is deprecated.
	rtcPeerConnection *RTCPeerConnection
}

// func (d *RTCDataChannel) generateID() error {
// 	// TODO: base on DTLS role, currently static at "true".
// 	client := true
//
// 	var id uint16
// 	if !client {
// 		id++
// 	}
//
// 	for ; id < *d.Transport.MaxChannels-1; id += 2 {
// 		_, ok := d.rtcPeerConnection.dataChannels[id]
// 		if !ok {
// 			d.ID = &id
// 			return nil
// 		}
// 	}
// 	return &rtcerr.OperationError{Err: ErrMaxDataChannelID}
// }

// SendOpenChannelMessage is a test to send OpenChannel manually
func (d *RTCDataChannel) SendOpenChannelMessage() error {
	if err := d.rtcPeerConnection.networkManager.SendOpenChannelMessage(*d.ID, d.Label); err != nil {
		return &rtcerr.UnknownError{Err: err}
	}
	return nil

}

// Send sends the passed message to the DataChannel peer
func (d *RTCDataChannel) Send(p datachannel.Payload) error {
	if err := d.rtcPeerConnection.networkManager.SendDataChannelMessage(p, *d.ID); err != nil {
		return &rtcerr.UnknownError{Err: err}
	}
	return nil
}
