// +build !js

package webrtc

import (
	"crypto/rand"
	"encoding/binary"
	"math/big"
	"testing"
	"time"

	"github.com/pion/transport/test"
	"github.com/stretchr/testify/assert"
)

func TestGenerateDataChannelID(t *testing.T) {
	api := NewAPI()

	testCases := []struct {
		client bool
		c      *PeerConnection
		result uint16
	}{
		{true, &PeerConnection{sctpTransport: api.NewSCTPTransport(nil), dataChannels: map[uint16]*DataChannel{}, api: api}, 0},
		{true, &PeerConnection{sctpTransport: api.NewSCTPTransport(nil), dataChannels: map[uint16]*DataChannel{1: nil}, api: api}, 0},
		{true, &PeerConnection{sctpTransport: api.NewSCTPTransport(nil), dataChannels: map[uint16]*DataChannel{0: nil}, api: api}, 2},
		{true, &PeerConnection{sctpTransport: api.NewSCTPTransport(nil), dataChannels: map[uint16]*DataChannel{0: nil, 2: nil}, api: api}, 4},
		{true, &PeerConnection{sctpTransport: api.NewSCTPTransport(nil), dataChannels: map[uint16]*DataChannel{0: nil, 4: nil}, api: api}, 2},
		{false, &PeerConnection{sctpTransport: api.NewSCTPTransport(nil), dataChannels: map[uint16]*DataChannel{}, api: api}, 1},
		{false, &PeerConnection{sctpTransport: api.NewSCTPTransport(nil), dataChannels: map[uint16]*DataChannel{0: nil}, api: api}, 1},
		{false, &PeerConnection{sctpTransport: api.NewSCTPTransport(nil), dataChannels: map[uint16]*DataChannel{1: nil}, api: api}, 3},
		{false, &PeerConnection{sctpTransport: api.NewSCTPTransport(nil), dataChannels: map[uint16]*DataChannel{1: nil, 3: nil}, api: api}, 5},
		{false, &PeerConnection{sctpTransport: api.NewSCTPTransport(nil), dataChannels: map[uint16]*DataChannel{1: nil, 5: nil}, api: api}, 3},
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

func TestDataChannel_EventHandlers(t *testing.T) {
	to := test.TimeOut(time.Second * 20)
	defer to.Stop()

	report := test.CheckRoutines(t)
	defer report()

	api := NewAPI()
	dc := &DataChannel{api: api}

	onOpenCalled := make(chan struct{})
	onMessageCalled := make(chan struct{})

	// Verify that the noop case works
	assert.NotPanics(t, func() { dc.onOpen() })

	dc.OnOpen(func() {
		close(onOpenCalled)
	})

	dc.OnMessage(func(p DataChannelMessage) {
		close(onMessageCalled)
	})

	// Verify that the set handlers are called
	assert.NotPanics(t, func() { dc.onOpen() })
	assert.NotPanics(t, func() { dc.onMessage(DataChannelMessage{Data: []byte("o hai")}) })

	// Wait for all handlers to be called
	<-onOpenCalled
	<-onMessageCalled
}

func TestDataChannel_MessagesAreOrdered(t *testing.T) {
	report := test.CheckRoutines(t)
	defer report()

	api := NewAPI()
	dc := &DataChannel{api: api}

	max := 512
	out := make(chan int)
	inner := func(msg DataChannelMessage) {
		// randomly sleep
		// math/rand a weak RNG, but this does not need to be secure. Ignore with #nosec
		/* #nosec */
		randInt, err := rand.Int(rand.Reader, big.NewInt(int64(max)))
		/* #nosec */
		if err != nil {
			t.Fatalf("Failed to get random sleep duration: %s", err)
		}
		time.Sleep(time.Duration(randInt.Int64()) * time.Microsecond)
		s, _ := binary.Varint(msg.Data)
		out <- int(s)
	}
	dc.OnMessage(func(p DataChannelMessage) {
		inner(p)
	})

	go func() {
		for i := 1; i <= max; i++ {
			buf := make([]byte, 8)
			binary.PutVarint(buf, int64(i))
			dc.onMessage(DataChannelMessage{Data: buf})
			// Change the registered handler a couple of times to make sure
			// that everything continues to work, we don't lose messages, etc.
			if i%2 == 0 {
				hdlr := func(msg DataChannelMessage) {
					inner(msg)
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

// Note(albrow): This test includes some features that aren't supported by the
// Wasm bindings (at least for now).
func TestDataChannelParamters_Go(t *testing.T) {
	report := test.CheckRoutines(t)
	defer report()

	t.Run("MaxPacketLifeTime exchange", func(t *testing.T) {
		var ordered = true
		var maxPacketLifeTime uint16 = 3
		options := &DataChannelInit{
			Ordered:           &ordered,
			MaxPacketLifeTime: &maxPacketLifeTime,
		}

		offerPC, answerPC, dc, done := setUpReliabilityParamTest(t, options)

		// Check if parameters are correctly set
		assert.True(t, dc.Ordered(), "Ordered should be set to true")
		if assert.NotNil(t, dc.MaxPacketLifeTime(), "should not be nil") {
			assert.Equal(t, maxPacketLifeTime, *dc.MaxPacketLifeTime(), "should match")
		}

		answerPC.OnDataChannel(func(d *DataChannel) {
			// Make sure this is the data channel we were looking for. (Not the one
			// created in signalPair).
			if d.Label() != expectedLabel {
				return
			}

			// Check if parameters are correctly set
			assert.True(t, d.ordered, "Ordered should be set to true")
			if assert.NotNil(t, d.maxPacketLifeTime, "should not be nil") {
				assert.Equal(t, maxPacketLifeTime, *d.maxPacketLifeTime, "should match")
			}
			done <- true
		})

		closeReliabilityParamTest(t, offerPC, answerPC, done)
	})

	t.Run("All other property methods", func(t *testing.T) {
		id := uint16(123)
		dc := &DataChannel{}
		dc.id = &id
		dc.label = "mylabel"
		dc.protocol = "myprotocol"
		dc.negotiated = true
		dc.priority = PriorityTypeMedium

		assert.Equal(t, dc.id, dc.ID(), "should match")
		assert.Equal(t, dc.label, dc.Label(), "should match")
		assert.Equal(t, dc.protocol, dc.Protocol(), "should match")
		assert.Equal(t, dc.negotiated, dc.Negotiated(), "should match")
		assert.Equal(t, dc.priority, dc.Priority(), "should match")
		assert.Equal(t, dc.readyState, dc.ReadyState(), "should match")
		assert.Equal(t, uint64(0), dc.BufferedAmount(), "should match")
		dc.SetBufferedAmountLowThreshold(1500)
		assert.Equal(t, uint64(1500), dc.BufferedAmountLowThreshold(), "should match")
	})
}

func TestDataChannelBufferedAmount(t *testing.T) {
	t.Run("set before datachannel becomes open", func(t *testing.T) {
		report := test.CheckRoutines(t)
		defer report()

		var nCbs int
		buf := make([]byte, 1000)

		offerPC, answerPC, err := newPair()
		if err != nil {
			t.Fatalf("Failed to create a PC pair for testing")
		}

		done := make(chan bool)

		answerPC.OnDataChannel(func(d *DataChannel) {
			// Make sure this is the data channel we were looking for. (Not the one
			// created in signalPair).
			if d.Label() != expectedLabel {
				return
			}
			var nPacketsReceived int
			d.OnMessage(func(msg DataChannelMessage) {
				nPacketsReceived++

				if nPacketsReceived == 10 {
					go func() {
						time.Sleep(time.Second)
						done <- true
					}()
				}
			})
			assert.True(t, d.Ordered(), "Ordered should be set to true")
		})

		dc, err := offerPC.CreateDataChannel(expectedLabel, nil)
		if err != nil {
			t.Fatalf("Failed to create a PC pair for testing")
		}

		assert.True(t, dc.Ordered(), "Ordered should be set to true")

		dc.OnOpen(func() {
			for i := 0; i < 10; i++ {
				e := dc.Send(buf)
				if e != nil {
					t.Fatalf("Failed to send string on data channel")
				}
				assert.Equal(t, uint64(1500), dc.BufferedAmountLowThreshold(), "value mimatch")

				//assert.Equal(t, (i+1)*len(buf), int(dc.BufferedAmount()), "unexpected bufferedAmount")
			}
		})

		dc.OnMessage(func(msg DataChannelMessage) {
		})

		// The value is temporarily stored in the dc object
		// until the dc gets opened
		dc.SetBufferedAmountLowThreshold(1500)
		// The callback function is temporarily stored in the dc object
		// until the dc gets opened
		dc.OnBufferedAmountLow(func() {
			nCbs++
		})

		err = signalPair(offerPC, answerPC)
		if err != nil {
			t.Fatalf("Failed to signal our PC pair for testing")
		}

		closePair(t, offerPC, answerPC, done)

		assert.True(t, nCbs > 0, "callback should be made at least once")
	})

	t.Run("set after datachannel becomes open", func(t *testing.T) {
		report := test.CheckRoutines(t)
		defer report()

		var nCbs int
		buf := make([]byte, 1000)

		offerPC, answerPC, err := newPair()
		if err != nil {
			t.Fatalf("Failed to create a PC pair for testing")
		}

		done := make(chan bool)

		answerPC.OnDataChannel(func(d *DataChannel) {
			// Make sure this is the data channel we were looking for. (Not the one
			// created in signalPair).
			if d.Label() != expectedLabel {
				return
			}
			var nPacketsReceived int
			d.OnMessage(func(msg DataChannelMessage) {
				nPacketsReceived++

				if nPacketsReceived == 10 {
					go func() {
						time.Sleep(time.Second)
						done <- true
					}()
				}
			})
			assert.True(t, d.Ordered(), "Ordered should be set to true")
		})

		dc, err := offerPC.CreateDataChannel(expectedLabel, nil)
		if err != nil {
			t.Fatalf("Failed to create a PC pair for testing")
		}

		assert.True(t, dc.Ordered(), "Ordered should be set to true")

		dc.OnOpen(func() {
			// The value should directly be passed to sctp
			dc.SetBufferedAmountLowThreshold(1500)
			// The callback function should directly be passed to sctp
			dc.OnBufferedAmountLow(func() {
				nCbs++
			})

			for i := 0; i < 10; i++ {
				e := dc.Send(buf)
				if e != nil {
					t.Fatalf("Failed to send string on data channel")
				}
				assert.Equal(t, uint64(1500), dc.BufferedAmountLowThreshold(), "value mimatch")

				//assert.Equal(t, (i+1)*len(buf), int(dc.BufferedAmount()), "unexpected bufferedAmount")
			}
		})

		dc.OnMessage(func(msg DataChannelMessage) {
		})

		err = signalPair(offerPC, answerPC)
		if err != nil {
			t.Fatalf("Failed to signal our PC pair for testing")
		}

		closePair(t, offerPC, answerPC, done)

		assert.True(t, nCbs > 0, "callback should be made at least once")
	})
}
