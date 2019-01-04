package webrtc

import "fmt"

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

func newRTCIceProtocol(raw string) (RTCIceProtocol, error) {
	switch raw {
	case rtcIceProtocolUDPStr:
		return RTCIceProtocolUDP, nil
	case rtcIceProtocolTCPStr:
		return RTCIceProtocolTCP, nil
	default:
		return RTCIceProtocol(Unknown), fmt.Errorf("unknown protocol: %s", raw)
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
