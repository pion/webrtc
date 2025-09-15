// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js
// +build !js

package webrtc

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pion/interceptor"
	"github.com/pion/rtp"
	"github.com/pion/transport/v3/test"
	"github.com/pion/webrtc/v4/pkg/media"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// If a remote doesn't support a Codec used by a `TrackLocalStatic`
// an error should be returned to the user.
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

		noCodecPC, err := NewAPI(WithMediaEngine(&MediaEngine{})).NewPeerConnection(Configuration{})
		assert.NoError(t, err)

		_, err = pc.AddTrack(track)
		assert.NoError(t, err)

		assert.ErrorIs(t, signalPair(pc, noCodecPC), ErrUnsupportedCodec)

		closePairNow(t, noCodecPC, pc)
	})

	t.Run("Answerer", func(t *testing.T) {
		pc, err := NewPeerConnection(Configuration{})
		assert.NoError(t, err)

		mediaEngine := &MediaEngine{}
		assert.NoError(t, mediaEngine.RegisterCodec(RTPCodecParameters{
			RTPCodecCapability: RTPCodecCapability{
				MimeType: "video/VP9", ClockRate: 90000, Channels: 0, SDPFmtpLine: "", RTCPFeedback: nil,
			},
			PayloadType: 96,
		}, RTPCodecTypeVideo))

		vp9OnlyPC, err := NewAPI(WithMediaEngine(mediaEngine)).NewPeerConnection(Configuration{})
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

		invalidCodecTrack, err := NewTrackLocalStaticSample(
			RTPCodecCapability{MimeType: "video/invalid-codec"}, "video", "pion",
		)
		assert.NoError(t, err)

		_, err = offerer.AddTrack(invalidCodecTrack)
		assert.NoError(t, err)

		assert.True(t, errors.Is(signalPair(offerer, answerer), ErrUnsupportedCodec))
		closePairNow(t, offerer, answerer)
	})
}

// Assert that Bind/Unbind happens when expected.
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
		RTPCodecCapability: RTPCodecCapability{
			MimeType:     "video/VP8",
			ClockRate:    90000,
			Channels:     0,
			SDPFmtpLine:  "",
			RTCPFeedback: nil,
		},
		PayloadType: 100,
	}, RTPCodecTypeVideo))

	mediaEngineTwo := &MediaEngine{}
	assert.NoError(t, mediaEngineTwo.RegisterCodec(RTPCodecParameters{
		RTPCodecCapability: RTPCodecCapability{
			MimeType:     "video/VP8",
			ClockRate:    90000,
			Channels:     0,
			SDPFmtpLine:  "",
			RTCPFeedback: nil,
		},
		PayloadType: 200,
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

		onTrackFiredFunc()
	})

	assert.NoError(t, signalPair(offerer, answerer))

	sendVideoUntilDone(t, onTrackFired.Done(), []*TrackLocalStaticSample{track})

	closePairNow(t, offerer, answerer)
}

// Assert that writing to a Track doesn't modify the input
// Even though we can pass a pointer we shouldn't modify the incoming value.
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
// does not block.
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

type TestPacketizer struct {
	rtp.Packetizer
	checked [3]bool
}

func (p *TestPacketizer) GeneratePadding(samples uint32) []*rtp.Packet {
	packets := p.Packetizer.GeneratePadding(samples)
	for _, packet := range packets {
		// Reset padding to ensure we control it
		packet.Header.PaddingSize = 0
		packet.PaddingSize = 0
		packet.Payload = nil

		p.checked[packet.SequenceNumber%3] = true
		switch packet.SequenceNumber % 3 {
		case 0:
			// Recommended way to add padding
			packet.Header.PaddingSize = 255
		case 1:
			// This was used as a workaround so has to be supported too
			packet.Payload = make([]byte, 255)
			packet.Payload[254] = 255
		case 2:
			// This field is deprecated but still used by some clients
			packet.PaddingSize = 255
		}
	}

	return packets
}

