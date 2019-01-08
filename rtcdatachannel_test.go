package webrtc

import (
	"crypto/rand"
	"encoding/binary"
	"math/big"
	"testing"
	"time"

	sugar "github.com/pions/webrtc/pkg/datachannel"

	"github.com/pions/transport/test"
	"github.com/stretchr/testify/assert"
)

func TestGenerateDataChannelID(t *testing.T) {
	api := NewAPI()

	testCases := []struct {
		client bool
		c      *RTCPeerConnection
		result uint16
	}{
		{true, &RTCPeerConnection{sctpTransport: NewRTCSctpTransport(nil), dataChannels: map[uint16]*RTCDataChannel{}, api: api}, 0},
		{true, &RTCPeerConnection{sctpTransport: NewRTCSctpTransport(nil), dataChannels: map[uint16]*RTCDataChannel{1: nil}, api: api}, 0},
		{true, &RTCPeerConnection{sctpTransport: NewRTCSctpTransport(nil), dataChannels: map[uint16]*RTCDataChannel{0: nil}, api: api}, 2},
		{true, &RTCPeerConnection{sctpTransport: NewRTCSctpTransport(nil), dataChannels: map[uint16]*RTCDataChannel{0: nil, 2: nil}, api: api}, 4},
		{true, &RTCPeerConnection{sctpTransport: NewRTCSctpTransport(nil), dataChannels: map[uint16]*RTCDataChannel{0: nil, 4: nil}, api: api}, 2},
		{false, &RTCPeerConnection{sctpTransport: NewRTCSctpTransport(nil), dataChannels: map[uint16]*RTCDataChannel{}, api: api}, 1},
		{false, &RTCPeerConnection{sctpTransport: NewRTCSctpTransport(nil), dataChannels: map[uint16]*RTCDataChannel{0: nil}, api: api}, 1},
		{false, &RTCPeerConnection{sctpTransport: NewRTCSctpTransport(nil), dataChannels: map[uint16]*RTCDataChannel{1: nil}, api: api}, 3},
		{false, &RTCPeerConnection{sctpTransport: NewRTCSctpTransport(nil), dataChannels: map[uint16]*RTCDataChannel{1: nil, 3: nil}, api: api}, 5},
		{false, &RTCPeerConnection{sctpTransport: NewRTCSctpTransport(nil), dataChannels: map[uint16]*RTCDataChannel{1: nil, 5: nil}, api: api}, 3},
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

func TestRTCDataChannel_Send(t *testing.T) {
	report := test.CheckRoutines(t)
	defer report()

	api := NewAPI()
	offerPC, answerPC, err := api.newPair()

	if err != nil {
		t.Fatalf("Failed to create a PC pair for testing")
	}

	done := make(chan bool)

	dc, err := offerPC.CreateDataChannel("data", nil)

	if err != nil {
		t.Fatalf("Failed to create a PC pair for testing")
	}

	dc.OnOpen(func() {
		e := dc.Send(sugar.PayloadString{Data: []byte("Ping")})
		if e != nil {
			t.Fatalf("Failed to send string on data channel")
		}
	})
	dc.OnMessage(func(payload sugar.Payload) {
		done <- true
	})

	answerPC.OnDataChannel(func(d *RTCDataChannel) {
		d.OnMessage(func(payload sugar.Payload) {
			e := d.Send(sugar.PayloadBinary{Data: []byte("Pong")})
			if e != nil {
				t.Fatalf("Failed to send string on data channel")
			}
		})
	})

	err = signalPair(offerPC, answerPC)

	if err != nil {
		t.Fatalf("Failed to signal our PC pair for testing")
	}

	select {
	case <-time.After(10 * time.Second):
		t.Fatalf("Datachannel Send Test Timeout")
	case <-done:
		err = offerPC.Close()
		if err != nil {
			t.Fatalf("Failed to close offer PC")
		}
		err = answerPC.Close()
		if err != nil {
			t.Fatalf("Failed to close answer PC")
		}
	}
}

func TestRTCDataChannel_EventHandlers(t *testing.T) {
	report := test.CheckRoutines(t)
	defer report()

	api := NewAPI()
	dc := &RTCDataChannel{settingEngine: &api.settingEngine}

	onOpenCalled := make(chan bool)
	onMessageCalled := make(chan bool)

	// Verify that the noop case works
	assert.NotPanics(t, func() { dc.onOpen() })
	assert.NotPanics(t, func() { dc.onMessage(nil) })

	dc.OnOpen(func() {
		onOpenCalled <- true
	})

	dc.OnMessage(func(p sugar.Payload) {
		go func() {
			onMessageCalled <- true
		}()
	})

	// Verify that the handlers deal with nil inputs
	assert.NotPanics(t, func() { dc.onMessage(nil) })

	// Verify that the set handlers are called
	assert.NotPanics(t, func() { dc.onOpen() })
	assert.NotPanics(t, func() { dc.onMessage(&sugar.PayloadString{Data: []byte("o hai")}) })

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
	report := test.CheckRoutines(t)
	defer report()

	api := NewAPI()
	dc := &RTCDataChannel{settingEngine: &api.settingEngine}

	max := 512
	out := make(chan int)
	inner := func(p sugar.Payload) {
		// randomly sleep
		// NB: The big.Int/crypto.Rand is overkill but makes the linter happy
		randInt, err := rand.Int(rand.Reader, big.NewInt(int64(max)))
		if err != nil {
			t.Fatalf("Failed to get random sleep duration: %s", err)
		}
		time.Sleep(time.Duration(randInt.Int64()) * time.Microsecond)
		switch p := p.(type) {
		case *sugar.PayloadBinary:
			s, _ := binary.Varint(p.Data)
			out <- int(s)
		}
	}
	dc.OnMessage(func(p sugar.Payload) {
		inner(p)
	})

	go func() {
		for i := 1; i <= max; i++ {
			buf := make([]byte, 8)
			binary.PutVarint(buf, int64(i))
			dc.onMessage(&sugar.PayloadBinary{Data: buf})
			// Change the registered handler a couple of times to make sure
			// that everything continues to work, we don't lose messages, etc.
			if i%2 == 0 {
				hdlr := func(p sugar.Payload) {
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
