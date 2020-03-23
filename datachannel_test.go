package webrtc

import (
	"io"
	"sync"
	"testing"
	"time"

	"github.com/pion/transport/test"
	"github.com/stretchr/testify/assert"
)

// expectedLabel represents the label of the data channel we are trying to test.
// Some other channels may have been created during initialization (in the Wasm
// bindings this is a requirement).
const expectedLabel = "data"

func closePairNow(t *testing.T, pc1, pc2 io.Closer) {
	var fail bool
	if err := pc1.Close(); err != nil {
		t.Errorf("Failed to close PeerConnection: %v", err)
		fail = true
	}
	if err := pc2.Close(); err != nil {
		t.Errorf("Failed to close PeerConnection: %v", err)
		fail = true
	}
	if fail {
		t.FailNow()
	}
}

func closePair(t *testing.T, pc1, pc2 io.Closer, done <-chan bool) {
	select {
	case <-time.After(10 * time.Second):
		t.Fatalf("closePair timed out waiting for done signal")
	case <-done:
		closePairNow(t, pc1, pc2)
	}
}

func setUpDataChannelParametersTest(t *testing.T, options *DataChannelInit) (*PeerConnection, *PeerConnection, *DataChannel, chan bool) {
	offerPC, answerPC, err := newPair()
	if err != nil {
		t.Fatalf("Failed to create a PC pair for testing")
	}
	done := make(chan bool)

	dc, err := offerPC.CreateDataChannel(expectedLabel, options)
	if err != nil {
		t.Fatalf("Failed to create a PC pair for testing")
	}

	return offerPC, answerPC, dc, done
}

func closeReliabilityParamTest(t *testing.T, pc1, pc2 *PeerConnection, done chan bool) {
	err := signalPair(pc1, pc2)
	if err != nil {
		t.Fatalf("Failed to signal our PC pair for testing")
	}

	closePair(t, pc1, pc2, done)
}

func TestDataChannel_Open(t *testing.T) {
	t.Run("handler should be called once", func(t *testing.T) {
		report := test.CheckRoutines(t)
		defer report()

		offerPC, answerPC, err := newPair()
		if err != nil {
			t.Fatalf("Failed to create a PC pair for testing")
		}

		done := make(chan bool)
		openCalls := make(chan bool, 2)

		answerPC.OnDataChannel(func(d *DataChannel) {
			if d.Label() != expectedLabel {
				return
			}
			d.OnOpen(func() {
				openCalls <- true
			})
			d.OnMessage(func(msg DataChannelMessage) {
				go func() {
					// Wait a little bit to ensure all messages are processed.
					time.Sleep(100 * time.Millisecond)
					done <- true
				}()
			})
		})

		dc, err := offerPC.CreateDataChannel(expectedLabel, nil)
		assert.NoError(t, err)

		dc.OnOpen(func() {
			e := dc.SendText("Ping")
			if e != nil {
				t.Fatalf("Failed to send string on data channel")
			}
		})

		assert.NoError(t, signalPair(offerPC, answerPC))

		closePair(t, offerPC, answerPC, done)

		assert.Len(t, openCalls, 1)
	})
}

