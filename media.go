package webrtc

import (
	"github.com/pions/webrtc/pkg/rtp"
)

// RTCSample contains media, and the amount of samples in it
type RTCSample struct {
	Data    []byte
	Samples uint32
}

// RTCTrack represents a track that is communicated
type RTCTrack struct {
	ID          string
	PayloadType uint8
	Kind        RTCRtpCodecType
	Label       string
	Ssrc        uint32
	Codec       *RTCRtpCodec
	Packets     <-chan *rtp.Packet
	Samples     chan<- RTCSample
}