func Test_TrackLocalStatic_Padding(t *testing.T) {
	mediaEngineOne := &MediaEngine{}
	assert.NoError(t, mediaEngineOne.RegisterCodec(RTPCodecParameters{
		RTPCodecCapability: RTPCodecCapability{
			MimeType:     "video/VP8",
			ClockRate:    90000,
			Channels:     0,
			SDPFmtpLine:  "",
			RTCPFeedback: nil,
		},
		PayloadType: 100,
	}, RTPCodecTypeVideo))

	mediaEngineTwo := &MediaEngine{}
	assert.NoError(t, mediaEngineTwo.RegisterCodec(RTPCodecParameters{
		RTPCodecCapability: RTPCodecCapability{
			MimeType:     "video/VP8",
			ClockRate:    90000,
			Channels:     0,
			SDPFmtpLine:  "",
			RTCPFeedback: nil,
		},
		PayloadType: 200,
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
			assert.Equal(t, p.Header.PaddingSize, byte(255))
		}

		onTrackFiredFunc()
	})

	assert.NoError(t, signalPair(offerer, answerer))

	exit := false

	// Use a custom packetizer that generates packets with padding in a few different ways
	packetizer := &TestPacketizer{Packetizer: track.packetizer}
	track.packetizer = packetizer

	for !exit {
		select {
		case <-time.After(1 * time.Millisecond):
			assert.NoError(t, track.GeneratePadding(1))
		case <-onTrackFired.Done():
			exit = true
		}
	}

	closePairNow(t, offerer, answerer)

	assert.Equal(t, [3]bool{true, true, true}, packetizer.checked)
}

func Test_TrackLocalStatic_RTX(t *testing.T) {
	defer test.TimeOut(time.Second * 30).Stop()
	defer test.CheckRoutines(t)()

	offerer, answerer, err := newPair()
	assert.NoError(t, err)

	track, err := NewTrackLocalStaticRTP(RTPCodecCapability{MimeType: MimeTypeVP8}, "video", "pion")
	assert.NoError(t, err)

	_, err = offerer.AddTrack(track)
	assert.NoError(t, err)

	assert.NoError(t, signalPair(offerer, answerer))

	track.mu.Lock()
	assert.NotZero(t, track.bindings[0].ssrcRTX)
	assert.NotZero(t, track.bindings[0].payloadTypeRTX)
	track.mu.Unlock()

	closePairNow(t, offerer, answerer)
}

type customCodecPayloader struct {
	invokeCount atomic.Int32
}

func (c *customCodecPayloader) Payload(_ uint16, payload []byte) [][]byte {
	c.invokeCount.Add(1)

	return [][]byte{payload}
}

func Test_TrackLocalStatic_Payloader(t *testing.T) {
	const mimeTypeCustomCodec = "video/custom-codec"

	mediaEngine := &MediaEngine{}
	assert.NoError(t, mediaEngine.RegisterCodec(RTPCodecParameters{
		RTPCodecCapability: RTPCodecCapability{
			MimeType:     mimeTypeCustomCodec,
			ClockRate:    90000,
			Channels:     0,
			SDPFmtpLine:  "",
			RTCPFeedback: nil,
		},
		PayloadType: 96,
	}, RTPCodecTypeVideo))

	offerer, err := NewAPI(WithMediaEngine(mediaEngine)).NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	answerer, err := NewAPI(WithMediaEngine(mediaEngine)).NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	customPayloader := &customCodecPayloader{}
	track, err := NewTrackLocalStaticSample(
		RTPCodecCapability{MimeType: mimeTypeCustomCodec},
		"video",
		"pion",
		WithPayloader(func(c RTPCodecCapability) (rtp.Payloader, error) {
			require.Equal(t, c.MimeType, mimeTypeCustomCodec)

			return customPayloader, nil
		}),
	)
	assert.NoError(t, err)

	_, err = offerer.AddTrack(track)
	assert.NoError(t, err)

	assert.NoError(t, signalPair(offerer, answerer))

	onTrackFired, onTrackFiredFunc := context.WithCancel(context.Background())
	answerer.OnTrack(func(*TrackRemote, *RTPReceiver) {
		onTrackFiredFunc()
	})

	sendVideoUntilDone(t, onTrackFired.Done(), []*TrackLocalStaticSample{track})

	closePairNow(t, offerer, answerer)
}

