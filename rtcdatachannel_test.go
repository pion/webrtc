package webrtc

import (
	"crypto/rand"
	"encoding/binary"
	"math/big"
	"testing"
	"time"

	"github.com/pions/webrtc/pkg/datachannel"
	"github.com/stretchr/testify/assert"
)

func TestGenerateDataChannelID(t *testing.T) {
	testCases := []struct {
		client bool
		c      *RTCPeerConnection
		result uint16
	}{
		{true, &RTCPeerConnection{sctpTransport: NewRTCSctpTransport(nil), dataChannels: map[uint16]*RTCDataChannel{}, api: defaultAPI}, 0},
		{true, &RTCPeerConnection{sctpTransport: NewRTCSctpTransport(nil), dataChannels: map[uint16]*RTCDataChannel{1: nil}, api: defaultAPI}, 0},
		{true, &RTCPeerConnection{sctpTransport: NewRTCSctpTransport(nil), dataChannels: map[uint16]*RTCDataChannel{0: nil}, api: defaultAPI}, 2},
		{true, &RTCPeerConnection{sctpTransport: NewRTCSctpTransport(nil), dataChannels: map[uint16]*RTCDataChannel{0: nil, 2: nil}, api: defaultAPI}, 4},
		{true, &RTCPeerConnection{sctpTransport: NewRTCSctpTransport(nil), dataChannels: map[uint16]*RTCDataChannel{0: nil, 4: nil}, api: defaultAPI}, 2},
		{false, &RTCPeerConnection{sctpTransport: NewRTCSctpTransport(nil), dataChannels: map[uint16]*RTCDataChannel{}, api: defaultAPI}, 1},
		{false, &RTCPeerConnection{sctpTransport: NewRTCSctpTransport(nil), dataChannels: map[uint16]*RTCDataChannel{0: nil}, api: defaultAPI}, 1},
		{false, &RTCPeerConnection{sctpTransport: NewRTCSctpTransport(nil), dataChannels: map[uint16]*RTCDataChannel{1: nil}, api: defaultAPI}, 3},
		{false, &RTCPeerConnection{sctpTransport: NewRTCSctpTransport(nil), dataChannels: map[uint16]*RTCDataChannel{1: nil, 3: nil}, api: defaultAPI}, 5},
		{false, &RTCPeerConnection{sctpTransport: NewRTCSctpTransport(nil), dataChannels: map[uint16]*RTCDataChannel{1: nil, 5: nil}, api: defaultAPI}, 3},
	}

	for _, testCase := range testCases {
		id, err := testCase.c.generateDataChannelID(testCase.client)
		if err != nil {
			t.Errorf("failed to generate id: %v", err)
			return
		}
		if id != testCase.result {
			t.Errorf("Wrong id: %d expected %d", id, testCase.result)
		}
	}
}

func TestRTCDataChannel_EventHandlers(t *testing.T) {
	dc := &RTCDataChannel{settingEngine: &defaultAPI.settingEngine}

	onOpenCalled := make(chan bool)
	onMessageCalled := make(chan bool)

	// Verify that the noop case works
	assert.NotPanics(t, func() { dc.onOpen() })
	assert.NotPanics(t, func() { dc.onMessage(nil) })

	dc.OnOpen(func() {
		onOpenCalled <- true
	})

	dc.OnMessage(func(p datachannel.Payload) {
		go func() {
			onMessageCalled <- true
		}()
	})

	// Verify that the handlers deal with nil inputs
	assert.NotPanics(t, func() { dc.onMessage(nil) })

	// Verify that the set handlers are called
	assert.NotPanics(t, func() { dc.onOpen() })
	assert.NotPanics(t, func() { dc.onMessage(&datachannel.PayloadString{Data: []byte("o hai")}) })

	allTrue := func(vals []bool) bool {
		for _, val := range vals {
			if !val {
				return false
			}
		}
		return true
	}

	assert.True(t, allTrue([]bool{
		<-onOpenCalled,
		<-onMessageCalled,
	}))
}

func TestRTCDataChannel_MessagesAreOrdered(t *testing.T) {
	dc := &RTCDataChannel{settingEngine: &defaultAPI.settingEngine}

	max := 512
	out := make(chan int)
	inner := func(p datachannel.Payload) {
		// randomly sleep
		// NB: The big.Int/crypto.Rand is overkill but makes the linter happy
		randInt, err := rand.Int(rand.Reader, big.NewInt(int64(max)))
		if err != nil {
			t.Fatalf("Failed to get random sleep duration: %s", err)
		}
		time.Sleep(time.Duration(randInt.Int64()) * time.Microsecond)
		switch p := p.(type) {
		case *datachannel.PayloadBinary:
			s, _ := binary.Varint(p.Data)
			out <- int(s)
		}
	}
	dc.OnMessage(func(p datachannel.Payload) {
		inner(p)
	})

	go func() {
		for i := 1; i <= max; i++ {
			buf := make([]byte, 8)
			binary.PutVarint(buf, int64(i))
			dc.onMessage(&datachannel.PayloadBinary{Data: buf})
			// Change the registered handler a couple of times to make sure
			// that everything continues to work, we don't lose messages, etc.
			if i%2 == 0 {
				hdlr := func(p datachannel.Payload) {
					inner(p)
				}
				dc.OnMessage(hdlr)
			}
		}
	}()

	values := make([]int, 0, max)
	for v := range out {
		values = append(values, v)
		if len(values) == max {
			close(out)
		}
	}

	expected := make([]int, max)
	for i := 1; i <= max; i++ {
		expected[i-1] = i
	}
	assert.EqualValues(t, expected, values)
}
