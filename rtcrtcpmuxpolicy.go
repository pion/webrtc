package webrtc

// RTCRtcpMuxPolicy affects what ICE candidates are gathered to support non-multiplexed RTCP
type RTCRtcpMuxPolicy int

const (
	// RTCRtcpMuxPolicyNegotiate indicates to gather ICE candidates for both RTP and RTCP candidates.
	RTCRtcpMuxPolicyNegotiate RTCRtcpMuxPolicy = iota + 1

	// RTCRtcpMuxPolicyRequire indicates to gather ICE candidates only for RTP and multiplex RTCP on the RTP candidates
	RTCRtcpMuxPolicyRequire
)

func NewRTCRtcpMuxPolicy(raw string) RTCRtcpMuxPolicy {
	switch raw {
	case "negotiate":
		return RTCRtcpMuxPolicyNegotiate
	case "require":
		return RTCRtcpMuxPolicyRequire
	default:
		return RTCRtcpMuxPolicy(Unknown)
	}
}

func (t RTCRtcpMuxPolicy) String() string {
	switch t {
	case RTCRtcpMuxPolicyNegotiate:
		return "negotiate"
	case RTCRtcpMuxPolicyRequire:
		return "require"
	default:
		return ErrUnknownType.Error()
	}
}