func Test_TrackLocalStatic_Timestamp(t *testing.T) {
	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	initialTimestamp := uint32(12345)
	track, err := NewTrackLocalStaticSample(
		RTPCodecCapability{MimeType: MimeTypeVP8},
		"video",
		"pion",
		WithRTPTimestamp(initialTimestamp),
	)
	assert.NoError(t, err)

	pcOffer, pcAnswer, err := newPair()
	assert.NoError(t, err)

	_, err = pcOffer.AddTrack(track)
	assert.NoError(t, err)

	onTrackFired, onTrackFiredFunc := context.WithCancel(context.Background())
	pcAnswer.OnTrack(func(trackRemote *TrackRemote, _ *RTPReceiver) {
		pkt, _, err := trackRemote.ReadRTP()
		assert.NoError(t, err)
		assert.GreaterOrEqual(t, pkt.Timestamp, initialTimestamp)
		// not accurate, but some grace period for slow CI test runners.
		assert.LessOrEqual(t, pkt.Timestamp, initialTimestamp+100000)

		onTrackFiredFunc()
	})

	assert.NoError(t, signalPair(pcOffer, pcAnswer))

	sendVideoUntilDone(t, onTrackFired.Done(), []*TrackLocalStaticSample{track})

	<-onTrackFired.Done()
	closePairNow(t, pcOffer, pcAnswer)
}

type dummyWriter struct{}

func (dummyWriter) WriteRTP(_ *rtp.Header, _ []byte) (int, error) { return 0, nil }
func (dummyWriter) Write(_ []byte) (int, error)                   { return 0, nil }

type dummyTrackLocalContext struct {
	id string
}

func (d dummyTrackLocalContext) ID() string                                      { return d.id }
func (d dummyTrackLocalContext) SSRC() SSRC                                      { return 0 }
func (d dummyTrackLocalContext) SSRCRetransmission() SSRC                        { return 0 }
func (d dummyTrackLocalContext) SSRCForwardErrorCorrection() SSRC                { return 0 }
func (d dummyTrackLocalContext) WriteStream() TrackLocalWriter                   { return dummyWriter{} }
func (d dummyTrackLocalContext) HeaderExtensions() []RTPHeaderExtensionParameter { return nil }
func (d dummyTrackLocalContext) RTCPReader() interceptor.RTCPReader              { return nil }
func (d dummyTrackLocalContext) CodecParameters() []RTPCodecParameters {
	return []RTPCodecParameters{{
		RTPCodecCapability: RTPCodecCapability{
			MimeType:  MimeTypeVP8,
			ClockRate: 90000,
		},
		PayloadType: 96,
	}}
}

func Test_TrackLocalStaticRTP_Unbind_ErrUnbindFailed(t *testing.T) {
	track, err := NewTrackLocalStaticRTP(
		RTPCodecCapability{MimeType: MimeTypeVP8},
		"video",
		"pion",
	)
	require.NoError(t, err)

	ctx := dummyTrackLocalContext{id: "nonexistent-id"}

	err = track.Unbind(ctx)
	require.ErrorIs(t, err, ErrUnbindFailed)
}

func Test_TrackLocalStaticRTP_Kind_Default(t *testing.T) {
	track, err := NewTrackLocalStaticRTP(
		RTPCodecCapability{MimeType: "application/unknown"},
		"id",
		"stream",
	)
	require.NoError(t, err)

	require.Equal(t, RTPCodecType(0), track.Kind())
}

func Test_TrackLocalStaticRTP_Codec_ReturnsConfiguredCodec(t *testing.T) {
	testCapability := RTPCodecCapability{
		MimeType:     MimeTypeVP8,
		ClockRate:    90000,
		Channels:     0,
		SDPFmtpLine:  "profile-id=0",
		RTCPFeedback: []RTCPFeedback{{Type: "nack"}, {Type: "ccm", Parameter: "fir"}},
	}

	track, err := NewTrackLocalStaticRTP(testCapability, "video", "pion")
	require.NoError(t, err)

	got := track.Codec()
	require.Equal(t, testCapability, got)
}

var errWriteBoom = errors.New("fake write failure")

type errWriter struct{}

func (errWriter) WriteRTP(_ *rtp.Header, _ []byte) (int, error) { return 0, errWriteBoom }
func (errWriter) Write(_ []byte) (int, error)                   { return 0, nil }

func Test_TrackLocalStaticRTP_writeRTP_ReturnsError(t *testing.T) {
	track, err := NewTrackLocalStaticRTP(
		RTPCodecCapability{MimeType: MimeTypeVP8},
		"id",
		"stream",
	)
	require.NoError(t, err)

	track.mu.Lock()
	track.bindings = []trackBinding{{
		id:          "b1",
		ssrc:        0x1234,
		payloadType: 96,
		writeStream: errWriter{},
	}}
	track.mu.Unlock()

	pkt := &rtp.Packet{Payload: []byte{0x01, 0x02, 0x03}}

	err = track.writeRTP(pkt)
	require.Error(t, err)
	require.Contains(t, err.Error(), errWriteBoom.Error())
}

