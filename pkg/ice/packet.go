package ice

import (
	"net"
)

// Packet contains information about the incoming message from the network
type Packet struct {
	// Transport represents the connection and local address through which the
	// packet is arriving.
	Transport *Transport

	// Buffer contains the actual raw message bytes that were received.
	Buffer []byte

	// Addr represents the address information of the endpoint that submitted
	// the packet.
	Addr *net.UDPAddr
}
