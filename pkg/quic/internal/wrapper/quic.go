// Package wrapper is a wrapper around lucas-clemente/quic-go to match
// the net.Conn based interface used troughout pions/webrtc.
package wrapper

import (
	"crypto"
	"crypto/tls"
	"crypto/x509"
	"net"

	quic "github.com/lucas-clemente/quic-go"
)

// Config represents the configuration of a Quic session
type Config struct {
	Certificate *x509.Certificate
	PrivateKey  crypto.PrivateKey
}

var quicConfig = &quic.Config{
	Versions:                              []quic.VersionNumber{101},
	MaxIncomingStreams:                    1000,
	MaxIncomingUniStreams:                 -1,              // disable unidirectional streams
	MaxReceiveStreamFlowControlWindow:     3 * (1 << 20),   // 3 MB
	MaxReceiveConnectionFlowControlWindow: 4.5 * (1 << 20), // 4.5 MB
	AcceptCookie: func(clientAddr net.Addr, cookie *quic.Cookie) bool {
		return true
	},
	KeepAlive: true,
}

// Client establishes a QUIC session over an existing conn
func Client(conn net.Conn, config *Config) (*Session, error) {
	tlscfg := getTLSConfig(config)
	s, err := quic.Dial(newFakePacketConn(conn), &fakeAddr{}, "localhost:1234", tlscfg, quicConfig)
	if err != nil {
		return nil, err
	}
	return &Session{s: s}, nil
}

// Server creates a listener for listens for incoming QUIC sessions
func Server(conn net.Conn, config *Config) (*Listener, error) {
	tlscfg := getTLSConfig(config)
	l, err := quic.Listen(newFakePacketConn(conn), tlscfg, quicConfig)
	if err != nil {
		return nil, err
	}
	return &Listener{l: l}, nil
}

func getTLSConfig(config *Config) *tls.Config {
	/* #nosec G402 */
	return &tls.Config{
		InsecureSkipVerify: true, // Using self signed certificates; WebRTC will check the fingerprint
		ClientAuth:         tls.RequireAnyClientCert,
		Certificates: []tls.Certificate{{
			Certificate: [][]byte{config.Certificate.Raw},
			PrivateKey:  config.PrivateKey,
		}},
	}
}

// A Session is a QUIC connection between two peers.
type Session struct {
	s quic.Session
}

// OpenStream opens a new stream
func (s *Session) OpenStream() (*Stream, error) {
	str, err := s.s.OpenStream()
	if err != nil {
		return nil, err
	}
	return &Stream{s: str}, nil
}

// AcceptStream accepts an incoming stream
func (s *Session) AcceptStream() (*Stream, error) {
	str, err := s.s.AcceptStream()
	if err != nil {
		return nil, err
	}
	return &Stream{s: str}, nil
}

// GetRemoteCertificates returns the certificate chain presented by remote peer.
func (s *Session) GetRemoteCertificates() []*x509.Certificate {
	return s.s.ConnectionState().PeerCertificates
}