func TestDataChannel_Send(t *testing.T) {
	t.Run("before signaling", func(t *testing.T) {
		report := test.CheckRoutines(t)
		defer report()

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
			d.OnMessage(func(msg DataChannelMessage) {
				e := d.Send([]byte("Pong"))
				if e != nil {
					t.Fatalf("Failed to send string on data channel")
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
			e := dc.SendText("Ping")
			if e != nil {
				t.Fatalf("Failed to send string on data channel")
			}
		})
		dc.OnMessage(func(msg DataChannelMessage) {
			done <- true
		})

		err = signalPair(offerPC, answerPC)
		if err != nil {
			t.Fatalf("Failed to signal our PC pair for testing: %+v", err)
		}

		closePair(t, offerPC, answerPC, done)
	})

	t.Run("after connected", func(t *testing.T) {
		report := test.CheckRoutines(t)
		defer report()

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
			d.OnMessage(func(msg DataChannelMessage) {
				e := d.Send([]byte("Pong"))
				if e != nil {
					t.Fatalf("Failed to send string on data channel")
				}
			})
			assert.True(t, d.Ordered(), "Ordered should be set to true")
		})

		once := &sync.Once{}
		offerPC.OnICEConnectionStateChange(func(state ICEConnectionState) {
			if state == ICEConnectionStateConnected || state == ICEConnectionStateCompleted {
				// wasm fires completed state multiple times
				once.Do(func() {
					dc, createErr := offerPC.CreateDataChannel(expectedLabel, nil)
					if createErr != nil {
						t.Fatalf("Failed to create a PC pair for testing")
					}

					assert.True(t, dc.Ordered(), "Ordered should be set to true")

					dc.OnMessage(func(msg DataChannelMessage) {
						done <- true
					})
					// TODO: currently there is no way of properly subscribing to OnOpen with the js binding,
					// because CreateDataChannel might return an already open data channel
					//
					e := dc.SendText("Ping")
					if e != nil {
						// wasm binding doesn't fire OnOpen (we probably already missed it)
						dc.OnOpen(func() {
							e = dc.SendText("Ping")
							if e != nil {
								t.Fatalf("Failed to send string on data channel")
							}
						})
					}
				})
			}
		})

		err = signalPair(offerPC, answerPC)
		if err != nil {
			t.Fatalf("Failed to signal our PC pair for testing")
		}

		closePair(t, offerPC, answerPC, done)
	})
}

func TestDataChannel_Close(t *testing.T) {
	report := test.CheckRoutines(t)
	defer report()

	t.Run("Close after PeerConnection Closed", func(t *testing.T) {
		offerPC, answerPC, err := newPair()
		assert.NoError(t, err)

		dc, err := offerPC.CreateDataChannel(expectedLabel, nil)
		assert.NoError(t, err)

		assert.NoError(t, offerPC.Close())
		assert.NoError(t, answerPC.Close())
		assert.NoError(t, dc.Close())
	})

	t.Run("Close before connected", func(t *testing.T) {
		offerPC, answerPC, err := newPair()
		assert.NoError(t, err)

		dc, err := offerPC.CreateDataChannel(expectedLabel, nil)
		assert.NoError(t, err)

		assert.NoError(t, dc.Close())
		assert.NoError(t, offerPC.Close())
		assert.NoError(t, answerPC.Close())
	})
}

