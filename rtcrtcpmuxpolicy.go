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

// NewRTCRtcpMuxPolicy defines a procedure for creating a new RTCRtcpMuxPolicy
// from a raw string naming the rtcp multiplexing policy.
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
