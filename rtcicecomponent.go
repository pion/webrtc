package webrtc

// RTCIceComponent describes if the ice transport is used for RTP
// (or RTCP multiplexing).
type RTCIceComponent int

const (
	// RTCIceComponentRtp indicates that the ICE Transport is used for RTP (or
	// RTCP multiplexing), as defined in
	// https://tools.ietf.org/html/rfc5245#section-4.1.1.1. Protocols
	// multiplexed with RTP (e.g. data channel) share its component ID. This
	// represents the component-id value 1 when encoded in candidate-attribute.
	RTCIceComponentRtp RTCIceComponent = iota + 1

	// RTCIceComponentRtcp indicates that the ICE Transport is used for RTCP as
	// defined by https://tools.ietf.org/html/rfc5245#section-4.1.1.1. This
	// represents the component-id value 2 when encoded in candidate-attribute.
	RTCIceComponentRtcp
)

// This is done this way because of a linter.
const (
	rtcIceComponentRtpStr  = "rtp"
	rtcIceComponentRtcpStr = "rtcp"
)

func newRTCIceComponent(raw string) RTCIceComponent {
	switch raw {
	case rtcIceComponentRtpStr:
		return RTCIceComponentRtp
	case rtcIceComponentRtcpStr:
		return RTCIceComponentRtcp
	default:
		return RTCIceComponent(Unknown)
	}
}

func (t RTCIceComponent) String() string {
	switch t {
	case RTCIceComponentRtp:
		return rtcIceComponentRtpStr
	case RTCIceComponentRtcp:
		return rtcIceComponentRtcpStr
	default:
		return ErrUnknownType.Error()
	}
}
