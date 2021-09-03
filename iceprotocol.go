package webrtc

import (
	"fmt"
	"strings"
)

// ICEProtocol indicates the transport protocol type that is used in the
// ice.URL structure.
type ICEProtocol int

const (
	// ICEProtocolUDP indicates the URL uses a UDP transport.
	ICEProtocolUDP ICEProtocol = iota + 1

	// ICEProtocolTCP indicates the URL uses a TCP transport.
	ICEProtocolTCP

	// ICEProtocolSSLTCP indicates the URL uses a TCP+SSL transport.
	ICEProtocolSSLTCP
)

// This is done this way because of a linter.
const (
	iceProtocolUDPStr = "udp"
	iceProtocolTCPStr = "tcp"
	iceProtocolSSLTCPStr = "ssltcp"
)

// NewICEProtocol takes a string and converts it to ICEProtocol
func NewICEProtocol(raw string) (ICEProtocol, error) {
	switch {
	case strings.EqualFold(iceProtocolUDPStr, raw):
		return ICEProtocolUDP, nil
	case strings.EqualFold(iceProtocolTCPStr, raw):
		return ICEProtocolTCP, nil
	case strings.EqualFold(iceProtocolSSLTCPStr, raw):
		return ICEProtocolSSLTCP, nil
	default:
		return ICEProtocol(Unknown), fmt.Errorf("%w: %s", errICEProtocolUnknown, raw)
	}
}

func (t ICEProtocol) String() string {
	switch t {
	case ICEProtocolUDP:
		return iceProtocolUDPStr
	case ICEProtocolTCP:
		return iceProtocolTCPStr
	case ICEProtocolSSLTCP:
		return iceProtocolSSLTCPStr
	default:
		return ErrUnknownType.Error()
	}
}
