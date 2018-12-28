package wrapper

import (
	"net"
	"time"
)

type fakePacketConn struct {
	c net.Conn
}

func newFakePacketConn(conn net.Conn) *fakePacketConn {
	return &fakePacketConn{
		c: conn,
	}
}

func (c *fakePacketConn) ReadFrom(p []byte) (int, net.Addr, error) {
	n, err := c.c.Read(p)
	return n, nil, err
}

func (c *fakePacketConn) WriteTo(p []byte, addr net.Addr) (int, error) {
	n, err := c.c.Write(p)
	return n, err
}

func (c *fakePacketConn) Close() error {
	return c.c.Close()
}

func (c *fakePacketConn) LocalAddr() net.Addr {
	return &fakeAddr{}
}

func (c *fakePacketConn) SetDeadline(t time.Time) error {
	return c.c.SetDeadline(t)
}

func (c *fakePacketConn) SetReadDeadline(t time.Time) error {
	return c.c.SetReadDeadline(t)
}

func (c *fakePacketConn) SetWriteDeadline(t time.Time) error {
	return c.c.SetWriteDeadline(t)
}

type fakeAddr struct {
	ID string
}

func (a *fakeAddr) Network() string {
	return "WebRTC"
}

func (a *fakeAddr) String() string {
	return "fakeAddr"
}
