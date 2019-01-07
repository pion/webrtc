package quic

import (
	"crypto"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"sync"

	"github.com/pions/webrtc/pkg/quic/internal/wrapper"
)

// TransportBase is the base for Transport. Most of the
// functionality of a Transport is in the base class to allow for
// other subclasses (such as a p2p variant) to share the same interface.
type TransportBase struct {
	lock                      sync.RWMutex
	onBidirectionalStreamHdlr func(*BidirectionalStream)
	session                   *wrapper.Session
}

// Config is used to hold the configuration of StartBase
type Config struct {
	Client      bool
	Certificate *x509.Certificate
	PrivateKey  crypto.PrivateKey
}

// StartBase is used to start the TransportBase. Most implementations
// should instead use the methods on quic.Transport or
// webrtc.RTCQuicTransport to setup a Quic connection.
func (b *TransportBase) StartBase(conn net.Conn, config *Config) error {
	cfg := config.clone()
	if config.Client {
		// Assumes the peer offered to be passive and we accepted.
		s, err := wrapper.Client(conn, cfg)
		if err != nil {
			return err
		}
		b.session = s
	} else {
		// Assumes we offer to be passive and this is accepted.
		l, err := wrapper.Server(conn, cfg)
		if err != nil {
			return err
		}
		s, err := l.Accept()
		if err != nil {
			return err
		}
		b.session = s
	}

	go b.acceptStreams()

	return nil
}

func (c *Config) clone() *wrapper.Config {
	return &wrapper.Config{
		Certificate: c.Certificate,
		PrivateKey:  c.PrivateKey,
	}
}

// CreateBidirectionalStream creates an QuicBidirectionalStream object.
func (b *TransportBase) CreateBidirectionalStream() (*BidirectionalStream, error) {
	s, err := b.session.OpenStream()
	if err != nil {
		return nil, err
	}

	return &BidirectionalStream{
		s: s,
	}, nil
}

// OnBidirectionalStream allows setting an event handler for that is fired
// when data is received from a BidirectionalStream for the first time.
func (b *TransportBase) OnBidirectionalStream(f func(*BidirectionalStream)) {
	b.lock.Lock()
	defer b.lock.Unlock()
	b.onBidirectionalStreamHdlr = f
}

func (b *TransportBase) onBidirectionalStream(s *BidirectionalStream) {
	b.lock.Lock()
	f := b.onBidirectionalStreamHdlr
	b.lock.Unlock()
	if f != nil {
		go f(s)
	}
}

// GetRemoteCertificates returns the certificate chain in use by the remote side
func (b *TransportBase) GetRemoteCertificates() []*x509.Certificate {
	return b.session.GetRemoteCertificates()
}

func (b *TransportBase) acceptStreams() {
	for {
		s, err := b.session.AcceptStream()
		if err != nil {
			fmt.Println("Failed to accept stream:", err)
			// TODO: Kill TransportBase?
			return
		}

		stream := &BidirectionalStream{s: s}
		b.onBidirectionalStream(stream)
	}
}

// Stop stops and closes the TransportBase.
func (b *TransportBase) Stop(stopInfo TransportStopInfo) error {
	b.lock.Lock()
	defer b.lock.Unlock()

	if b.session == nil {
		return nil
	}

	if stopInfo.ErrorCode > 0 ||
		len(stopInfo.Reason) > 0 {
		return b.session.CloseWithError(stopInfo.ErrorCode, errors.New(stopInfo.Reason))
	}

	return b.session.Close()
}
