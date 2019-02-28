package mux

import (
	"net"
)

type connection interface {
	net.Conn

	WriteBatch(packets [][]byte) (n int, err error)
	ReadBatch(packets [][]byte) (n int, err error)
}

type simpleBatcher struct {
	net.Conn
}

func (sb simpleBatcher) WriteBatch(packets [][]byte) (n int, err error) {
	for i, packet := range packets {
		_, err = sb.Write(packet)
		if err != nil {
			return i, err
		}
	}

	return len(packets), nil
}

func (sb simpleBatcher) ReadBatch(packets [][]byte) (n int, err error) {
	if len(packets) == 0 {
		return 0, nil
	}

	_, err = sb.Read(packets[0])
	if err != nil {
		return 0, err
	}

	return 1, nil
}

func newConnection(conn net.Conn) (c connection) {
	// See if the connection already obeys the interface, giving us full batching support.
	c, ok := conn.(connection)
	if ok {
		return c
	}

	// Otherwise use a wrapper that translates the batch calls into single reads/writes
	return simpleBatcher{conn}
}
