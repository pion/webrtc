package webrtc

import (
	"fmt"
)

var supportedNetworkTypes = []NetworkType{
	NetworkTypeUDP4,
	NetworkTypeUDP6,
	// NetworkTypeTCP4, // Not supported yet
	// NetworkTypeTCP6, // Not supported yet
}

// NetworkType represents the type of network
type NetworkType int

const (
	// NetworkTypeUDP4 indicates UDP over IPv4.
	NetworkTypeUDP4 NetworkType = iota + 1

	// NetworkTypeUDP6 indicates UDP over IPv6.
	NetworkTypeUDP6

	// NetworkTypeTCP4 indicates TCP over IPv4.
	NetworkTypeTCP4

	// NetworkTypeTCP6 indicates TCP over IPv6.
	NetworkTypeTCP6
)

// This is done this way because of a linter.
const (
	networkTypeUDP4Str = "udp4"
	networkTypeUDP6Str = "udp6"
	networkTypeTCP4Str = "tcp4"
	networkTypeTCP6Str = "tcp6"
)

func (t NetworkType) String() string {
	switch t {
	case NetworkTypeUDP4:
		return networkTypeUDP4Str
	case NetworkTypeUDP6:
		return networkTypeUDP6Str
	case NetworkTypeTCP4:
		return networkTypeTCP4Str
	case NetworkTypeTCP6:
		return networkTypeTCP6Str
	default:
		return ErrUnknownType.Error()
	}
}

func newNetworkType(raw string) (NetworkType, error) {
	switch raw {
	case networkTypeUDP4Str:
		return NetworkTypeUDP4, nil
	case networkTypeUDP6Str:
		return NetworkTypeUDP6, nil
	case networkTypeTCP4Str:
		return NetworkTypeTCP4, nil
	case networkTypeTCP6Str:
		return NetworkTypeTCP6, nil
	default:
		return NetworkType(Unknown), fmt.Errorf("unknown network type: %s", raw)
	}
}
