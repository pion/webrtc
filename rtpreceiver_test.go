// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js

package webrtc

import (
	"context"
	"errors"
	"io"
	"math"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pion/interceptor"
	mock_interceptor "github.com/pion/interceptor/pkg/mock"
	"github.com/pion/interceptor/pkg/stats"
	"github.com/pion/interceptor/pkg/twcc"
	"github.com/pion/logging"
	"github.com/pion/rtcp"
	"github.com/pion/rtp"
	"github.com/pion/sdp/v3"
	"github.com/pion/srtp/v3"
	"github.com/pion/transport/v5/packetio"
	"github.com/pion/transport/v5/test"
	"github.com/pion/webrtc/v5/pkg/media"
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
	defer func() {
		assert.NoError(t, wan.Stop())
		closePairNow(t, sender, receiver)
	}()

	track, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: MimeTypeVP8}, "video", "pion")
	assert.NoError(t, err)

	rtpSender, err := sender.AddTrack(track)
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

	offer, err := sender.CreateOffer(nil)
	require.NoError(t, err)
	offerGathered := GatheringCompletePromise(sender)
	require.NoError(t, sender.SetLocalDescription(offer))
	<-offerGathered
	require.NoError(t, receiver.SetRemoteDescription(*sender.LocalDescription()))
	answer, err := receiver.CreateAnswer(nil)
	require.NoError(t, err)
	answerGathered := GatheringCompletePromise(receiver)
	require.NoError(t, receiver.SetLocalDescription(answer))
	receivers := receiver.GetReceivers()
	require.Len(t, receivers, 1)
	require.Error(t, receivers[0].SetReadDeadlineSimulcastSSRC(time.Now(), rtpSender.GetParameters().Encodings[0].SSRC))
	<-answerGathered
	require.NoError(t, sender.SetRemoteDescription(*receiver.LocalDescription()))

	<-peerConnectionsConnected
	assert.NoError(t, track.WriteSample(media.Sample{Data: []byte{0xAA}, Duration: time.Second}))

	<-seenPacket.Done()
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

	close(receiver.received)
	track := receiver.Track()

	reads := []func([]byte) (int, interceptor.Attributes, error){
		track.Read,
		receiver.Read,
		func(b []byte) (int, interceptor.Attributes, error) { return receiver.ReadSimulcast(b, "rid") },
	}

	var readers sync.WaitGroup
	for _, read := range reads {
		readers.Add(1)
		go func() {
			defer readers.Done()
			_, _, readErr := read(make([]byte, 1500))
			assert.ErrorIs(t, readErr, os.ErrDeadlineExceeded)
		}()
	}

	expired := time.Now().Add(-time.Second)
	assert.NoError(t, track.SetReadDeadline(expired))
	assert.NoError(t, receiver.SetReadDeadline(expired))
	assert.NoError(t, receiver.SetReadDeadlineSimulcast(expired, "rid"))
	readers.Wait()

	assert.NoError(t, track.SetReadDeadline(time.Time{}))
	assert.NoError(t, receiver.SetReadDeadlineSimulcast(time.Time{}, "rid"))
	for _, read := range reads {
		readers.Add(1)
		go func() {
			defer readers.Done()
			_, _, readErr := read(make([]byte, 1500))
			assert.True(t, errors.Is(readErr, io.EOF) || errors.Is(readErr, io.ErrClosedPipe), "%v", readErr)
		}()
	}
	require.NoError(t, receiver.Stop())
	readers.Wait()

	params := RTPParameters{
		Codecs: []RTPCodecParameters{
			{
				RTPCodecCapability: RTPCodecCapability{MimeType: MimeTypeVP8},
			},
		},
	}
	ridStreamInfo := &interceptor.StreamInfo{SSRC: 1111}
	rtxStreamInfo := &interceptor.StreamInfo{SSRC: 2222}
	track, err = receiver.receiveForRid("rid", params, ridStreamInfo, &streamsForSSRCResult{}, nil)
	assert.Nil(t, track)
	assert.ErrorIs(t, err, io.EOF)

	err = receiver.receiveForRtx(SSRC(0), "rid", rtxStreamInfo, &streamsForSSRCResult{})
	assert.ErrorIs(t, err, io.EOF)
}

