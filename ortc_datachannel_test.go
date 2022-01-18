//go:build !js
// +build !js

package webrtc

import (
	"io"
	"testing"
	"time"

	"github.com/pion/transport/test"
	"github.com/stretchr/testify/assert"
)

func TestDataChannel_ORTCE2E(t *testing.T) {
	lim := test.TimeOut(time.Second * 20)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	stackA, stackB, err := newORTCPair()
	assert.NoError(t, err)

	awaitSetup := make(chan struct{})
	awaitString := make(chan struct{})
	awaitBinary := make(chan struct{})
	stackB.sctp.OnDataChannel(func(d *DataChannel) {
		close(awaitSetup)

		d.OnMessage(func(msg DataChannelMessage) {
			if msg.IsString {
				close(awaitString)
			} else {
				close(awaitBinary)
			}
		})
	})

	assert.NoError(t, signalORTCPair(stackA, stackB))

	var id uint16 = 1
	dcParams := &DataChannelParameters{
		Label: "Foo",
		ID:    &id,
	}
	channelA, err := stackA.api.NewDataChannel(stackA.sctp, dcParams)
	assert.NoError(t, err)

	<-awaitSetup

	assert.NoError(t, channelA.SendText("ABC"))
	assert.NoError(t, channelA.Send([]byte("ABC")))

	<-awaitString
	<-awaitBinary

	assert.NoError(t, stackA.close())
	assert.NoError(t, stackB.close())

	// attempt to send when channel is closed
	assert.ErrorIs(t, channelA.Send([]byte("ABC")), io.ErrClosedPipe)
	assert.ErrorIs(t, channelA.SendText("test"), io.ErrClosedPipe)
	assert.ErrorIs(t, channelA.ensureOpen(), io.ErrClosedPipe)
}
