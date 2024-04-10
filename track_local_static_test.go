// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js
// +build !js

package webrtc

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/pion/rtp"
	"github.com/pion/transport/v2/test"
	"github.com/stretchr/testify/assert"
)

// If a remote doesn't support a Codec used by a `TrackLocalStatic`
// an error should be returned to the user
func Test_TrackLocalStatic_NoCodecIntersection(t *testing.T) {
	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	track, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: MimeTypeVP8}, "video", "pion")
	assert.NoError(t, err)

	t.Run("Offerer", func(t *testing.T) {
		pc, err := NewPeerConnection(Configuration{})
		assert.NoError(t, err)

		noCodecPC, err := NewAPI().NewPeerConnection(Configuration{})
		assert.NoError(t, err)

		_, err = pc.AddTrack(track)
		assert.NoError(t, err)

		assert.ErrorIs(t, signalPair(pc, noCodecPC), ErrUnsupportedCodec)

		closePairNow(t, noCodecPC, pc)
	})

	t.Run("Answerer", func(t *testing.T) {
		pc, err := NewPeerConnection(Configuration{})
		assert.NoError(t, err)

		m := &MediaEngine{}
		assert.NoError(t, m.RegisterCodec(RTPCodecParameters{
			RTPCodecCapability: RTPCodecCapability{MimeType: "video/VP9", ClockRate: 90000, Channels: 0, SDPFmtpLine: "", RTCPFeedback: nil},
			PayloadType:        96,
		}, RTPCodecTypeVideo))

		vp9OnlyPC, err := NewAPI(WithMediaEngine(m)).NewPeerConnection(Configuration{})
		assert.NoError(t, err)

		_, err = vp9OnlyPC.AddTransceiverFromKind(RTPCodecTypeVideo)
		assert.NoError(t, err)

		_, err = pc.AddTrack(track)
		assert.NoError(t, err)

		assert.True(t, errors.Is(signalPair(vp9OnlyPC, pc), ErrUnsupportedCodec))

		closePairNow(t, vp9OnlyPC, pc)
	})

	t.Run("Local", func(t *testing.T) {
		offerer, answerer, err := newPair()
		assert.NoError(t, err)

		invalidCodecTrack, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: "video/invalid-codec"}, "video", "pion")
		assert.NoError(t, err)

		_, err = offerer.AddTrack(invalidCodecTrack)
		assert.NoError(t, err)

		assert.True(t, errors.Is(signalPair(offerer, answerer), ErrUnsupportedCodec))
		closePairNow(t, offerer, answerer)
	})
}

// Assert that Bind/Unbind happens when expected
func Test_TrackLocalStatic_Closed(t *testing.T) {
	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	pcOffer, pcAnswer, err := newPair()
	assert.NoError(t, err)

	_, err = pcAnswer.AddTransceiverFromKind(RTPCodecTypeVideo)
	assert.NoError(t, err)

	vp8Writer, err := NewTrackLocalStaticRTP(RTPCodecCapability{MimeType: MimeTypeVP8}, "video", "pion")
	assert.NoError(t, err)

	_, err = pcOffer.AddTrack(vp8Writer)
	assert.NoError(t, err)

	assert.Equal(t, len(vp8Writer.bindings), 0, "No binding should exist before signaling")

	assert.NoError(t, signalPair(pcOffer, pcAnswer))

	assert.Equal(t, len(vp8Writer.bindings), 1, "binding should exist after signaling")

	closePairNow(t, pcOffer, pcAnswer)

	assert.Equal(t, len(vp8Writer.bindings), 0, "No binding should exist after close")
}

func Test_TrackLocalStatic_PayloadType(t *testing.T) {
	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	mediaEngineOne := &MediaEngine{}
	assert.NoError(t, mediaEngineOne.RegisterCodec(RTPCodecParameters{
		RTPCodecCapability: RTPCodecCapability{MimeType: "video/VP8", ClockRate: 90000, Channels: 0, SDPFmtpLine: "", RTCPFeedback: nil},
		PayloadType:        100,
	}, RTPCodecTypeVideo))

	mediaEngineTwo := &MediaEngine{}
	assert.NoError(t, mediaEngineTwo.RegisterCodec(RTPCodecParameters{
		RTPCodecCapability: RTPCodecCapability{MimeType: "video/VP8", ClockRate: 90000, Channels: 0, SDPFmtpLine: "", RTCPFeedback: nil},
		PayloadType:        200,
	}, RTPCodecTypeVideo))

	offerer, err := NewAPI(WithMediaEngine(mediaEngineOne)).NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	answerer, err := NewAPI(WithMediaEngine(mediaEngineTwo)).NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	track, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: MimeTypeVP8}, "video", "pion")
	assert.NoError(t, err)

	_, err = offerer.AddTransceiverFromKind(RTPCodecTypeVideo)
	assert.NoError(t, err)

	_, err = answerer.AddTrack(track)
	assert.NoError(t, err)

	onTrackFired, onTrackFiredFunc := context.WithCancel(context.Background())
	offerer.OnTrack(func(track *TrackRemote, r *RTPReceiver) {
		assert.Equal(t, track.PayloadType(), PayloadType(100))
		assert.Equal(t, track.Codec().RTPCodecCapability.MimeType, "video/VP8")

		onTrackFiredFunc()
	})

	assert.NoError(t, signalPair(offerer, answerer))

	sendVideoUntilDone(onTrackFired.Done(), t, []*TrackLocalStaticSample{track})

	closePairNow(t, offerer, answerer)
}