func TestRTPReceiver_ReadRTP_SimulcastNoRace(t *testing.T) {
	receiver := &RTPReceiver{
		kind:       RTPCodecTypeVideo,
		received:   make(chan any),
		closedChan: make(chan struct{}),
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
		"low", params, &interceptor.StreamInfo{SSRC: 1111},
		&streamsForSSRCResult{rtpInterceptor: lowInterceptor}, nil,
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
		SSRC(0), "low", repairStreamInfo,
		&streamsForSSRCResult{rtpInterceptor: repairInterceptor},
	))

	highInterceptor := interceptor.RTPReaderFunc(
		func(b []byte, a interceptor.Attributes) (int, interceptor.Attributes, error) {
			return 0, a, io.EOF
		},
	)
	_, err = receiver.receiveForRid(
		"high", params, &interceptor.StreamInfo{SSRC: 2222},
		&streamsForSSRCResult{rtpInterceptor: highInterceptor}, nil,
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

	assert.Equal(t, pr, inbound.PacketsReceived)
	assert.Equal(t, pl, inbound.PacketsLost)
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
			var boundStreamInfos []*interceptor.StreamInfo
			mockInterceptor := &mock_interceptor.Interceptor{
				BindRemoteStreamFn: func(
					info *interceptor.StreamInfo,
					reader interceptor.RTPReader,
				) interceptor.RTPReader {
					boundStreamInfos = append(boundStreamInfos, info)

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
			rtpReceiver := <-rtpReceiverReceived
			rtpReceiver.mu.RLock()
			require.Len(t, rtpReceiver.tracks, 1)
			assert.Same(t, rtpReceiver.tracks[0].repairReader, rtpReceiver.tracks[0].rtpReader.repair.Load())
			rtpReceiver.mu.RUnlock()
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

func TestRTPReceiverRTXMalformedPacketNoPanic(t *testing.T) {
	api := NewAPI()
	receiver, err := api.NewRTPReceiver(RTPCodecTypeVideo, &DTLSTransport{api: api})
	require.NoError(t, err)
	receiver.configureReceive(RTPReceiveParameters{Encodings: []RTPDecodingParameters{{
		RTPCodingParameters: RTPCodingParameters{SSRC: 1111, RTX: RTPRtxParameters{SSRC: 2222}},
	}}})
	track := receiver.Track()
	close(receiver.received)
	t.Cleanup(func() { assert.NoError(t, receiver.Stop()) })

	packets := [][]byte{
		nil, // Empty read with stale buffer contents.
		{0x90, 97, 0, 1, 0, 0, 0, 0, 0, 0, 0x08, 0xAE},          // Truncated extension.
		{0xA0, 97, 0, 1, 0, 0, 0, 0, 0, 0, 0x08, 0xAE, 0, 1, 0}, // Zero padding length.
		{0xA0, 97, 0, 1, 0, 0, 0, 0, 0, 0, 0x08, 0xAE, 0, 1, 4}, // Padding exceeds payload.
		{0xA0, 97, 0, 1, 0, 0, 0, 0, 0, 0, 0x08, 0xAE, 0, 1},    // Padding leaves a short OSN.
		{
			// Valid RTX with CSRC, extra extension padding, marker, and two bytes of RTP padding.
			0xB1, 0xE1, 0x13, 0x88, 0, 0, 0, 0, 0, 0, 0x08, 0xAE,
			1, 2, 3, 4, 0xBE, 0xDE, 0, 2, 0x10, 0x42, 0, 0, 0, 0, 0, 0,
			0x04, 0xD2, 0xA1, 0, 2,
		},
	}
	repairReader := interceptor.RTPReaderFunc(
		func(b []byte, attributes interceptor.Attributes) (int, interceptor.Attributes, error) {
			if len(packets) == 0 {
				return 0, attributes, io.EOF
			}
			b[0] = 0xA0
			n := copy(b, packets[0])
			packets = packets[1:]

			return n, attributes, nil
		},
	)

	receiver.tracks[0].rtpInterceptor = repairReader

	packet, attributes, err := track.ReadRTP()
	require.NoError(t, err)
	require.NotNil(t, packet)
	assert.Equal(t, uint8(96), packet.PayloadType)
	assert.Equal(t, uint32(1111), packet.SSRC)
	assert.Equal(t, uint8(97), attributes.Get(AttributeRtxPayloadType))
	assert.Equal(t, uint16(5000), attributes.Get(AttributeRtxSequenceNumber))
	assert.Equal(t, uint32(2222), attributes.Get(AttributeRtxSsrc))
	assert.Equal(t, uint16(1234), packet.SequenceNumber)
	assert.True(t, packet.Marker)
	assert.Equal(t, []uint32{0x01020304}, packet.CSRC)
	assert.Equal(t, []byte{0x42}, packet.GetExtension(1))
	assert.Equal(t, byte(2), packet.Header.PaddingSize)
	assert.Equal(t, []byte{0xA1}, packet.Payload)
}

func newRTXReadTestReceiver(t *testing.T, api *API) (*RTPReceiver, func(*rtp.Packet)) {
	t.Helper()
	key, salt := make([]byte, 16), make([]byte, 14)
	config := &srtp.Config{
		Profile: srtp.ProtectionProfileAes128CmHmacSha1_80,
		Keys: srtp.SessionKeys{
			LocalMasterKey: key, LocalMasterSalt: salt, RemoteMasterKey: key, RemoteMasterSalt: salt,
		},
		BufferFactory: api.settingEngine.BufferFactory,
	}
	local, remote := net.Pipe()
	rtpSession, err := srtp.NewSessionSRTP(local, config)
	require.NoError(t, err)
	rtcpLocal, rtcpRemote := net.Pipe()
	rtcpSession, err := srtp.NewSessionSRTCP(rtcpLocal, config)
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, remote.Close())
		assert.NoError(t, rtcpRemote.Close())
		assert.NoError(t, rtpSession.Close())
		assert.NoError(t, rtcpSession.Close())
		assert.NoError(t, api.interceptor.Close())
	})
	transport := &DTLSTransport{api: api}
	transport.srtpSession.Store(rtpSession)
	transport.srtcpSession.Store(rtcpSession)
	receiver, err := api.NewRTPReceiver(RTPCodecTypeVideo, transport)
	require.NoError(t, err)
	require.NoError(t, receiver.Receive(RTPReceiveParameters{Encodings: []RTPDecodingParameters{{
		RTPCodingParameters: RTPCodingParameters{SSRC: 1111, RTX: RTPRtxParameters{SSRC: 2222}},
	}}}))
	t.Cleanup(func() { assert.NoError(t, receiver.Stop()) })
	require.NoError(t, receiver.Track().SetReadDeadline(time.Now().Add(time.Second)))
	encrypt, err := srtp.CreateContext(key, salt, config.Profile)
	require.NoError(t, err)

	return receiver, func(packet *rtp.Packet) {
		t.Helper()
		raw, marshalErr := packet.Marshal()
		require.NoError(t, marshalErr)
		encrypted, encryptErr := encrypt.EncryptRTP(nil, raw, nil)
		require.NoError(t, encryptErr)
		require.NoError(t, remote.SetWriteDeadline(time.Now().Add(time.Second)))
		_, writeErr := remote.Write(encrypted)
		require.NoError(t, writeErr)
	}
}

