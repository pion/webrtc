package network

import (
	"github.com/pions/webrtc/pkg/rtp"
)

// BufferTransportGenerator generates a new channel for the associated SSRC
// This channel is used to send RTP packets to users of pion-WebRTC
type BufferTransportGenerator func(uint32) chan<- *rtp.Packet

// ICENotifier notifies the RTCPeerConnection if ICE state has changed for this port
type ICENotifier func(*Port)
