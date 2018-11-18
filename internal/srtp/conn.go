package srtp

import (
	"io"
	"net"
	"time"
)

// This 'Conn' only serves to demux incoming SRTP for now

// Wrap wraps an io.ReadWriter to create a SRTP Conn
func Wrap(parent io.ReadWriter, inboundHandler func([]byte)) *Conn {
	return &Conn{
		parent:         parent,
		inboundHandler: inboundHandler,
	}
}

// Conn represents the SRTP Conn
type Conn struct {
	parent         io.ReadWriter
	inboundHandler func([]byte) // Temporarily call the existing SRTP handler
}

func isSRTP(p []byte) bool {
	return 127 < p[0] && p[0] < 192
}

// Read implements the Conn Read method.
func (c *Conn) Read(p []byte) (int, error) {
	for {
		i, err := c.parent.Read(p)
		if err != nil {
			return i, err
		}

		if !isSRTP(p[:i]) {
			return i, nil
		}

		c.inboundHandler(p[:i])
	}
}

// Write implements the Conn Write method.
func (c *Conn) Write(p []byte) (int, error) {
	return c.parent.Write(p)
}

// Close implements the Conn Close method.
func (c *Conn) Close() error {
	// TODO: Should unblock read/write
	return nil
}

// TODO: Maybe just switch to using io.ReadWriteCloser?

// LocalAddr is a stub
func (c *Conn) LocalAddr() net.Addr {
	return nil
}

// RemoteAddr is a stub
func (c *Conn) RemoteAddr() net.Addr {
	return nil
}

// SetDeadline is a stub
func (c *Conn) SetDeadline(t time.Time) error {
	return nil
}

// SetReadDeadline is a stub
func (c *Conn) SetReadDeadline(t time.Time) error {
	return nil
}

// SetWriteDeadline is a stub
func (c *Conn) SetWriteDeadline(t time.Time) error {
	return nil
}