func TestRTPReceiverReadPullsRTXProbeThroughTWCC(t *testing.T) {
	t.Cleanup(test.CheckRoutines(t))
	registry := &interceptor.Registry{}
	factory, err := twcc.NewSenderInterceptor(twcc.SendInterval(5 * time.Millisecond))
	require.NoError(t, err)
	registry.Add(factory)
	var packetsRead atomic.Int32
	registry.Add(&mock_interceptor.Factory{NewInterceptorFn: func(string) (interceptor.Interceptor, error) {
		return &mock_interceptor.Interceptor{BindRemoteStreamFn: func(info *interceptor.StreamInfo, reader interceptor.RTPReader) interceptor.RTPReader {
			return interceptor.RTPReaderFunc(func(b []byte, attributes interceptor.Attributes) (int, interceptor.Attributes, error) {
				n, attributes, readErr := reader.Read(b, attributes)
				if readErr == nil {
					var header rtp.Header
					_, headerErr := header.Unmarshal(b[:n])
					assert.NoError(t, headerErr)
					assert.Equal(t, info.SSRC, header.SSRC)
					packetsRead.Add(1)
				}

				return n, attributes, readErr
			})
		}}, nil
	}})
	api := NewAPI(WithInterceptorRegistry(registry))
	api.interceptor, err = registry.Build("")
	require.NoError(t, err)
	require.NoError(t, api.mediaEngine.RegisterHeaderExtension(
		RTPHeaderExtensionCapability{URI: sdp.TransportCCURI}, RTPCodecTypeVideo,
	))
	feedback := make(chan *rtcp.TransportLayerCC, 1)
	api.interceptor.BindRTCPWriter(interceptor.RTCPWriterFunc(func(packets []rtcp.Packet, _ interceptor.Attributes) (int, error) {
		for _, packet := range packets {
			if report, ok := packet.(*rtcp.TransportLayerCC); ok {
				feedback <- report
			}
		}

		return 0, nil
	}))
	receiver, send := newRTXReadTestReceiver(t, api)
	var extensionID int
	for _, extension := range receiver.GetParameters().HeaderExtensions {
		if extension.URI == sdp.TransportCCURI {
			extensionID = extension.ID
		}
	}
	require.NotZero(t, extensionID)
	probe := &rtp.Packet{Header: rtp.Header{
		Version: 2, SSRC: 2222, PayloadType: 97, SequenceNumber: 1, Padding: true, PaddingSize: 20,
	}}
	require.NoError(t, probe.SetExtension(uint8(extensionID), []byte{0, 77})) //nolint:gosec
	send(probe)
	send(&rtp.Packet{Header: rtp.Header{Version: 2, SSRC: 1111, PayloadType: 96, SequenceNumber: 1}, Payload: []byte{0xAA}})
	assert.Zero(t, packetsRead.Load(), "interceptors must be driven by Read")
	packet, _, err := receiver.Track().ReadRTP()
	require.NoError(t, err)
	assert.Equal(t, uint32(1111), packet.SSRC)
	assert.Equal(t, int32(2), packetsRead.Load(), "both the RTX probe and primary packet must pass through interceptors")
	select {
	case report := <-feedback:
		assert.Equal(t, uint16(77), report.BaseSequenceNumber)
		assert.Equal(t, uint16(1), report.PacketStatusCount)
	case <-time.After(time.Second):
		require.FailNow(t, "RTX padding probe was not included in TWCC feedback")
	}

	send(&rtp.Packet{Header: rtp.Header{Version: 2, SSRC: 2222, PayloadType: 97, SequenceNumber: 2}, Payload: []byte{0, 42, 0xBB}})
	packet, attributes, err := receiver.Track().ReadRTP()
	require.NoError(t, err)
	assert.Equal(t, uint32(1111), packet.SSRC)
	assert.Equal(t, uint16(42), packet.SequenceNumber)
	assert.Equal(t, uint16(2), attributes.Get(AttributeRtxSequenceNumber))
	assert.Equal(t, uint32(2222), attributes.Get(AttributeRtxSsrc))
}

