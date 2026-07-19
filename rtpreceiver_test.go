// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js

package webrtc

import (
	"context"
	"io"
	"math"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pion/interceptor"
	mock_interceptor "github.com/pion/interceptor/pkg/mock"
	"github.com/pion/interceptor/pkg/stats"
	"github.com/pion/logging"
	"github.com/pion/rtp"
	"github.com/pion/transport/v4/packetio"
	"github.com/pion/transport/v4/test"
	"github.com/pion/webrtc/v4/pkg/media"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Assert that SetReadDeadline works as expected
// This test uses VNet since we must have zero loss.
func Test_RTPReceiver_SetReadDeadline(t *testing.T) {
	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	sender, receiver, wan := createVNetPair(t, &interceptor.Registry{})

	track, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: MimeTypeVP8}, "video", "pion")
	assert.NoError(t, err)

	_, err = sender.AddTrack(track)
	assert.NoError(t, err)

	seenPacket, seenPacketCancel := context.WithCancel(context.Background())
	receiver.OnTrack(func(trackRemote *TrackRemote, r *RTPReceiver) {
		// Set Deadline for both RTP and RTCP Stream
		assert.NoError(t, r.SetReadDeadline(time.Now().Add(time.Second)))
		assert.NoError(t, trackRemote.SetReadDeadline(time.Now().Add(time.Second)))

		// First call will not error because we cache for probing
		_, _, readErr := trackRemote.ReadRTP()
		assert.NoError(t, readErr)

		_, _, readErr = trackRemote.ReadRTP()
		assert.Error(t, readErr)

		_, _, readErr = r.ReadRTCP()
		assert.Error(t, readErr)

		seenPacketCancel()
	})

	peerConnectionsConnected := untilConnectionState(PeerConnectionStateConnected, sender, receiver)

	assert.NoError(t, signalPair(sender, receiver))

	peerConnectionsConnected.Wait()
	assert.NoError(t, track.WriteSample(media.Sample{Data: []byte{0xAA}, Duration: time.Second}))

	<-seenPacket.Done()
	assert.NoError(t, wan.Stop())
	closePairNow(t, sender, receiver)
}

