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

type RTCDataChannelInit struct {
	Ordered           bool
	MaxPacketLifeTime *uint16
	MaxRetransmits    *uint16
	Protocol          string
	Negotiated        bool
	Id                uint16
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
		ordered := options.Ordered
		priority := options.Priority
		negotiated := options.Negotiated
	}

	id := 0
	if negotiated {
		id := options.Id
	} else {
		// TODO: generate id
	}

	if id > 65534 {
		return nil, &TypeError{Err: ErrInvalidValue}
	}

	if r.sctp.State == RTCSctpTransportStateConnected &&
		id >= r.sctp.MaxChannels {
		return nil, &OperationError{Err: ErrMaxDataChannels}
	}

	// TODO: Actually allocate datachannel
	res := &RTCDataChannel{
		Label:             label,
		ID:                id,
		rtcPeerConnection: r,
	}

	return res, nil
}

// Send sends the passed message to the DataChannel peer
func (r *RTCDataChannel) Send(p datachannel.Payload) error {
	if err := r.rtcPeerConnection.networkManager.SendDataChannelMessage(p, r.ID); err != nil {
		return &UnknownError{Err: err}
	}
	return nil
}
