// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js
// +build !js

package webrtc

import (
	"context"
	"testing"
	"time"

	"github.com/pion/transport/v2/test"
	"github.com/pion/webrtc/v3/pkg/media"
	"github.com/stretchr/testify/assert"
)

func Test_ORTC_Media(t *testing.T) {
	lim := test.TimeOut(time.Second * 20)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	stackA, stackB, err := newORTCPair()
	assert.NoError(t, err)

	assert.NoError(t, stackA.api.mediaEngine.RegisterDefaultCodecs())
	assert.NoError(t, stackB.api.mediaEngine.RegisterDefaultCodecs())

	assert.NoError(t, signalORTCPair(stackA, stackB))

	track, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: MimeTypeVP8}, "video", "pion")
	assert.NoError(t, err)

	rtpSender, err := stackA.api.NewRTPSender(track, stackA.dtls)
	assert.NoError(t, err)
	assert.NoError(t, rtpSender.Send(rtpSender.GetParameters()))

	rtpReceiver, err := stackB.api.NewRTPReceiver(RTPCodecTypeVideo, stackB.dtls)
	assert.NoError(t, err)
	assert.NoError(t, rtpReceiver.Receive(RTPReceiveParameters{Encodings: []RTPDecodingParameters{
		{RTPCodingParameters: rtpSender.GetParameters().Encodings[0].RTPCodingParameters},
	}}))

	seenPacket, seenPacketCancel := context.WithCancel(context.Background())
	go func() {
		track := rtpReceiver.Track()
		_, _, err := track.ReadRTP()
		assert.NoError(t, err)

		seenPacketCancel()
	}()

	func() {
		for range time.Tick(time.Millisecond * 20) {
			select {
			case <-seenPacket.Done():
				return
			default:
				assert.NoError(t, track.WriteSample(media.Sample{Data: []byte{0xAA}, Duration: time.Second}))
			}
		}
	}()

	assert.NoError(t, rtpSender.Stop())
	assert.NoError(t, rtpReceiver.Stop())

	assert.NoError(t, stackA.close())
	assert.NoError(t, stackB.close())
}
