package webrtc

import (
	"fmt"
	"strings"
)

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
	switch {
	case strings.EqualFold(rtcIceProtocolUDPStr, raw):
		return RTCIceProtocolUDP, nil
	case strings.EqualFold(rtcIceProtocolTCPStr, raw):
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
