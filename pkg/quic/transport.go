// Package quic implements the QUIC API for Client-to-Server Connections
// https://w3c.github.io/webrtc-quic/
package quic

// Transport is a quic transport focused on client/server use cases.
type Transport struct {
	TransportBase
}

// NewTransport creates a new Transport
// func NewTransport(url string) (*Transport, error) {
// }