func Test_TrackLocalStaticRTP_Write_UnmarshalError(t *testing.T) {
	track, err := NewTrackLocalStaticRTP(
		RTPCodecCapability{MimeType: MimeTypeVP8},
		"id",
		"stream",
	)
	require.NoError(t, err)

	n, werr := track.Write([]byte{0x80}) // < 12-byte RTP header
	require.Error(t, werr)
	require.Equal(t, 0, n)
}

func Test_TrackLocalStaticSample_Codec_ReturnsConfiguredCodec(t *testing.T) {
	testCapability := RTPCodecCapability{
		MimeType:     MimeTypeVP8,
		ClockRate:    90000,
		Channels:     0,
		SDPFmtpLine:  "profile-id=0",
		RTCPFeedback: []RTCPFeedback{{Type: "nack"}, {Type: "ccm", Parameter: "fir"}},
	}

	sample, err := NewTrackLocalStaticSample(testCapability, "video", "pion")
	require.NoError(t, err)

	got := sample.Codec()
	require.Equal(t, testCapability, got)
}

var errPayloaderBoom = errors.New("payloader boom")

func Test_TrackLocalStaticSample_Bind_PayloaderError(t *testing.T) {
	sample, err := NewTrackLocalStaticSample(
		RTPCodecCapability{MimeType: MimeTypeVP8, ClockRate: 90000},
		"video",
		"pion",
	)
	require.NoError(t, err)

	sample.rtpTrack.mu.Lock()
	sample.rtpTrack.payloader = func(_ RTPCodecCapability) (rtp.Payloader, error) {
		return nil, errPayloaderBoom
	}
	sample.rtpTrack.mu.Unlock()

	_, bindErr := sample.Bind(dummyTrackLocalContext{id: "ctx-1"})
	require.ErrorIs(t, bindErr, errPayloaderBoom)

	sample.rtpTrack.mu.RLock()
	defer sample.rtpTrack.mu.RUnlock()
	require.Nil(t, sample.packetizer)
}

type fakePacketizer struct {
	skipCalls  int
	lastSample uint32

	packetizeCalls int
}

func (f *fakePacketizer) SkipSamples(n uint32) { f.skipCalls++; f.lastSample = n }
func (f *fakePacketizer) GeneratePadding(samples uint32) []*rtp.Packet {
	f.packetizeCalls++
	f.lastSample = samples

	return []*rtp.Packet{{}, {}}
}
func (f *fakePacketizer) EnableAbsSendTime(value int) {}
func (f *fakePacketizer) Packetize(_ []byte, _ uint32) []*rtp.Packet {
	f.packetizeCalls++

	return []*rtp.Packet{
		{Payload: []byte{0x01}},
		{Payload: []byte{0x02}},
	}
}

func Test_TrackLocalStaticSample_WriteSample_AppendErrors(t *testing.T) {
	testSample, err := NewTrackLocalStaticSample(
		RTPCodecCapability{MimeType: MimeTypeVP8},
		"video",
		"pion",
	)
	require.NoError(t, err)

	testSample.rtpTrack.mu.Lock()
	testSample.rtpTrack.bindings = []trackBinding{{
		id:          "b1",
		ssrc:        0x1234,
		payloadType: 96,
		writeStream: errWriter{},
	}}
	testSample.rtpTrack.mu.Unlock()

	fp := &fakePacketizer{}
	testSample.rtpTrack.mu.Lock()
	testSample.packetizer = fp
	testSample.sequencer = rtp.NewRandomSequencer()
	testSample.clockRate = 48000
	testSample.rtpTrack.mu.Unlock()

	in := media.Sample{
		Data:               []byte("hi"),
		Duration:           20 * time.Millisecond,
		PrevDroppedPackets: 3,
	}

	err = testSample.WriteSample(in)

	require.Error(t, err)
	require.Contains(t, err.Error(), errWriteBoom.Error())

	require.Equal(t, 1, fp.skipCalls)
	require.Equal(t, uint32(960*3), fp.lastSample)

	require.Equal(t, 1, fp.packetizeCalls)
}

func Test_TrackLocalStaticSample_GeneratePadding_PacketizerNil_ReturnsNil(t *testing.T) {
	s, err := NewTrackLocalStaticSample(
		RTPCodecCapability{MimeType: MimeTypeVP8},
		"video",
		"pion",
	)
	require.NoError(t, err)

	err = s.GeneratePadding(10)
	require.NoError(t, err)
}

