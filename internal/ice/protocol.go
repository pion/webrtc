package ice

import (
	"fmt"
	"strings"
)

// Protocol indicates the transport protocol type that is used in the
// ice.URL structure.
type Protocol int

const (
	// ProtocolUDP indicates the URL uses a UDP transport.
	ProtocolUDP Protocol = iota + 1

	// ProtocolTCP indicates the URL uses a TCP transport.
	ProtocolTCP
)

// This is done this way because of a linter.
const (
	protocolUDPStr = "udp"
	protocolTCPStr = "tcp"
)

// NewProtocol takes a string and converts it to Protocol
func NewProtocol(raw string) (Protocol, error) {
	switch {
	case strings.EqualFold(protocolUDPStr, raw):
		return ProtocolUDP, nil
	case strings.EqualFold(protocolTCPStr, raw):
		return ProtocolTCP, nil
	default:
		return Protocol(Unknown), fmt.Errorf("unknown protocol: %s", raw)
	}
}

func (t Protocol) String() string {
	switch t {
	case ProtocolUDP:
		return protocolUDPStr
	case ProtocolTCP:
		return protocolTCPStr
	default:
		return ErrUnknownType.Error()
	}
}
