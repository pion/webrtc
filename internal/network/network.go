package network

import "github.com/pions/webrtc/pkg/rtp"

// BufferTransportGenerator generates a new channel for the associated SSRC
// This channel is used to send RTP packets to users of pion-WebRTC
type BufferTransportGenerator func(uint32) chan<- *rtp.Packet