func TestDataChannelParameters(t *testing.T) {
	report := test.CheckRoutines(t)
	defer report()

	t.Run("MaxPacketLifeTime exchange", func(t *testing.T) {
		ordered := true
		maxPacketLifeTime := uint16(3)
		options := &DataChannelInit{
			Ordered:           &ordered,
			MaxPacketLifeTime: &maxPacketLifeTime,
		}

		offerPC, answerPC, dc, done := setUpDataChannelParametersTest(t, options)

		// Check if parameters are correctly set
		assert.Equal(t, dc.Ordered(), ordered, "Ordered should be same value as set in DataChannelInit")
		if assert.NotNil(t, dc.MaxPacketLifeTime(), "should not be nil") {
			assert.Equal(t, maxPacketLifeTime, *dc.MaxPacketLifeTime(), "should match")
		}

		answerPC.OnDataChannel(func(d *DataChannel) {
			if d.Label() != expectedLabel {
				return
			}
			// Check if parameters are correctly set
			assert.Equal(t, d.Ordered(), ordered, "Ordered should be same value as set in DataChannelInit")
			if assert.NotNil(t, d.MaxPacketLifeTime(), "should not be nil") {
				assert.Equal(t, maxPacketLifeTime, *d.MaxPacketLifeTime(), "should match")
			}
			done <- true
		})

		closeReliabilityParamTest(t, offerPC, answerPC, done)
	})

	t.Run("MaxRetransmits exchange", func(t *testing.T) {
		ordered := false
		maxRetransmits := uint16(3000)
		options := &DataChannelInit{
			Ordered:        &ordered,
			MaxRetransmits: &maxRetransmits,
		}

		offerPC, answerPC, dc, done := setUpDataChannelParametersTest(t, options)

		// Check if parameters are correctly set
		assert.False(t, dc.Ordered(), "Ordered should be set to false")
		if assert.NotNil(t, dc.MaxRetransmits(), "should not be nil") {
			assert.Equal(t, maxRetransmits, *dc.MaxRetransmits(), "should match")
		}

		answerPC.OnDataChannel(func(d *DataChannel) {
			// Make sure this is the data channel we were looking for. (Not the one
			// created in signalPair).
			if d.Label() != expectedLabel {
				return
			}
			// Check if parameters are correctly set
			assert.False(t, d.Ordered(), "Ordered should be set to false")
			if assert.NotNil(t, d.MaxRetransmits(), "should not be nil") {
				assert.Equal(t, maxRetransmits, *d.MaxRetransmits(), "should match")
			}
			done <- true
		})

		closeReliabilityParamTest(t, offerPC, answerPC, done)
	})

	t.Run("Protocol exchange", func(t *testing.T) {
		protocol := "json"
		options := &DataChannelInit{
			Protocol: &protocol,
		}

		offerPC, answerPC, dc, done := setUpDataChannelParametersTest(t, options)

		// Check if parameters are correctly set
		assert.Equal(t, protocol, dc.Protocol(), "Protocol should match DataChannelInit")

		answerPC.OnDataChannel(func(d *DataChannel) {
			// Make sure this is the data channel we were looking for. (Not the one
			// created in signalPair).
			if d.Label() != expectedLabel {
				return
			}
			// Check if parameters are correctly set
			assert.Equal(t, protocol, d.Protocol(), "Protocol should match what channel creator declared")
			done <- true
		})

		closeReliabilityParamTest(t, offerPC, answerPC, done)
	})

	t.Run("Negotiated exchange", func(t *testing.T) {
		const expectedMessage = "Hello World"

		negotiated := true
		var id uint16 = 500
		options := &DataChannelInit{
			Negotiated: &negotiated,
			ID:         &id,
		}

		offerPC, answerPC, offerDatachannel, done := setUpDataChannelParametersTest(t, options)
		answerDatachannel, err := answerPC.CreateDataChannel(expectedLabel, options)
		assert.NoError(t, err)

		answerPC.OnDataChannel(func(d *DataChannel) {
			// Ignore our default channel, exists to force ICE candidates. See signalPair for more info
			if d.Label() == "initial_data_channel" {
				return
			}

			t.Fatal("OnDataChannel must not be fired when negotiated == true")
		})
		offerPC.OnDataChannel(func(d *DataChannel) {
			t.Fatal("OnDataChannel must not be fired when negotiated == true")
		})

		seenAnswerMessage := &atomicBool{}
		seenOfferMessage := &atomicBool{}

		answerDatachannel.OnMessage(func(msg DataChannelMessage) {
			if msg.IsString && string(msg.Data) == expectedMessage {
				seenAnswerMessage.set(true)
			}
		})

		offerDatachannel.OnMessage(func(msg DataChannelMessage) {
			if msg.IsString && string(msg.Data) == expectedMessage {
				seenOfferMessage.set(true)
			}
		})

		go func() {
			for {
				if seenAnswerMessage.get() && seenOfferMessage.get() {
					break
				}

				if offerDatachannel.ReadyState() == DataChannelStateOpen {
					assert.NoError(t, offerDatachannel.SendText(expectedMessage))
				}
				if answerDatachannel.ReadyState() == DataChannelStateOpen {
					assert.NoError(t, answerDatachannel.SendText(expectedMessage))
				}

				time.Sleep(500 * time.Millisecond)
			}

			done <- true
		}()

		closeReliabilityParamTest(t, offerPC, answerPC, done)
	})
}
