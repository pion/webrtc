// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js
// +build !js

package webrtc

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/pion/sdp/v3"
	"github.com/pion/webrtc/v4/pkg/media"
	"github.com/stretchr/testify/assert"
)

func TestSetRTPParameters(t *testing.T) {
	sender, receiver, wan := createVNetPair(t, nil)

	outgoingTrack, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: MimeTypeVP8}, "video", "pion")
	assert.NoError(t, err)

	_, err = sender.AddTrack(outgoingTrack)
	assert.NoError(t, err)

	// Those parameters wouldn't make sense in a real application,
	// but for the sake of the test we just need different values.
	params := RTPParameters{
		Codecs: []RTPCodecParameters{
			{
				RTPCodecCapability: RTPCodecCapability{
					MimeTypeOpus, 48000, 2,
					"minptime=10;useinbandfec=1",
					[]RTCPFeedback{{"nack", ""}},
				},
				PayloadType: 111,
			},
		},
		HeaderExtensions: []RTPHeaderExtensionParameter{
			{URI: sdp.SDESMidURI},
			{URI: sdp.SDESRTPStreamIDURI},
			{URI: sdp.SDESRepairRTPStreamIDURI},
		},
	}

	seenPacket, seenPacketCancel := context.WithCancel(context.Background())
	receiver.OnTrack(func(_ *TrackRemote, r *RTPReceiver) {
		r.SetRTPParameters(params)

		incomingTrackCodecs := r.Track().Codec()

		assert.EqualValues(t, params.HeaderExtensions, r.Track().params.HeaderExtensions)

		assert.EqualValues(t, params.Codecs[0].MimeType, incomingTrackCodecs.MimeType)
		assert.EqualValues(t, params.Codecs[0].ClockRate, incomingTrackCodecs.ClockRate)
		assert.EqualValues(t, params.Codecs[0].Channels, incomingTrackCodecs.Channels)
		assert.EqualValues(t, params.Codecs[0].SDPFmtpLine, incomingTrackCodecs.SDPFmtpLine)
		assert.EqualValues(t, params.Codecs[0].RTCPFeedback, incomingTrackCodecs.RTCPFeedback)
		assert.EqualValues(t, params.Codecs[0].PayloadType, incomingTrackCodecs.PayloadType)

		seenPacketCancel()
	})

	peerConnectionsConnected := untilConnectionState(PeerConnectionStateConnected, sender, receiver)

	assert.NoError(t, signalPair(sender, receiver))

	peerConnectionsConnected.Wait()
	assert.NoError(t, outgoingTrack.WriteSample(media.Sample{Data: []byte{0xAA}, Duration: time.Second}))

	<-seenPacket.Done()
	assert.NoError(t, wan.Stop())
	closePairNow(t, sender, receiver)
}

func TestReceiveError(t *testing.T) {
	api := NewAPI()

	dtlsTransport, err := api.NewDTLSTransport(nil, nil)
	assert.NoError(t, err)

	rtpReceiver, err := api.NewRTPReceiver(RTPCodecTypeVideo, dtlsTransport)
	assert.NoError(t, err)

	rtpParameters := RTPReceiveParameters{
		Encodings: []RTPDecodingParameters{
			{
				RTPCodingParameters: RTPCodingParameters{
					SSRC: 1000,
				},
			},
		},
	}

	assert.Error(t, rtpReceiver.Receive(rtpParameters))

	chanErrs := make(chan error)
	go func() {
		_, _, chanErr := rtpReceiver.Read(nil)
		chanErrs <- chanErr

		_, _, chanErr = rtpReceiver.Track().ReadRTP()
		chanErrs <- chanErr
	}()

	assert.NoError(t, rtpReceiver.Stop())
	assert.Error(t, io.ErrClosedPipe, <-chanErrs)
	assert.Error(t, io.ErrClosedPipe, <-chanErrs)
}
