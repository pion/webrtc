package test

import (
	"net"
	"testing"
)

func BenchmarkPipe(b *testing.B) {
	ca, cb := net.Pipe()

	b.ResetTimer()

	opt := Options{
		MsgSize:  2048,
		MsgCount: b.N,
	}

	check(Stress(ca, cb, opt))
}

func BenchmarkUDP(b *testing.B) {
	var ca net.Conn
	var cb net.Conn

	ca, err := net.ListenUDP("udp", nil)
	check(err)
	defer func() {
		check(ca.Close())
	}()

	cb, err = net.Dial("udp", ca.LocalAddr().String())
	check(err)
	defer func() {
		check(cb.Close())
	}()

	b.ResetTimer()

	opt := Options{
		MsgSize:  2048,
		MsgCount: b.N,
	}

	check(Stress(cb, ca, opt))
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}