func TestRTPReceiver_ClosedReceiveForRIDAndRTX(t *testing.T) {
	lim := test.TimeOut(time.Second * 5)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	api := NewAPI()
	dtlsTransport, err := api.NewDTLSTransport(nil, nil)
	require.NoError(t, err)

	receiver, err := api.NewRTPReceiver(RTPCodecTypeVideo, dtlsTransport)
	require.NoError(t, err)

	receiver.configureReceive(RTPReceiveParameters{
		Encodings: []RTPDecodingParameters{
			{
				RTPCodingParameters: RTPCodingParameters{
					RID:  "rid",
					SSRC: 1111,
					RTX: RTPRtxParameters{
						SSRC: 2222,
					},
				},
			},
		},
	})

	require.NoError(t, receiver.Stop())

	params := RTPParameters{
		Codecs: []RTPCodecParameters{
			{
				RTPCodecCapability: RTPCodecCapability{MimeType: MimeTypeVP8},
			},
		},
	}
	ridStreamInfo := &interceptor.StreamInfo{SSRC: 1111}
	rtxStreamInfo := &interceptor.StreamInfo{SSRC: 2222}
	readCalled := make(chan struct{}, 1)
	rtpInterceptor := interceptor.RTPReaderFunc(
		func(_ []byte, a interceptor.Attributes) (int, interceptor.Attributes, error) {
			select {
			case readCalled <- struct{}{}:
			default:
			}

			return 0, a, io.EOF
		},
	)

	for range 50 {
		track, err := receiver.receiveForRid("rid", params, ridStreamInfo, nil, nil, false, nil, nil, nil)
		assert.Nil(t, track)
		assert.ErrorIs(t, err, io.EOF)

		err = receiver.receiveForRtx(SSRC(0), "rid", rtxStreamInfo, nil, rtpInterceptor, false, nil, nil)
		assert.ErrorIs(t, err, io.EOF)
	}

	select {
	case <-readCalled:
		assert.Fail(t, "repair reader invoked after Stop")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestRTPReceiverRepairReaderPolicy(t *testing.T) {
	for _, tt := range []struct {
		name         string
		customBuffer bool
		wrapped      bool
	}{
		{name: "default buffer passthrough"},
		{name: "default buffer wrapped", wrapped: true},
		{name: "custom buffer passthrough", customBuffer: true},
		{name: "custom buffer wrapped", customBuffer: true, wrapped: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			startImmediately := tt.wrapped && !tt.customBuffer
			receiver, track := newRepairReaderPolicyTestReceiver(t, tt.customBuffer, 2222, nil)
			var calls atomic.Int32
			releaseReader := make(chan struct{})
			t.Cleanup(func() { close(releaseReader) })
			repairReader := interceptor.RTPReaderFunc(
				func(_ []byte, a interceptor.Attributes) (int, interceptor.Attributes, error) {
					calls.Add(1)
					<-releaseReader

					return 0, a, io.EOF
				},
			)
			require.NoError(t, receiver.receiveForRtx(
				2222, "", &interceptor.StreamInfo{SSRC: 2222}, nil, repairReader, startImmediately, nil, nil,
			))

			repairReaderStarted := func() bool {
				receiver.mu.RLock()
				defer receiver.mu.RUnlock()

				return receiver.tracks[0].repairReaderStarted
			}
			assert.Equal(t, startImmediately, repairReaderStarted())
			if startImmediately {
				require.Eventually(t, func() bool { return calls.Load() == 1 }, time.Second, time.Millisecond)
			} else {
				assert.Zero(t, calls.Load())
			}

			b := make([]byte, receiveMTU)
			_, err := track.peek(b)
			require.NoError(t, err)
			if !startImmediately {
				assert.False(t, repairReaderStarted())
				assert.Zero(t, calls.Load())
			}

			_, _, err = track.Read(b)
			require.NoError(t, err)
			assert.True(t, repairReaderStarted())
			require.Eventually(t, func() bool { return calls.Load() == 1 }, time.Second, time.Millisecond)

			_, _, err = track.Read(b)
			require.NoError(t, err)
			assert.Never(t, func() bool { return calls.Load() > 1 }, 25*time.Millisecond, time.Millisecond)
		})
	}
}

func TestTrackRemoteReadUsesEagerRepairChannel(t *testing.T) {
	var primaryCalls atomic.Int32
	primaryReader := interceptor.RTPReaderFunc(
		func(b []byte, a interceptor.Attributes) (int, interceptor.Attributes, error) {
			primaryCalls.Add(1)

			return copy(b, []byte{
				0x80, 96, 0, 1, 0, 0, 0, 0, 0, 0, 0x04, 0x57, 0xAA,
			}), a, nil
		},
	)
	receiver, track := newRepairReaderPolicyTestReceiver(t, false, 2222, primaryReader)
	var repairCalls atomic.Int32
	repairReader := interceptor.RTPReaderFunc(
		func(b []byte, a interceptor.Attributes) (int, interceptor.Attributes, error) {
			if repairCalls.Add(1) != 1 {
				return 0, a, io.EOF
			}

			return copy(b, []byte{
				0x80, 97, 0x13, 0x88, 0, 0, 0, 0, 0, 0, 0x08, 0xAE,
				0x04, 0xD2, 0xA1,
			}), a, nil
		},
	)
	require.NoError(t, receiver.receiveForRtx(
		2222, "", &interceptor.StreamInfo{SSRC: 2222}, nil, repairReader, true, nil, nil,
	))
	require.Eventually(t, func() bool {
		receiver.mu.RLock()
		defer receiver.mu.RUnlock()

		return len(receiver.tracks[0].repairStreamChannel) == 1
	}, time.Second, time.Millisecond)

	packet, _, err := track.ReadRTP()
	require.NoError(t, err)
	require.NotNil(t, packet)
	assert.Equal(t, uint16(1234), packet.SequenceNumber)
	assert.Equal(t, []byte{0xA1}, packet.Payload)
	assert.Zero(t, primaryCalls.Load())
}

func TestRTPReceiverLateRepairBindAfterTrackRead(t *testing.T) {
	receiver, track := newRepairReaderPolicyTestReceiver(t, true, 0, nil)
	_, _, err := track.Read(make([]byte, receiveMTU))
	require.NoError(t, err)

	var calls atomic.Int32
	releaseReader := make(chan struct{})
	t.Cleanup(func() { close(releaseReader) })
	repairReader := interceptor.RTPReaderFunc(
		func(_ []byte, a interceptor.Attributes) (int, interceptor.Attributes, error) {
			calls.Add(1)
			<-releaseReader

			return 0, a, io.EOF
		},
	)
	require.NoError(t, receiver.receiveForRtx(
		0, "rid", &interceptor.StreamInfo{SSRC: 2222}, nil, repairReader, false, nil, nil,
	))

	receiver.mu.RLock()
	started := receiver.tracks[0].repairReaderStarted
	receiver.mu.RUnlock()
	assert.True(t, started)
	require.Eventually(t, func() bool { return calls.Load() == 1 }, time.Second, time.Millisecond)
}

func TestRTPReceiverRIDRepairReaderStartsForPrimaryWrapper(t *testing.T) {
	for _, primaryFirst := range []bool{true, false} {
		name := "RTX first"
		if primaryFirst {
			name = "primary first"
		}
		t.Run(name, func(t *testing.T) {
			api := NewAPI()
			receiver, err := api.NewRTPReceiver(RTPCodecTypeVideo, &DTLSTransport{api: api})
			require.NoError(t, err)
			receiver.configureReceive(RTPReceiveParameters{Encodings: []RTPDecodingParameters{{
				RTPCodingParameters: RTPCodingParameters{RID: "rid"},
			}}})
			close(receiver.received)
			t.Cleanup(func() {
				assert.NoError(t, receiver.Stop())
			})

			var repairCalls atomic.Int32
			releaseReader := make(chan struct{})
			t.Cleanup(func() { close(releaseReader) })
			repairReader := interceptor.RTPReaderFunc(
				func(_ []byte, a interceptor.Attributes) (int, interceptor.Attributes, error) {
					repairCalls.Add(1)
					<-releaseReader

					return 0, a, io.EOF
				},
			)
			params := RTPParameters{Codecs: []RTPCodecParameters{{
				RTPCodecCapability: RTPCodecCapability{MimeType: MimeTypeVP8},
				PayloadType:        96,
			}}}
			bindPrimary := func() error {
				_, bindErr := receiver.receiveForRid(
					"rid",
					params,
					&interceptor.StreamInfo{SSRC: 1111},
					nil,
					interceptor.RTPReaderFunc(nil),
					true,
					nil,
					nil,
					nil,
				)

				return bindErr
			}
			bindRTX := func() error {
				return receiver.receiveForRtx(
					0,
					"rid",
					&interceptor.StreamInfo{SSRC: 2222},
					nil,
					repairReader,
					false,
					nil,
					nil,
				)
			}

			first, second := bindRTX, bindPrimary
			if primaryFirst {
				first, second = bindPrimary, bindRTX
			}
			require.NoError(t, first())
			receiver.mu.RLock()
			startedAfterFirstBind := receiver.tracks[0].repairReaderStarted
			receiver.mu.RUnlock()
			assert.False(t, startedAfterFirstBind)
			assert.Zero(t, repairCalls.Load())

			require.NoError(t, second())
			track := receiver.Track()
			require.NotNil(t, track)
			assert.False(t, track.repairReadRequested.Load())
			receiver.mu.RLock()
			started := receiver.tracks[0].repairReaderStarted
			startImmediately := receiver.tracks[0].startRepairReaderImmediately
			receiver.mu.RUnlock()
			assert.True(t, startImmediately)
			assert.True(t, started)
			require.Eventually(t, func() bool { return repairCalls.Load() == 1 }, time.Second, time.Millisecond)
		})
	}
}

func TestRTPReceiverReadAfterStopDoesNotStartRepairReader(t *testing.T) {
	receiver, track := newRepairReaderPolicyTestReceiver(t, true, 2222, nil)
	var calls atomic.Int32
	repairReader := interceptor.RTPReaderFunc(
		func(_ []byte, a interceptor.Attributes) (int, interceptor.Attributes, error) {
			calls.Add(1)

			return 0, a, io.EOF
		},
	)
	require.NoError(t, receiver.receiveForRtx(
		2222, "", &interceptor.StreamInfo{SSRC: 2222}, nil, repairReader, false, nil, nil,
	))
	require.NoError(t, receiver.Stop())

	_, _, err := track.Read(make([]byte, receiveMTU))
	require.ErrorIs(t, err, io.EOF)
	receiver.mu.RLock()
	started := receiver.tracks[0].repairReaderStarted
	receiver.mu.RUnlock()
	assert.False(t, started)
	assert.Zero(t, calls.Load())
}

func newRepairReaderPolicyTestReceiver(
	t *testing.T,
	customBuffer bool,
	rtxSSRC SSRC,
	primaryReader interceptor.RTPReader,
) (*RTPReceiver, *TrackRemote) {
	t.Helper()

	transportSettings := SettingEngine{}
	if customBuffer {
		transportSettings.BufferFactory = func(packetio.BufferPacketType, uint32) io.ReadWriteCloser {
			return nil
		}
	}
	transportAPI := NewAPI(WithSettingEngine(transportSettings))
	receiverAPI := NewAPI()
	receiver, err := receiverAPI.NewRTPReceiver(
		RTPCodecTypeVideo,
		&DTLSTransport{api: transportAPI},
	)
	require.NoError(t, err)

	receiver.configureReceive(RTPReceiveParameters{Encodings: []RTPDecodingParameters{{
		RTPCodingParameters: RTPCodingParameters{
			RID:  "rid",
			SSRC: 1111,
			RTX:  RTPRtxParameters{SSRC: rtxSSRC},
		},
	}}})
	if primaryReader == nil {
		primaryReader = interceptor.RTPReaderFunc(
			func(b []byte, a interceptor.Attributes) (int, interceptor.Attributes, error) {
				return copy(b, []byte{
					0x80, 96, 0, 1, 0, 0, 0, 0, 0, 0, 0x04, 0x57, 0xAA,
				}), a, nil
			},
		)
	}
	params := RTPParameters{Codecs: []RTPCodecParameters{{
		RTPCodecCapability: RTPCodecCapability{MimeType: MimeTypeVP8},
		PayloadType:        96,
	}}}
	track, err := receiver.receiveForRid(
		"rid", params, &interceptor.StreamInfo{SSRC: 1111}, nil, primaryReader, false, nil, nil, nil,
	)
	require.NoError(t, err)
	close(receiver.received)
	t.Cleanup(func() {
		assert.NoError(t, receiver.Stop())
	})

	return receiver, track
}

func TestRTPReceiver_readRTX_ChannelAccessSafe(t *testing.T) {
	receiver := &RTPReceiver{
		kind:       RTPCodecTypeVideo,
		received:   make(chan any),
		closedChan: make(chan any),
		rtxPool: sync.Pool{New: func() any {
			return make([]byte, 1200)
		}},
	}

	receiver.configureReceive(RTPReceiveParameters{
		Encodings: []RTPDecodingParameters{
			{
				RTPCodingParameters: RTPCodingParameters{
					RID:  "rid",
					SSRC: 1111,
					RTX: RTPRtxParameters{
						SSRC: 2222,
					},
				},
			},
		},
	})

	params := RTPParameters{
		Codecs: []RTPCodecParameters{
			{
				RTPCodecCapability: RTPCodecCapability{MimeType: MimeTypeVP8},
				PayloadType:        96,
			},
		},
	}
	ridStreamInfo := &interceptor.StreamInfo{SSRC: 1111}
	track, err := receiver.receiveForRid("rid", params, ridStreamInfo, nil, nil, false, nil, nil, nil)
	require.NoError(t, err)

	close(receiver.received)

	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-stop:
				return
			default:
				_ = receiver.readRTX(track)
			}
		}
	}()

	repairStreamInfo := &interceptor.StreamInfo{SSRC: 2222}
	rtpInterceptor := interceptor.RTPReaderFunc(
		func(_ []byte, a interceptor.Attributes) (int, interceptor.Attributes, error) {
			return 0, a, io.EOF
		},
	)

	for range 50 {
		require.NoError(t, receiver.receiveForRtx(
			SSRC(2222), "", repairStreamInfo, nil, rtpInterceptor, false, nil, nil,
		))
	}

	close(stop)
	<-done
}

