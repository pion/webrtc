package webrtc

import (
	"github.com/pions/webrtc/pkg/media"
	"github.com/pions/webrtc/pkg/rtp"
)

// RTCSample contains media, and the amount of samples in it
//
// Deprecated: use RTCSample from github.com/pions/webrtc/pkg/media instead
type RTCSample = media.RTCSample

// RTCTrack represents a track that is communicated
type RTCTrack struct {
	ID          string
	PayloadType uint8
	Kind        RTCRtpCodecType
	Label       string
	Ssrc        uint32
	Codec       *RTCRtpCodec
	Packets     <-chan *rtp.Packet
	Samples     chan<- media.RTCSample
}
