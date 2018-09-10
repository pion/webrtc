package ice

import (
	"fmt"
	"net"

	"github.com/pions/pkg/stun"
	"github.com/pions/webrtc/internal/dtls"
	"golang.org/x/net/ipv4"
)

const receiveMTU = 8192

type Packet struct {
	Buffer []byte
	Addr   *net.UDPAddr
}

type Transport struct {
	Conn *ipv4.PacketConn
	Addr *stun.TransportAddr

	OnReceive func(*Transport, *Packet)
}

func NewTransport(address string) (*Transport, error) {
	listener, err := net.ListenPacket("udp4", address)
	if err != nil {
		return nil, err
	}

	addr, err := stun.NewTransportAddr(listener.LocalAddr())
	if err != nil {
		return nil, err
	}

	conn := ipv4.NewPacketConn(listener)
	dtls.AddListener(addr.String(), conn)

	transport := &Transport{
		Conn: conn,
		Addr: addr,
	}

	go transport.handler()
	return transport, nil
}

func (t *Transport) handler() {
	buffer := make([]byte, receiveMTU)
	for {
		n, _, addr, err := t.Conn.ReadFrom(buffer)
		if err != nil {
			t.Close()
			break
		}

		temp := make([]byte, n)
		copy(temp, buffer[:n])

		if t.OnReceive != nil {
			t.OnReceive(t, &Packet{
				Buffer: temp,
				Addr:   addr.(*net.UDPAddr),
			})
		}
	}
}

func (t *Transport) Send(raw []byte, remote *net.UDPAddr) error {
	if _, err := t.Conn.WriteTo(raw, nil, remote); err != nil {
		fmt.Printf("Failed to send packet: %s \n", err.Error())
	}
}

func (t *Transport) Close() error {
	return t.Conn.Close()
}
