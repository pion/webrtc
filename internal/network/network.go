package network

import (
	"github.com/pions/webrtc/pkg/rtp"
)

// BufferTransportGenerator generates a new channel for the associated SSRC
// This channel is used to send RTP packets to users of pion-WebRTC
type BufferTransportGenerator func(uint32, uint8) chan<- *rtp.Packet

// ICENotifier notifies the RTCPeerConnection if ICE state has changed for this port
type ICENotifier func(*Port)

// DataChannelEventHandler notifies the RTCPeerConnection of events relating to DataChannels
type DataChannelEventHandler func(DataChannelEvent)

// DataChannelEventType is the enum used to represent different types of DataChannelEvent
type DataChannelEventType int

// Enums for DataChannelEventType
const (
	NewDataChannel int = iota + 1
	NewMessage
)

// DataChannelEvent is the interface for all events that flow across the DataChannelEventHandler
type DataChannelEvent interface {
	StreamIdentifier() uint16
}

// DataChannelCreated is emitted when a new DataChannel is created
type DataChannelCreated struct {
	Label            string
	streamIdentifier uint16
}

// StreamIdentifier returns the streamIdentifier
func (d *DataChannelCreated) StreamIdentifier() uint16 {
	return d.streamIdentifier
}

// DataChannelMessage is emitted when a DataChannel recieves a message
type DataChannelMessage struct {
	Body             []byte
	streamIdentifier uint16
}

// StreamIdentifier returns the streamIdentifier
func (d *DataChannelMessage) StreamIdentifier() uint16 {
	return d.streamIdentifier
}
