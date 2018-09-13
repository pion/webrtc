package ice

import (
	"net"
)

const receiveMTU = 8192

type packet struct {
	transport *transport
	buffer    []byte
	addr      *net.UDPAddr
}

type transport struct {
	conn packetConn
	addr *net.UDPAddr

	onReceive func(*packet)
}

func newTransport(address string) (*transport, error) {
	listener, err := net.ListenPacket("udp", address)
	if err != nil {
		return nil, err
	}

	// addr, err := stun.NewTransportAddr(listener.LocalAddr())
	// if err != nil {
	// 	return nil, err
	// }

	var t *transport
	if addr := listener.LocalAddr().(*net.UDPAddr); addr.IP.To4() != nil {
		t = &transport{conn: newPacketConnIPv4(listener), addr: addr}
	} else {
		t = &transport{conn: newPacketConnIPv6(listener), addr: addr}
	}
	// dtls.AddListener(addr.String(), conn)

	go t.handler()
	return t, nil
}

func (t *transport) host() string {
	host, _, _ := net.SplitHostPort(t.addr.String())
	return host
}

func (t *transport) port() int {
	return t.addr.Port
}

func (t *transport) handler() {
	buffer := make([]byte, receiveMTU)
	for {
		n, _, addr, err := t.conn.readFrom(buffer)
		if err != nil {
			t.close()
			break
		}

		temp := make([]byte, n)
		copy(temp, buffer[:n])

		if t.onReceive != nil {
			go t.onReceive(&packet{
				transport: t,
				buffer:    temp,
				addr:      addr.(*net.UDPAddr),
			})
		}
	}
}

func (t *transport) send(raw []byte, cm controlMessage, remote *net.UDPAddr) error {
	if _, err := t.conn.writeTo(raw, cm, remote); err != nil {
		return err
	}
	return nil
}

func (t *transport) close() error {
	return t.conn.close()
}