func TestRTPReceiver_ReadRTP_SimulcastNoRace(t *testing.T) {
	receiver := &RTPReceiver{
		kind:       RTPCodecTypeVideo,
		received:   make(chan any),
		closedChan: make(chan any),
		rtxPool: sync.Pool{New: func() any {
			return make([]byte, 1200)
		}},
	}

	receiver.configureReceive(RTPReceiveParameters{
		Encodings: []RTPDecodingParameters{
			{RTPCodingParameters: RTPCodingParameters{RID: "low", SSRC: 1111}},
			{RTPCodingParameters: RTPCodingParameters{RID: "high", SSRC: 2222}},
		},
	})

	params := RTPParameters{
		Codecs: []RTPCodecParameters{
			{
				RTPCodecCapability: RTPCodecCapability{MimeType: MimeTypeVP8},
				PayloadType:        96,
			},
		},
	}

	lowPkt, err := rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			PayloadType:    96,
			SequenceNumber: 1,
			Timestamp:      1,
			SSRC:           1111,
		},
		Payload: []byte{0x01},
	}.Marshal()
	require.NoError(t, err)

	lowCh := make(chan []byte, 10)
	lowInterceptor := interceptor.RTPReaderFunc(
		func(b []byte, a interceptor.Attributes) (int, interceptor.Attributes, error) {
			pkt, ok := <-lowCh
			if !ok {
				return 0, a, io.EOF
			}

			n := copy(b, pkt)

			return n, a, nil
		},
	)
	lowTrack, err := receiver.receiveForRid(
		"low", params, &interceptor.StreamInfo{SSRC: 1111}, nil, lowInterceptor, false, nil, nil, nil,
	)
	require.NoError(t, err)
	lowTrack.mu.Lock()
	lowTrack.payloadType = 96
	lowTrack.codec = params.Codecs[0]
	lowTrack.params = params
	lowTrack.mu.Unlock()

	close(receiver.received)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for range 5 {
			_, _, err = lowTrack.Read(make([]byte, 1500))
			require.NoError(t, err)
		}
	}()

	repairStreamInfo := &interceptor.StreamInfo{SSRC: 3333}
	repairInterceptor := interceptor.RTPReaderFunc(
		func(_ []byte, a interceptor.Attributes) (int, interceptor.Attributes, error) {
			return 0, a, io.EOF
		},
	)
	require.NoError(t, receiver.receiveForRtx(
		SSRC(0), "low", repairStreamInfo, nil, repairInterceptor, false, nil, nil,
	))

	highInterceptor := interceptor.RTPReaderFunc(
		func(b []byte, a interceptor.Attributes) (int, interceptor.Attributes, error) {
			return 0, a, io.EOF
		},
	)
	_, err = receiver.receiveForRid(
		"high", params, &interceptor.StreamInfo{SSRC: 2222}, nil, highInterceptor, false, nil, nil, nil,
	)
	require.NoError(t, err)
	receiver.tracks[1].track.mu.Lock()
	receiver.tracks[1].track.payloadType = 96
	receiver.tracks[1].track.codec = params.Codecs[0]
	receiver.tracks[1].track.params = params
	receiver.tracks[1].track.mu.Unlock()

	for range 5 {
		lowCh <- lowPkt
	}
	close(lowCh)
	wg.Wait()
}

