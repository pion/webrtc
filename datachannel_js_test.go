// +build js

package webrtc

import (
	"testing"

	"github.com/pions/transport/test"
	"github.com/stretchr/testify/assert"
)

// TODO(albrow): This test can be combined into a single test for both Go and
// the WASM binding after the Go API has been updated to make Ordered a method
// instead of a struct field.
func TestDataChannel_Send(t *testing.T) {
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
		if d.Label() != "data" {
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

	dc, err := offerPC.CreateDataChannel("data", nil)
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
		t.Fatalf("Failed to signal our PC pair for testing")
	}

	closePair(t, offerPC, answerPC, done)
}

// TODO(albrow): This test can be combined into a single test for both Go and
// the WASM binding after the Go API has been updated to make Ordered and
// MaxPacketLifeTime methods instead of struct fields.
func TestDataChannelParamters(t *testing.T) {
	report := test.CheckRoutines(t)
	defer report()

	t.Run("MaxPacketLifeTime exchange", func(t *testing.T) {
		// Note(albrow): See https://github.com/node-webrtc/node-webrtc/issues/492.
		// There is a bug in the npm wrtc package which causes this test to fail.
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
		// 	if d.Label() != "data" {
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
			if d.Label() != "data" {
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