func TestRTPReceiverInterceptorDiscardsPacket(t *testing.T) {
	for _, ssrc := range []uint32{1111, 2222} {
		api := NewAPI()
		api.interceptor = &mock_interceptor.Interceptor{BindRemoteStreamFn: func(_ *interceptor.StreamInfo, reader interceptor.RTPReader) interceptor.RTPReader {
			return interceptor.RTPReaderFunc(func(b []byte, attributes interceptor.Attributes) (int, interceptor.Attributes, error) {
				if _, _, err := reader.Read(b, attributes); err != nil {
					return 0, nil, err
				}

				return reader.Read(b, nil)
			})
		}}
		receiver, send := newRTXReadTestReceiver(t, api)
		for _, sequence := range []uint16{1, 2} {
			packet := &rtp.Packet{Header: rtp.Header{Version: 2, SSRC: ssrc, PayloadType: 96, SequenceNumber: sequence}, Payload: []byte{0xAA}}
			if ssrc == 2222 {
				packet.PayloadType = 97
				packet.Payload = []byte{0, byte(sequence), 0xAA}
			}
			send(packet)
		}
		packet, _, err := receiver.Track().ReadRTP()
		require.NoError(t, err)
		assert.Equal(t, uint16(2), packet.SequenceNumber)
	}
}

type rtxTestBuffer struct{ *packetio.Buffer }

func (b *rtxTestBuffer) Read(p []byte) (int, error) {
	n, _, err := b.Buffer.Read(p, nil)

	return n, err
}

func (b *rtxTestBuffer) Write(p []byte) (int, error) { return b.Buffer.Write(p, nil) }

func TestRTPReceiverRTXBufferFactory(t *testing.T) {
	seen := map[uint32]bool{}
	api := NewAPI(WithSettingEngine(SettingEngine{BufferFactory: func(kind packetio.BufferPacketType, ssrc uint32) io.ReadWriteCloser {
		if kind == packetio.RTPBufferPacket {
			seen[ssrc] = true
		}

		return &rtxTestBuffer{packetio.NewBuffer()}
	}}))
	receiver, send := newRTXReadTestReceiver(t, api)
	assert.True(t, seen[1111])
	assert.True(t, seen[2222])
	send(&rtp.Packet{Header: rtp.Header{Version: 2, SSRC: 2222, PayloadType: 97, SequenceNumber: 1}, Payload: []byte{0, 42, 0xAA}})
	packet, _, err := receiver.Track().ReadRTP()
	require.NoError(t, err)
	assert.Equal(t, uint16(42), packet.SequenceNumber)
}