// TestRTPReceiver_CollectStats_Mapping validates that collectStats maps
// interceptor/pkg/stats values into InboundRTPStreamStats.
func TestRTPReceiver_CollectStats_Mapping(t *testing.T) {
	ssrc := SSRC(1234)
	now := time.Now()
	pr := uint64(math.MaxUint32) + 42
	pl := int64(math.MaxInt32) + 7
	jitter := 0.123
	bytes := uint64(98765)
	hdrBytes := uint64(4321)
	fir := uint32(3)
	pli := uint32(5)
	nack := uint32(7)

	fg := &fakeGetter{s: stats.Stats{
		InboundRTPStreamStats: stats.InboundRTPStreamStats{
			ReceivedRTPStreamStats: stats.ReceivedRTPStreamStats{
				PacketsReceived: pr,
				PacketsLost:     pl,
				Jitter:          jitter,
			},
			LastPacketReceivedTimestamp: now,
			HeaderBytesReceived:         hdrBytes,
			BytesReceived:               bytes,
			FIRCount:                    fir,
			PLICount:                    pli,
			NACKCount:                   nack,
		},
	}}

	// Minimal RTPReceiver with one track
	receiver := &RTPReceiver{
		kind: RTPCodecTypeVideo,
		log:  logging.NewDefaultLoggerFactory().NewLogger("RTPReceiverTest"),
	}
	tr := newTrackRemote(RTPCodecTypeVideo, ssrc, 0, "", receiver)
	receiver.tracks = []trackStreams{{track: tr}}

	collector := newStatsReportCollector()
	receiver.collectStats(collector, nil)
	report := collector.Ready()

	// Fetch the generated inbound-rtp stat by ID
	statID := "inbound-rtp-1234"
	_, ok := report[statID]
	require.False(t, ok, "unexpected inbound stat")

	receiver.collectStats(collector, fg)
	report = collector.Ready()
	got, ok := report[statID]
	require.True(t, ok, "missing inbound stat")

	inbound, ok := got.(InboundRTPStreamStats)
	require.True(t, ok)

	// Wrap-around semantics for casts
	assert.Equal(t, uint32(pr), inbound.PacketsReceived) //nolint:gosec
	assert.Equal(t, int32(pl), inbound.PacketsLost)      //nolint:gosec
	assert.Equal(t, jitter, inbound.Jitter)
	assert.Equal(t, bytes, inbound.BytesReceived)
	assert.Equal(t, hdrBytes, inbound.HeaderBytesReceived)
	assert.Equal(t, fir, inbound.FIRCount)
	assert.Equal(t, pli, inbound.PLICount)
	assert.Equal(t, nack, inbound.NACKCount)
	// Timestamp should be set (millisecond precision)
	assert.Greater(t, float64(inbound.LastPacketReceivedTimestamp), 0.0)
}

