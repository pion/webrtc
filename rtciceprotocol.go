package webrtc

// RTCIceProtocol indicates the transport protocol type that is used in the
// ice.URL structure.
type RTCIceProtocol int

const (
	// RTCIceProtocolUDP indicates the URL uses a UDP transport.
	RTCIceProtocolUDP RTCIceProtocol = iota + 1

	// RTCIceProtocolTCP indicates the URL uses a TCP transport.
	RTCIceProtocolTCP
)

// This is done this way because of a linter.
const (
	rtcIceProtocolUDPStr = "udp"
	rtcIceProtocolTCPStr = "tcp"
)

func newRTCIceProtocol(raw string) RTCIceProtocol {
	switch raw {
	case rtcIceProtocolUDPStr:
		return RTCIceProtocolUDP
	case rtcIceProtocolTCPStr:
		return RTCIceProtocolTCP
	default:
		return RTCIceProtocol(Unknown)
	}
}

func (t RTCIceProtocol) String() string {
	switch t {
	case RTCIceProtocolUDP:
		return rtcIceProtocolUDPStr
	case RTCIceProtocolTCP:
		return rtcIceProtocolTCPStr
	default:
		return ErrUnknownType.Error()
	}
}
