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

func closePair(t *testing.T, pc1, pc2 io.Closer, done chan bool) {
	var err error
	select {
	case <-time.After(10 * time.Second):
		t.Fatalf("closePair timed out waiting for done signal")
	case <-done:
		err = pc1.Close()
		if err != nil {
			t.Fatalf("Failed to close offer PC")
		}
		err = pc2.Close()
		if err != nil {
			t.Fatalf("Failed to close answer PC")
		}
	}
}

func setUpReliabilityParamTest(t *testing.T, options *DataChannelInit) (*PeerConnection, *PeerConnection, *DataChannel, chan bool) {
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
				done <- true
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

func TestDataChannelParameters(t *testing.T) {
	report := test.CheckRoutines(t)
	defer report()

	t.Run("MaxPacketLifeTime exchange", func(t *testing.T) {
		// Note(albrow): See https://github.com/node-webrtc/node-webrtc/issues/492.
		// There is a bug in the npm wrtc package which causes this test to fail.
		// TODO(albrow): Uncomment this once issue is resolved.
		t.Skip("Skipping because of upstream issue")

		// var ordered = true
		// var maxPacketLifeTime uint16 = 3
		// options := &DataChannelInit{
		// 	Ordered:           &ordered,
		// 	MaxPacketLifeTime: &maxPacketLifeTime,
		// }

		// offerPC, answerPC, dc, done := setUpReliabilityParamTest(t, options)

		// // Check if parameters are correctly set
		// assert.True(t, dc.Ordered(), "Ordered should be set to true")
		// if assert.NotNil(t, dc.MaxPacketLifeTime(), "should not be nil") {
		// 	assert.Equal(t, maxPacketLifeTime, *dc.MaxPacketLifeTime(), "should match")
		// }

		// answerPC.OnDataChannel(func(d *DataChannel) {
		// 	if d.Label() != expectedLabel {
		// 		return
		// 	}
		// 	// Check if parameters are correctly set
		// 	assert.True(t, d.Ordered(), "Ordered should be set to true")
		// 	if assert.NotNil(t, d.MaxPacketLifeTime(), "should not be nil") {
		// 		assert.Equal(t, maxPacketLifeTime, *d.MaxPacketLifeTime(), "should match")
		// 	}
		// 	done <- true
		// })

		// closeReliabilityParamTest(t, offerPC, answerPC, done)
	})

	t.Run("MaxRetransmits exchange", func(t *testing.T) {
		var ordered = false
		var maxRetransmits uint16 = 3000
		options := &DataChannelInit{
			Ordered:        &ordered,
			MaxRetransmits: &maxRetransmits,
		}

		offerPC, answerPC, dc, done := setUpReliabilityParamTest(t, options)

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
}