func TestRTPReceiver_CollectStats_AudioPlayoutPull(t *testing.T) {
	receiver := &RTPReceiver{
		kind: RTPCodecTypeAudio,
		log:  logging.NewDefaultLoggerFactory().NewLogger("RTPReceiverTest"),
	}

	track := newTrackRemote(RTPCodecTypeAudio, 7777, 0, "", receiver)
	receiver.tracks = []trackStreams{{track: track}}

	provider := &fakeAudioPlayoutStatsProvider{
		stats: AudioPlayoutStats{
			ID:                   "media-playout-7777",
			Type:                 StatsTypeMediaPlayout,
			Kind:                 string(MediaKindAudio),
			TotalSamplesCount:    960,
			TotalSamplesDuration: float64(960) / 48000,
			TotalPlayoutDelay:    0.5,
		},
		ok: true,
	}
	_ = provider.AddTrack(track)

	collector := newStatsReportCollector()
	receiver.collectStats(collector, &fakeGetter{})
	report := collector.Ready()

	got, ok := report["media-playout-7777"]
	require.True(t, ok, "missing audio playout stats entry")

	playout, ok := got.(AudioPlayoutStats)
	require.True(t, ok)

	assert.Equal(t, provider.stats.TotalSamplesCount, playout.TotalSamplesCount)
	assert.Equal(t, provider.stats.TotalSamplesDuration, playout.TotalSamplesDuration)
	assert.Equal(t, provider.stats.TotalPlayoutDelay, playout.TotalPlayoutDelay)
	assert.NotZero(t, playout.Timestamp)
	assert.Equal(t, 1, provider.calls)
}

