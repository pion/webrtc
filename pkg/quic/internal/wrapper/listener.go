package wrapper

import (
	quic "github.com/pions/quic-go"
)

// A Listener for incoming QUIC connections
type Listener struct {
	l quic.Listener
}

// Accept accepts incoming streams
func (l *Listener) Accept() (*Session, error) {
	s, err := l.l.Accept()
	if err != nil {
		return nil, err
	}
	return &Session{s: s}, nil
}

// Close closes the listener
func (l *Listener) Close() error {
	return l.l.Close()
}
