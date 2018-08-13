package webrtc

import (
	"sync"

	"github.com/pions/webrtc/pkg/datachannel"
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

// RTCPriorityType determines the priority of a data channel.
type RTCPriorityType int

const (
	// RTCPriorityTypeVeryLow corresponds to "below normal"
	RTCPriorityTypeVeryLow RTCPriorityType = iota + 1

	// RTCPriorityTypeLow corresponds to "normal"
	RTCPriorityTypeLow

	// RTCPriorityTypeMedium corresponds to "high"
	RTCPriorityTypeMedium

	// RTCPriorityTypeHigh corresponds to "extra high"
	RTCPriorityTypeHigh
)

func (p RTCPriorityType) String() string {
	switch p {
	case RTCPriorityTypeVeryLow:
		return "very-low"
	case RTCPriorityTypeLow:
		return "low"
	case RTCPriorityTypeMedium:
		return "medium"
	case RTCPriorityTypeHigh:
		return "high"
	default:
		return "Unknown"
	}
}

// RTCDataChannelInit can be used to configure properties of the underlying channel such as data reliability.
type RTCDataChannelInit struct {
	Ordered           bool
	MaxPacketLifeTime *uint16
	MaxRetransmits    *uint16
	Protocol          string
	Negotiated        bool
	ID                uint16
	Priority          RTCPriorityType
}

// CreateDataChannel creates a new RTCDataChannel object with the given label and optitional options.
func (r *RTCPeerConnection) CreateDataChannel(label string, options *RTCDataChannelInit) (*RTCDataChannel, error) {
	if r.IsClosed {
		return nil, &InvalidStateError{Err: ErrConnectionClosed}
	}

	if len(label) > 65535 {
		return nil, &TypeError{Err: ErrInvalidValue}
	}

	// Defaults
	ordered := true
	priority := RTCPriorityTypeLow
	negotiated := false

	if options != nil {
		ordered = options.Ordered
		priority = options.Priority
		negotiated = options.Negotiated
	}

	var id uint16
	if negotiated {
		id = options.ID
	} else {
		var err error
		id, err = r.generateDataChannelID(true) // TODO: base on DTLS role
		if err != nil {
			return nil, err
		}
	}

	if id > 65534 {
		return nil, &TypeError{Err: ErrInvalidValue}
	}

	if r.sctp.State == RTCSctpTransportStateConnected &&
		id >= r.sctp.MaxChannels {
		return nil, &OperationError{Err: ErrMaxDataChannels}
	}

	_ = ordered  // TODO
	_ = priority // TODO
	res := &RTCDataChannel{
		Label:             label,
		ID:                id,
		rtcPeerConnection: r,
	}

	// Remember datachannel
	r.dataChannels[id] = res

	// Send opening message
	// r.networkManager.SendOpenChannelMessage(id, label)

	return res, nil
}

func (r *RTCPeerConnection) generateDataChannelID(client bool) (uint16, error) {
	var id uint16
	if !client {
		id++
	}

	for ; id < r.sctp.MaxChannels-1; id += 2 {
		_, ok := r.dataChannels[id]
		if !ok {
			return id, nil
		}
	}
	return 0, &OperationError{Err: ErrMaxDataChannels}
}

// SendOpenChannelMessage is a test to send OpenChannel manually
func (d *RTCDataChannel) SendOpenChannelMessage() error {
	if err := d.rtcPeerConnection.networkManager.SendOpenChannelMessage(d.ID, d.Label); err != nil {
		return &UnknownError{Err: err}
	}
	return nil

}

// Send sends the passed message to the DataChannel peer
func (d *RTCDataChannel) Send(p datachannel.Payload) error {
	if err := d.rtcPeerConnection.networkManager.SendDataChannelMessage(p, d.ID); err != nil {
		return &UnknownError{Err: err}
	}
	return nil
}