func TestRTPReceiver_CollectStats_AudioPlayoutSharedProvider(t *testing.T) {
	receiver := &RTPReceiver{
		kind: RTPCodecTypeAudio,
		log:  logging.NewDefaultLoggerFactory().NewLogger("RTPReceiverTest"),
	}

	trackOne := newTrackRemote(RTPCodecTypeAudio, 5555, 0, "", receiver)
	trackTwo := newTrackRemote(RTPCodecTypeAudio, 6666, 0, "", receiver)
	receiver.tracks = []trackStreams{{track: trackOne}, {track: trackTwo}}

	provider := &fakeAudioPlayoutStatsProvider{
		stats: AudioPlayoutStats{
			ID:                "shared-playout",
			Type:              StatsTypeMediaPlayout,
			Kind:              string(MediaKindAudio),
			TotalSamplesCount: 100,
		},
		ok: true,
	}

	_ = provider.AddTrack(trackOne)
	_ = provider.AddTrack(trackTwo)

	collector := newStatsReportCollector()
	receiver.collectStats(collector, &fakeGetter{})
	report := collector.Ready()

	got, ok := report["shared-playout"]
	require.True(t, ok, "shared provider stats missing")

	playout, ok := got.(AudioPlayoutStats)
	require.True(t, ok)
	assert.Equal(t, provider.stats.TotalSamplesCount, playout.TotalSamplesCount)
	assert.Equal(t, provider.stats.Type, playout.Type)
	assert.Equal(t, provider.stats.Kind, playout.Kind)
	assert.Equal(t, provider.stats.ID, playout.ID)
	assert.NotZero(t, playout.Timestamp)
	assert.Equal(t, 2, provider.calls)
}

