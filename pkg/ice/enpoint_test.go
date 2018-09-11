package ice

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type cmTester struct {
}

func (c *cmTester) String() string {
	return ""
}

func (c *cmTester) Marshal() []byte {
	return []byte("")
}

func TestNewPacketConnIPv4(t *testing.T) {
	testCases := []struct {
		address string
		message []byte
	}{
		{"localhost:12345", []byte("unittest")},
		{"localhost:0", []byte("unittest")},
	}

	for _, testCase := range testCases {
		listener, err := net.ListenPacket("udp", testCase.address)
		assert.Nil(t, err)

		conn := newPacketConnIPv4(listener)

		err = conn.setReadDeadline(time.Now().Add(time.Second * 1))
		assert.Nil(t, err)

		err = conn.setDeadline(time.Now().Add(time.Second * 1))
		assert.Nil(t, err)

		err = conn.setWriteDeadline(time.Now().Add(time.Second * 1))
		assert.Nil(t, err)

		_, err = conn.writeTo([]byte("unittest"), nil, conn.localAddr())
		assert.Nil(t, err)

		var buffer []byte
		_, _, _, err = conn.readFrom(buffer)
		assert.Nil(t, err)

		cm := cmTester{}
		_, err = conn.writeTo([]byte("unittest"), &cm, conn.localAddr())
		assert.NotNil(t, err)

		err = conn.close()
		assert.Nil(t, err)
	}
}

func TestNewPacketConnIPv6(t *testing.T) {
	testCases := []struct {
		address string
		message []byte
	}{
		{"[::1]:12345", []byte("unittest")},
		{"[::1]:0", []byte("unittest")},
	}

	for _, testCase := range testCases {
		listener, err := net.ListenPacket("udp", testCase.address)
		assert.Nil(t, err)

		conn := newPacketConnIPv6(listener)

		err = conn.setReadDeadline(time.Now().Add(time.Second * 1))
		assert.Nil(t, err)

		err = conn.setDeadline(time.Now().Add(time.Second * 1))
		assert.Nil(t, err)

		err = conn.setWriteDeadline(time.Now().Add(time.Second * 1))
		assert.Nil(t, err)

		_, err = conn.writeTo([]byte("unittest"), nil, conn.localAddr())
		assert.Nil(t, err)

		var buffer []byte
		_, _, _, err = conn.readFrom(buffer)
		assert.Nil(t, err)

		cm := cmTester{}
		_, err = conn.writeTo([]byte("unittest"), &cm, conn.localAddr())
		assert.NotNil(t, err)

		err = conn.close()
		assert.Nil(t, err)
	}
}
