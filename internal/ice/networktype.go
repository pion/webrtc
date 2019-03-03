package ice

import (
	"fmt"
	"net"
	"strings"
)

const (
	udp = "udp"
	tcp = "tcp"
)

var supportedNetworks = []string{
	udp,
	// tcp, // Not supported yet
}

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

func (t NetworkType) String() string {
	switch t {
	case NetworkTypeUDP4:
		return "udp4"
	case NetworkTypeUDP6:
		return "udp6"
	case NetworkTypeTCP4:
		return "tcp4"
	case NetworkTypeTCP6:
		return "tcp6"
	default:
		return ErrUnknownType.Error()
	}
}

// NetworkShort returns the short network description
func (t NetworkType) NetworkShort() string {
	switch t {
	case NetworkTypeUDP4, NetworkTypeUDP6:
		return udp
	case NetworkTypeTCP4, NetworkTypeTCP6:
		return tcp
	default:
		return ErrUnknownType.Error()
	}
}

// IsReliable returns true if the network is reliable
func (t NetworkType) IsReliable() bool {
	switch t {
	case NetworkTypeUDP4, NetworkTypeUDP6:
		return false
	case NetworkTypeTCP4, NetworkTypeTCP6:
		return true
	}
	return false
}

// IsIPv4 returns whether the network type is IPv4 or not.
func (t NetworkType) IsIPv4() bool {
	switch t {
	case NetworkTypeUDP4, NetworkTypeTCP4:
		return true
	case NetworkTypeUDP6, NetworkTypeTCP6:
		return false
	}
	return false
}

// IsIPv6 returns whether the network type is IPv6 or not.
func (t NetworkType) IsIPv6() bool {
	switch t {
	case NetworkTypeUDP4, NetworkTypeTCP4:
		return false
	case NetworkTypeUDP6, NetworkTypeTCP6:
		return true
	}
	return false
}

// determineNetworkType determines the type of network based on
// the short network string and an IP address.
func determineNetworkType(network string, ip net.IP) (NetworkType, error) {
	ipv4 := ip.To4() != nil

	switch {
	case strings.HasPrefix(strings.ToLower(network), udp):
		if ipv4 {
			return NetworkTypeUDP4, nil
		}
		return NetworkTypeUDP6, nil

	case strings.HasPrefix(strings.ToLower(network), tcp):
		if ipv4 {
			return NetworkTypeTCP4, nil
		}
		return NetworkTypeTCP6, nil

	}

	return NetworkType(0), fmt.Errorf("unable to determine networkType from %s %s", network, ip)
}