func TestRTPReceiver_CollectStats_AudioPlayoutTimestampAlignment(t *testing.T) {
	receiver := &RTPReceiver{
		kind: RTPCodecTypeAudio,
		log:  logging.NewDefaultLoggerFactory().NewLogger("RTPReceiverTest"),
	}

	track := newTrackRemote(RTPCodecTypeAudio, 9999, 0, "", receiver)
	receiver.tracks = []trackStreams{{track: track}}

	provider := &fakeAudioPlayoutStatsProvider{
		stats: AudioPlayoutStats{
			ID:                "media-playout-9999",
			Type:              StatsTypeMediaPlayout,
			Kind:              string(MediaKindAudio),
			TotalSamplesCount: 1,
		},
		ok: true,
	}

	_ = provider.AddTrack(track)

	collector := newStatsReportCollector()
	receiver.collectStats(collector, &fakeGetter{})
	report := collector.Ready()

	got, ok := report["media-playout-9999"]
	require.True(t, ok, "playout stats missing")
	playout, ok := got.(AudioPlayoutStats)
	require.True(t, ok, "playout stats type assertion failed")
	require.NotZero(t, provider.lastNow)
	assert.Equal(t, statsTimestampFrom(provider.lastNow), playout.Timestamp)
}

type fakeGetter struct{ s stats.Stats }

func (f *fakeGetter) Get(uint32) *stats.Stats { return &f.s }

type fakeAudioPlayoutStatsProvider struct {
	stats AudioPlayoutStats
	ok    bool

	calls   int
	lastNow time.Time
}

func (f *fakeAudioPlayoutStatsProvider) Snapshot(now time.Time) (AudioPlayoutStats, bool) {
	f.calls++
	f.lastNow = now

	return f.stats, f.ok
}

func (f *fakeAudioPlayoutStatsProvider) AddTrack(track *TrackRemote) error {
	track.addProvider(f)

	return nil
}

func (f *fakeAudioPlayoutStatsProvider) RemoveTrack(track *TrackRemote) {
	track.removeProvider(f)
}