// Assert that writing to a Track doesn't modify the input
// Even though we can pass a pointer we shouldn't modify the incoming value
func Test_TrackLocalStatic_Mutate_Input(t *testing.T) {
	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	pcOffer, pcAnswer, err := newPair()
	assert.NoError(t, err)

	vp8Writer, err := NewTrackLocalStaticRTP(RTPCodecCapability{MimeType: MimeTypeVP8}, "video", "pion")
	assert.NoError(t, err)

	_, err = pcOffer.AddTrack(vp8Writer)
	assert.NoError(t, err)

	assert.NoError(t, signalPair(pcOffer, pcAnswer))

	pkt := &rtp.Packet{Header: rtp.Header{SSRC: 1, PayloadType: 1}}
	assert.NoError(t, vp8Writer.WriteRTP(pkt))

	assert.Equal(t, pkt.Header.SSRC, uint32(1))
	assert.Equal(t, pkt.Header.PayloadType, uint8(1))

	closePairNow(t, pcOffer, pcAnswer)
}

// Assert that writing to a Track that has Binded (but not connected)
// does not block
func Test_TrackLocalStatic_Binding_NonBlocking(t *testing.T) {
	lim := test.TimeOut(time.Second * 5)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	pcOffer, pcAnswer, err := newPair()
	assert.NoError(t, err)

	_, err = pcOffer.AddTransceiverFromKind(RTPCodecTypeVideo)
	assert.NoError(t, err)

	vp8Writer, err := NewTrackLocalStaticRTP(RTPCodecCapability{MimeType: MimeTypeVP8}, "video", "pion")
	assert.NoError(t, err)

	_, err = pcAnswer.AddTrack(vp8Writer)
	assert.NoError(t, err)

	offer, err := pcOffer.CreateOffer(nil)
	assert.NoError(t, err)

	assert.NoError(t, pcAnswer.SetRemoteDescription(offer))

	answer, err := pcAnswer.CreateAnswer(nil)
	assert.NoError(t, err)
	assert.NoError(t, pcAnswer.SetLocalDescription(answer))

	_, err = vp8Writer.Write(make([]byte, 20))
	assert.NoError(t, err)

	closePairNow(t, pcOffer, pcAnswer)
}

func BenchmarkTrackLocalWrite(b *testing.B) {
	offerPC, answerPC, err := newPair()
	defer closePairNow(b, offerPC, answerPC)
	if err != nil {
		b.Fatalf("Failed to create a PC pair for testing")
	}

	track, err := NewTrackLocalStaticRTP(RTPCodecCapability{MimeType: MimeTypeVP8}, "video", "pion")
	assert.NoError(b, err)

	_, err = offerPC.AddTrack(track)
	assert.NoError(b, err)

	_, err = answerPC.AddTransceiverFromKind(RTPCodecTypeVideo)
	assert.NoError(b, err)

	b.SetBytes(1024)

	buf := make([]byte, 1024)
	for i := 0; i < b.N; i++ {
		_, err := track.Write(buf)
		assert.NoError(b, err)
	}
}

func Test_TrackLocalStatic_Padding(t *testing.T) {
	mediaEngineOne := &MediaEngine{}
	assert.NoError(t, mediaEngineOne.RegisterCodec(RTPCodecParameters{
		RTPCodecCapability: RTPCodecCapability{MimeType: "video/VP8", ClockRate: 90000, Channels: 0, SDPFmtpLine: "", RTCPFeedback: nil},
		PayloadType:        100,
	}, RTPCodecTypeVideo))

	mediaEngineTwo := &MediaEngine{}
	assert.NoError(t, mediaEngineTwo.RegisterCodec(RTPCodecParameters{
		RTPCodecCapability: RTPCodecCapability{MimeType: "video/VP8", ClockRate: 90000, Channels: 0, SDPFmtpLine: "", RTCPFeedback: nil},
		PayloadType:        200,
	}, RTPCodecTypeVideo))

	offerer, err := NewAPI(WithMediaEngine(mediaEngineOne)).NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	answerer, err := NewAPI(WithMediaEngine(mediaEngineTwo)).NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	track, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: MimeTypeVP8}, "video", "pion")
	assert.NoError(t, err)

	_, err = offerer.AddTransceiverFromKind(RTPCodecTypeVideo)
	assert.NoError(t, err)

	_, err = answerer.AddTrack(track)
	assert.NoError(t, err)

	onTrackFired, onTrackFiredFunc := context.WithCancel(context.Background())

	offerer.OnTrack(func(track *TrackRemote, _ *RTPReceiver) {
		assert.Equal(t, track.PayloadType(), PayloadType(100))
		assert.Equal(t, track.Codec().RTPCodecCapability.MimeType, "video/VP8")

		for i := 0; i < 20; i++ {
			// Padding payload
			p, _, e := track.ReadRTP()
			assert.NoError(t, e)
			assert.True(t, p.Padding)
			assert.Equal(t, p.PaddingSize, byte(255))
		}

		onTrackFiredFunc()
	})

	assert.NoError(t, signalPair(offerer, answerer))

	exit := false

	for !exit {
		select {
		case <-time.After(1 * time.Millisecond):
			assert.NoError(t, track.GeneratePadding(1))
		case <-onTrackFired.Done():
			exit = true
		}
	}

	closePairNow(t, offerer, answerer)
}
