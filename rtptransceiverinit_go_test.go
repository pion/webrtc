// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js
// +build !js

package webrtc

import (
	"context"
	"testing"
	"time"

	"github.com/pion/transport/v3/test"
	"github.com/stretchr/testify/assert"
)

func Test_RTPTransceiverInit_SSRC(t *testing.T) {
	lim := test.TimeOut(time.Second * 30) //nolint
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	track, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: MimeTypeOpus}, "a", "b")
	assert.NoError(t, err)

	t.Run("SSRC of 0 is ignored", func(t *testing.T) {
		offerer, answerer, err := newPair()
		assert.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())
		answerer.OnTrack(func(track *TrackRemote, _ *RTPReceiver) {
			assert.NotEqual(t, 0, track.SSRC())
			cancel()
		})

		_, err = offerer.AddTransceiverFromTrack(track, RTPTransceiverInit{
			Direction: RTPTransceiverDirectionSendonly,
			SendEncodings: []RTPEncodingParameters{
				{
					RTPCodingParameters: RTPCodingParameters{
						SSRC: 0,
					},
				},
			},
		})
		assert.NoError(t, err)
		assert.NoError(t, signalPair(offerer, answerer))
		sendVideoUntilDone(ctx.Done(), t, []*TrackLocalStaticSample{track})
		closePairNow(t, offerer, answerer)
	})

	t.Run("SSRC of 5000", func(t *testing.T) {
		offerer, answerer, err := newPair()
		assert.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())
		answerer.OnTrack(func(track *TrackRemote, _ *RTPReceiver) {
			assert.NotEqual(t, 5000, track.SSRC())
			cancel()
		})

		_, err = offerer.AddTransceiverFromTrack(track, RTPTransceiverInit{
			Direction: RTPTransceiverDirectionSendonly,
			SendEncodings: []RTPEncodingParameters{
				{
					RTPCodingParameters: RTPCodingParameters{
						SSRC: 5000,
					},
				},
			},
		})
		assert.NoError(t, err)
		assert.NoError(t, signalPair(offerer, answerer))
		sendVideoUntilDone(ctx.Done(), t, []*TrackLocalStaticSample{track})
		closePairNow(t, offerer, answerer)
	})
}