func TestRTPReceiverRTXStreamInfoMimeType(t *testing.T) {
	for _, tt := range []struct {
		name          string
		configureNack bool
	}{
		{name: "passthrough"},
		{name: "ConfigureNack", configureNack: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			lim := test.TimeOut(time.Second * 30)
			defer lim.Stop()

			report := test.CheckRoutines(t)
			defer report()

			mediaEngine := &MediaEngine{}
			require.NoError(t, mediaEngine.RegisterDefaultCodecs())
			ir := &interceptor.Registry{}
			if tt.configureNack {
				require.NoError(t, ConfigureNack(mediaEngine, ir))
			}

			// Collect the final readers and StreamInfos bound on the receiver side.
			var (
				boundStreamInfos        []*interceptor.StreamInfo
				readerWrappedByMimeType = map[string]bool{}
			)
			mockInterceptor := &mock_interceptor.Interceptor{
				BindRemoteStreamFn: func(
					info *interceptor.StreamInfo,
					reader interceptor.RTPReader,
				) interceptor.RTPReader {
					boundStreamInfos = append(boundStreamInfos, info)
					_, passthrough := reader.(*srtpRTPReader)
					readerWrappedByMimeType[info.MimeType] = !passthrough

					return reader
				},
			}
			ir.Add(&mock_interceptor.Factory{
				NewInterceptorFn: func(_ string) (interceptor.Interceptor, error) { return mockInterceptor, nil },
			})

			sender, receiver, err := NewAPI(
				WithMediaEngine(mediaEngine),
				WithInterceptorRegistry(ir),
			).newPair(Configuration{})
			require.NoError(t, err)
			defer closePairNow(t, sender, receiver)

			track, err := NewTrackLocalStaticSample(
				RTPCodecCapability{MimeType: MimeTypeVP8},
				"video",
				"pion",
			)
			require.NoError(t, err)

			_, err = sender.AddTrack(track)
			require.NoError(t, err)

			// OnTrack is reached through internal probing, without a public TrackRemote.Read.
			trackReceived, trackReceivedCancel := context.WithCancel(context.Background())
			rtpReceiverReceived := make(chan *RTPReceiver, 1)
			receiver.OnTrack(func(_ *TrackRemote, rtpReceiver *RTPReceiver) {
				rtpReceiverReceived <- rtpReceiver
				trackReceivedCancel()
			})

			require.NoError(t, signalPair(sender, receiver))

			func() {
				ticker := time.NewTicker(time.Millisecond * 20)
				defer ticker.Stop()
				for {
					select {
					case <-trackReceived.Done():
						return
					case <-ticker.C:
						require.NoError(t, track.WriteSample(media.Sample{Data: []byte{0xAA}, Duration: time.Second}))
					}
				}
			}()

			rtxCount := 0
			for _, info := range boundStreamInfos {
				if info.MimeType == MimeTypeRTX {
					rtxCount++
				}
			}
			assert.Equal(t, 1, rtxCount,
				"expected exactly one RTX StreamInfo with MimeType %q, got %d (all types: %v)",
				MimeTypeRTX, rtxCount, mimeTypes(boundStreamInfos))
			assert.Equal(t, tt.configureNack, readerWrappedByMimeType[MimeTypeVP8], "primary reader wrapper")
			assert.False(t, readerWrappedByMimeType[MimeTypeRTX], "RTX reader wrapper")

			rtpReceiver := <-rtpReceiverReceived
			rtpReceiver.mu.RLock()
			trackCount := len(rtpReceiver.tracks)
			repairReaderStarted := trackCount == 1 && rtpReceiver.tracks[0].repairReaderStarted
			rtpReceiver.mu.RUnlock()
			require.Equal(t, 1, trackCount)
			assert.Equal(t, tt.configureNack, repairReaderStarted)
		})
	}
}

// helper to print all mime types for debugging.
func mimeTypes(infos []*interceptor.StreamInfo) []string {
	out := make([]string, len(infos))
	for i, info := range infos {
		out[i] = info.MimeType
	}

	return out
}

// TestRTPReceiver_CollectStats_RID validates that collectStats correctly maps RID
// from TrackRemote into InboundRTPStreamStats.
func TestRTPReceiver_CollectStats_RID(t *testing.T) {
	ssrc := SSRC(1234)

	fg := &fakeGetter{s: stats.Stats{}}

	receiver := &RTPReceiver{
		kind: RTPCodecTypeVideo,
		log:  logging.NewDefaultLoggerFactory().NewLogger("RTPReceiverTest"),
	}

	// Case 1: RID empty
	tr := newTrackRemote(RTPCodecTypeVideo, ssrc, 0, "", receiver)
	receiver.tracks = []trackStreams{{track: tr}}

	collector := newStatsReportCollector()
	receiver.collectStats(collector, fg)
	report := collector.Ready()

	statID := "inbound-rtp-1234"
	got, ok := report[statID]
	require.True(t, ok)

	inbound, ok := got.(InboundRTPStreamStats)
	require.True(t, ok)

	assert.Equal(t, "", inbound.Rid)

	// Case 2: RID present
	rid := "f"
	tr = newTrackRemote(RTPCodecTypeVideo, ssrc, 0, rid, receiver)
	receiver.tracks = []trackStreams{{track: tr}}

	collector = newStatsReportCollector()
	receiver.collectStats(collector, fg)
	report = collector.Ready()

	got, ok = report[statID]
	require.True(t, ok)

	inbound, ok = got.(InboundRTPStreamStats)
	require.True(t, ok)

	assert.Equal(t, rid, inbound.Rid)
}
