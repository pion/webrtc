package ice

import (
	"net"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTransport(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		testCases := []struct {
			address string
			message []byte
		}{
			{"localhost:12345", []byte("unittest")},
			{"localhost:0", []byte("unittest")},
			{"[::1]:12345", []byte("unittest")},
			{"[::1]:0", []byte("unittest")},
		}

		for i, testCase := range testCases {
			var wg sync.WaitGroup
			transport, err := newTransport(testCase.address)

			wg.Add(1)
			transport.onReceive = func(packet *packet) {
				assert.Equal(t, testCase.message, packet.buffer,
					"testCase: %d %v", i, testCase,
				)
				wg.Done()
			}

			addr, err := net.ResolveUDPAddr("udp", transport.addr.String())
			assert.Nil(t, err)

			err = transport.send(testCase.message, nil, addr)
			assert.Nil(t, err)

			if err != nil {
				wg.Done()
			}

			wg.Wait()

			err = transport.close()
			assert.Nil(t, err)
		}
	})
	t.Run("Failure", func(t *testing.T) {
		testCases := []struct {
			address string
		}{
			{"[:..:1:0"},
		}

		for i, testCase := range testCases {
			_, err := newTransport(testCase.address)
			assert.NotNil(t, err, "testCase: %d %v", i, testCase)
		}
	})
}

func TestTransport_send(t *testing.T) {
	t.Run("Failure", func(t *testing.T) {
		testCases := []struct {
			address string
			message []byte
		}{
			{"localhost:12345", []byte("unittest")},
		}

		for i, testCase := range testCases {
			transport, err := newTransport(testCase.address)
			assert.Nil(t, err)

			cm := cmTester{}
			err = transport.send(testCase.message, &cm, transport.addr)
			assert.NotNil(t, err, "testCase: %d %v", i, testCase)
		}
	})
}