func Test_TrackLocalStaticSample_GeneratePadding_AppendsAndReturnsError(t *testing.T) {
	testSample, err := NewTrackLocalStaticSample(
		RTPCodecCapability{MimeType: MimeTypeVP8},
		"video",
		"pion",
	)
	require.NoError(t, err)

	testSample.rtpTrack.mu.Lock()
	testSample.rtpTrack.bindings = []trackBinding{{
		id:          "b1",
		ssrc:        0x1234,
		payloadType: 96,
		writeStream: errWriter{},
	}}

	fp := &fakePacketizer{}
	testSample.packetizer = fp
	testSample.rtpTrack.mu.Unlock()

	err = testSample.GeneratePadding(7)
	require.Error(t, err)
	require.Contains(t, err.Error(), errWriteBoom.Error())

	require.Equal(t, 1, fp.packetizeCalls)
	require.Equal(t, uint32(7), fp.lastSample)
}

func Test_TrackRemote_Msid(t *testing.T) {
	t.Run("Populated", func(t *testing.T) {
		tr := newTrackRemote(RTPCodecTypeVideo, 1234, 0, "", nil)

		tr.mu.Lock()
		tr.id = "video"
		tr.streamID = "desktop"
		tr.mu.Unlock()

		require.Equal(t, "desktop video", tr.Msid())
	})

	t.Run("Empty", func(t *testing.T) {
		tr := newTrackRemote(RTPCodecTypeAudio, 0, 0, "", nil)
		require.Equal(t, " ", tr.Msid())
	})
}

func Test_TrackRemote_checkAndUpdateTrack_ShortPacket(t *testing.T) {
	tr := newTrackRemote(RTPCodecTypeVideo, 0, 0, "", &RTPReceiver{
		api:  &API{mediaEngine: &MediaEngine{}},
		kind: RTPCodecTypeVideo,
	})

	err := tr.checkAndUpdateTrack([]byte{0x80})
	require.ErrorIs(t, err, errRTPTooShort)
}

func Test_TrackRemote_checkAndUpdateTrack_CodecNotFound(t *testing.T) {
	me := &MediaEngine{} // intentionally empty: no codecs registered.
	api := &API{mediaEngine: me}
	recv := &RTPReceiver{api: api, kind: RTPCodecTypeVideo}
	tr := newTrackRemote(RTPCodecTypeVideo, 0, 0, "", recv)

	// minimal RTP header-sized buffer with a payload type byte.
	b := []byte{0x80, 96}

	err := tr.checkAndUpdateTrack(b)
	require.ErrorIs(t, err, ErrCodecNotFound)
}

func Test_TrackRemote_ReadRTP_UnmarshalError(t *testing.T) {
	me := &MediaEngine{}
	require.NoError(t, me.RegisterCodec(RTPCodecParameters{
		RTPCodecCapability: RTPCodecCapability{
			MimeType:  MimeTypeVP8,
			ClockRate: 90000,
		},
		PayloadType: 96,
	}, RTPCodecTypeVideo))

	api := &API{
		mediaEngine:   me,
		settingEngine: &SettingEngine{},
	}

	recv := &RTPReceiver{
		api:  api,
		kind: RTPCodecTypeVideo,
	}

	tr := newTrackRemote(RTPCodecTypeVideo, 0, 0, "", recv)

	tr.mu.Lock()
	tr.peeked = []byte{0x80, 96}
	tr.peekedAttributes = nil
	tr.mu.Unlock()

	pkt, attrs, err := tr.ReadRTP()
	require.Error(t, err, "expected Unmarshal to fail on too-short RTP data")
	require.Nil(t, pkt)
	require.Nil(t, attrs)
}

func TestBaseTrackLocalContext_HeaderExtensions_ReturnsParams(t *testing.T) {
	hdrs := []RTPHeaderExtensionParameter{
		{URI: "urn:ietf:params:rtp-hdrext:sdes:mid", ID: 1},
		{URI: "urn:ietf:params:rtp-hdrext:sdes:rtp-stream-id", ID: 2},
	}

	ctx := baseTrackLocalContext{
		params: RTPParameters{
			HeaderExtensions: hdrs,
		},
	}

	got := ctx.HeaderExtensions()
	require.Equal(t, hdrs, got)

	got[0].URI = "changed"
	assert.Equal(t, "changed", ctx.params.HeaderExtensions[0].URI)
}

func TestBaseTrackLocalContext_HeaderExtensions_NilWhenUnset(t *testing.T) {
	var ctx baseTrackLocalContext
	assert.Nil(t, ctx.HeaderExtensions())
}
