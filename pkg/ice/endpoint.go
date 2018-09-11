package ice

import (
	"net"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

type controlMessage interface {
	String() string
	Marshal() []byte
}

type packetConn interface {
	readFrom(p []byte) (int, controlMessage, net.Addr, error)
	writeTo(p []byte, cm controlMessage, addr net.Addr) (int, error)
	close() error
	localAddr() *net.UDPAddr
	setDeadline(t time.Time) error
	setReadDeadline(t time.Time) error
	setWriteDeadline(t time.Time) error
}

type packetConnIPv4 struct {
	conn *ipv4.PacketConn
}

func newPacketConnIPv4(c net.PacketConn) *packetConnIPv4 {
	return &packetConnIPv4{conn: ipv4.NewPacketConn(c)}
}

func (c *packetConnIPv4) readFrom(b []byte) (int, controlMessage, net.Addr, error) {
	n, cm, addr, err := c.conn.ReadFrom(b)
	return n, cm, addr, errors.Wrap(err, "failed reading packet")
}

func (c *packetConnIPv4) writeTo(b []byte, cm controlMessage, addr net.Addr) (int, error) {
	ip4cm, ok := cm.(*ipv4.ControlMessage)
	if cm != nil && !ok {
		return 0, errors.Errorf("failed type assertion %#v.(*ipv4.ControlMessage)", cm)
	}

	n, err := c.conn.WriteTo(b, ip4cm, addr)
	return n, errors.Wrap(err, "failed sending packet")
}

func (c *packetConnIPv4) close() error {
	return c.conn.Close()
}

func (c *packetConnIPv4) localAddr() *net.UDPAddr {
	return c.conn.LocalAddr().(*net.UDPAddr)
}

func (c *packetConnIPv4) setDeadline(t time.Time) error {
	return c.conn.SetDeadline(t)
}

func (c *packetConnIPv4) setReadDeadline(t time.Time) error {
	return c.conn.SetReadDeadline(t)
}

func (c *packetConnIPv4) setWriteDeadline(t time.Time) error {
	return c.conn.SetWriteDeadline(t)
}

type packetConnIPv6 struct {
	conn *ipv6.PacketConn
}

func newPacketConnIPv6(c net.PacketConn) *packetConnIPv6 {
	return &packetConnIPv6{conn: ipv6.NewPacketConn(c)}
}

func (c *packetConnIPv6) readFrom(b []byte) (int, controlMessage, net.Addr, error) {
	n, cm, addr, err := c.conn.ReadFrom(b)
	return n, cm, addr, errors.Wrap(err, "failed reading packet")
}

func (c *packetConnIPv6) writeTo(b []byte, cm controlMessage, addr net.Addr) (int, error) {
	ip6cm, ok := cm.(*ipv6.ControlMessage)
	if cm != nil && !ok {
		return 0, errors.Errorf("failed type assertion %#v.(*ipv6.ControlMessage)", cm)
	}

	n, err := c.conn.WriteTo(b, ip6cm, addr)
	return n, errors.Wrap(err, "failed sending packet")
}

func (c *packetConnIPv6) close() error {
	return c.conn.Close()
}

func (c *packetConnIPv6) localAddr() *net.UDPAddr {
	return c.conn.LocalAddr().(*net.UDPAddr)
}

func (c *packetConnIPv6) setDeadline(t time.Time) error {
	return c.conn.SetDeadline(t)
}

func (c *packetConnIPv6) setReadDeadline(t time.Time) error {
	return c.conn.SetReadDeadline(t)
}

func (c *packetConnIPv6) setWriteDeadline(t time.Time) error {
	return c.conn.SetWriteDeadline(t)
}
