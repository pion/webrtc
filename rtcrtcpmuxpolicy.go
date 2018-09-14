package webrtc

// RTCRtcpMuxPolicy affects what ICE candidates are gathered to support
// non-multiplexed RTCP.
type RTCRtcpMuxPolicy int

const (
	// RTCRtcpMuxPolicyNegotiate indicates to gather ICE candidates for both
	// RTP and RTCP candidates. If the remote-endpoint is capable of
	// multiplexing RTCP, multiplex RTCP on the RTP candidates. If it is not,
	// use both the RTP and RTCP candidates separately.
	RTCRtcpMuxPolicyNegotiate RTCRtcpMuxPolicy = iota + 1

	// RTCRtcpMuxPolicyRequire indicates to gather ICE candidates only for
	// RTP and multiplex RTCP on the RTP candidates. If the remote endpoint is
	// not capable of rtcp-mux, session negotiation will fail.
	RTCRtcpMuxPolicyRequire
)

// This is done this way because of a linter.
const (
	rtcRtcpMuxPolicyNegotiateStr = "negotiate"
	rtcRtcpMuxPolicyRequireStr   = "require"
)

func newRTCRtcpMuxPolicy(raw string) RTCRtcpMuxPolicy {
	switch raw {
	case rtcRtcpMuxPolicyNegotiateStr:
		return RTCRtcpMuxPolicyNegotiate
	case rtcRtcpMuxPolicyRequireStr:
		return RTCRtcpMuxPolicyRequire
	default:
		return RTCRtcpMuxPolicy(Unknown)
	}
}

func (t RTCRtcpMuxPolicy) String() string {
	switch t {
	case RTCRtcpMuxPolicyNegotiate:
		return rtcRtcpMuxPolicyNegotiateStr
	case RTCRtcpMuxPolicyRequire:
		return rtcRtcpMuxPolicyRequireStr
	default:
		return ErrUnknownType.Error()
	}
}
