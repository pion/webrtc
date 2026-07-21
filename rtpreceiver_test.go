// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js

package webrtc

import (
	"context"
	"encoding/binary"
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
		name           string
		customBuffer   bool
		primaryWrapped bool
		repairWrapped  bool
		eager          bool
	}{
		{name: "default buffers passthrough"},
		{name: "default buffers primary wrapped", primaryWrapped: true, eager: true},
		{name: "default buffers repair wrapped", repairWrapped: true, eager: true},
		{name: "custom buffer passthrough", customBuffer: true},
		{
			name:           "custom buffer wrapped",
			customBuffer:   true,
			primaryWrapped: true,
			repairWrapped:  true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			primaryPacket := newRepairTestRTPPacket(t, 1111, 96, 1000, []byte{0xAA})
			primaryReader := interceptor.RTPReaderFunc(
				func(b []byte, a interceptor.Attributes) (int, interceptor.Attributes, error) {
					return copy(b, primaryPacket), a, nil
				},
			)
			receiver, track := newRepairTestReceiver(
				t,
				tt.customBuffer,
				2222,
				primaryReader,
				tt.primaryWrapped,
			)

			var firstCalls atomic.Int32
			firstReturned := make(chan struct{}, 1)
			require.NoError(t, receiver.receiveForRtx(
				2222,
				"",
				&interceptor.StreamInfo{SSRC: 2222},
				nil,
				countingEOFReader(&firstCalls, firstReturned),
				tt.repairWrapped,
				nil,
				nil,
			))
			assertRepairReaderCalls(t, &firstCalls, tt.eager)
			if tt.eager {
				requireRepairTestSignal(t, firstReturned, "first repair reader did not return")
			}

			var reboundCalls atomic.Int32
			reboundReturned := make(chan struct{}, 1)
			require.NoError(t, receiver.receiveForRtx(
				2222,
				"",
				&interceptor.StreamInfo{SSRC: 2222},
				nil,
				countingEOFReader(&reboundCalls, reboundReturned),
				tt.repairWrapped,
				nil,
				nil,
			))
			assertRepairReaderCalls(t, &reboundCalls, tt.eager)
			if tt.eager {
				requireRepairTestSignal(t, reboundReturned, "rebound repair reader did not return")
			}
			close(receiver.received)

			peekBuffer := make([]byte, receiveMTU)
			_, err := track.peek(peekBuffer)
			require.NoError(t, err)
			expectedSetupCalls := int32(0)
			if tt.eager {
				expectedSetupCalls = 1
			}
			assert.Equal(t, expectedSetupCalls, firstCalls.Load())
			if !tt.eager {
				assert.Never(t, func() bool { return reboundCalls.Load() != 0 }, 10*time.Millisecond, time.Millisecond)
			}

			_, _, err = track.Read(peekBuffer)
			require.NoError(t, err)
			require.Eventually(t, func() bool { return reboundCalls.Load() == 1 }, time.Second, time.Millisecond)
			if !tt.eager {
				requireRepairTestSignal(t, reboundReturned, "lazy repair reader did not return")
			}
			assert.Never(t, func() bool { return reboundCalls.Load() > 1 }, 10*time.Millisecond, time.Millisecond)

			_, _, err = track.Read(peekBuffer)
			require.NoError(t, err)
			assert.Never(t, func() bool { return reboundCalls.Load() > 1 }, 10*time.Millisecond, time.Millisecond)
			assert.Equal(t, expectedSetupCalls, firstCalls.Load(), "an obsolete generation must not start")
		})
	}
}

func TestTrackRemoteLateRepairBindAndRebind(t *testing.T) {
	for _, customBuffer := range []bool{false, true} {
		name := "default buffer"
		if customBuffer {
			name = "custom buffer"
		}

		t.Run(name, func(t *testing.T) {
			primaryStarted := make(chan struct{}, 1)
			primaryResults := make(chan controlledRTPRead)
			receiver, track := newRepairTestReceiver(
				t,
				customBuffer,
				0,
				controlledRTPReader(primaryStarted, nil, primaryResults),
				false,
			)
			t.Cleanup(func() { close(primaryResults) })
			close(receiver.received)

			firstRead := asyncRepairTestTrackRead(track, receiveMTU)
			requireRepairTestSignal(t, primaryStarted, "primary read did not start")

			firstRepairStarted := make(chan struct{}, 2)
			firstRepairReturned := make(chan struct{}, 2)
			firstRepairResults := make(chan controlledRTPRead, 2)
			t.Cleanup(func() { close(firstRepairResults) })
			require.NoError(t, receiver.receiveForRtx(
				0,
				"rid",
				&interceptor.StreamInfo{SSRC: 2222},
				nil,
				controlledRTPReader(firstRepairStarted, firstRepairReturned, firstRepairResults),
				false,
				nil,
				nil,
			))
			firstRepairResults <- controlledRTPRead{
				packet: newRepairTestRTXPacket(t, 2222, 5000, 1234, []byte{0xA1}),
			}
			requireRepairTestSignal(t, firstRepairStarted, "late repair reader did not start")
			requireRepairTestSignal(t, firstRepairReturned, "late repair reader did not return")
			assertRepairTestPacket(t, requireRepairTestRead(t, firstRead).packet, 1234, []byte{0xA1})
			requireRepairTestSignal(t, firstRepairStarted, "repair reader did not wait for another packet")

			secondRead := asyncRepairTestTrackRead(track, receiveMTU)
			require.Eventually(t, func() bool {
				if track.readMu.TryLock() {
					track.readMu.Unlock()

					return false
				}

				return true
			}, time.Second, time.Millisecond, "second read did not block before repair rebind")
			secondRepairResults := make(chan controlledRTPRead, 1)
			t.Cleanup(func() { close(secondRepairResults) })
			require.NoError(t, receiver.receiveForRtx(
				0,
				"rid",
				&interceptor.StreamInfo{SSRC: 3333},
				nil,
				controlledRTPReader(nil, nil, secondRepairResults),
				false,
				nil,
				nil,
			))

			firstRepairResults <- controlledRTPRead{
				packet: newRepairTestRTXPacket(t, 2222, 5001, 2345, []byte{0xA2}),
			}
			requireRepairTestSignal(t, firstRepairReturned, "stale repair reader did not return")
			select {
			case result := <-secondRead:
				require.FailNow(t, "stale repair packet was delivered", "result: %+v", result)
			case <-time.After(50 * time.Millisecond):
			}

			secondRepairResults <- controlledRTPRead{
				packet: newRepairTestRTXPacket(t, 3333, 6000, 3456, []byte{0xB1}),
			}
			assertRepairTestPacket(t, requireRepairTestRead(t, secondRead).packet, 3456, []byte{0xB1})
		})
	}
}

func TestTrackRemoteDetachedPrimaryRead(t *testing.T) {
	primaryStarted := make(chan struct{}, 1)
	primaryResults := make(chan controlledRTPRead, 1)
	receiver, track := newRepairTestReceiver(
		t,
		false,
		2222,
		controlledRTPReader(primaryStarted, nil, primaryResults),
		false,
	)
	t.Cleanup(func() { close(primaryResults) })
	repairResults := make(chan controlledRTPRead, 1)
	t.Cleanup(func() { close(repairResults) })
	require.NoError(t, receiver.receiveForRtx(
		2222,
		"",
		&interceptor.StreamInfo{SSRC: 2222},
		nil,
		controlledRTPReader(nil, nil, repairResults),
		false,
		nil,
		nil,
	))
	close(receiver.received)

	firstRead := asyncRepairTestTrackRead(track, 64)
	requireRepairTestSignal(t, primaryStarted, "primary read did not start")
	repairResults <- controlledRTPRead{
		packet: newRepairTestRTXPacket(t, 2222, 7000, 4567, []byte{0xC1}),
	}
	assertRepairTestPacket(t, requireRepairTestRead(t, firstRead).packet, 4567, []byte{0xC1})

	largePrimary := newRepairTestRTPPacket(t, 1111, 96, 8000, make([]byte, 500))
	primaryResults <- controlledRTPRead{packet: largePrimary}
	secondResult := requireRepairTestRead(t, asyncRepairTestTrackRead(track, receiveMTU))
	require.NoError(t, secondResult.err)
	assert.Equal(t, largePrimary, secondResult.packet, "the detached primary read was truncated by the earlier caller")

	thirdRead := asyncRepairTestTrackRead(track, 64)
	requireRepairTestSignal(t, primaryStarted, "next primary read did not start")
	repairResults <- controlledRTPRead{
		packet: newRepairTestRTXPacket(t, 2222, 7001, 5678, []byte{0xC2}),
	}
	assertRepairTestPacket(t, requireRepairTestRead(t, thirdRead).packet, 5678, []byte{0xC2})

	primaryResults <- controlledRTPRead{packet: []byte{0x80}, err: context.DeadlineExceeded}
	fourthRead := asyncRepairTestTrackRead(track, receiveMTU)
	requireRepairTestSignal(t, primaryStarted, "detached error did not trigger a fresh primary read")
	freshPrimary := newRepairTestRTPPacket(t, 1111, 96, 8001, []byte{0xDD})
	primaryResults <- controlledRTPRead{packet: freshPrimary}
	fourthResult := requireRepairTestRead(t, fourthRead)
	require.NoError(t, fourthResult.err)
	assert.Equal(t, freshPrimary, fourthResult.packet)

	shortRead := asyncRepairTestTrackRead(track, 1)
	requireRepairTestSignal(t, primaryStarted, "short-buffer primary read did not start")
	repairResults <- controlledRTPRead{
		packet: newRepairTestRTXPacket(t, 2222, 7002, 6789, []byte{0xC3}),
	}
	assert.ErrorIs(t, requireRepairTestRead(t, shortRead).err, io.ErrShortBuffer)
}

func BenchmarkTrackRemoteReadPrimaryWithRTX(b *testing.B) {
	for _, tt := range []struct {
		name    string
		rtxSSRC SSRC
	}{
		{name: "declared RTX", rtxSSRC: 2222},
		{name: "RID without repair"},
	} {
		b.Run(tt.name, func(b *testing.B) {
			packet := []byte{0x80, 0x60, 0x00, 0x01, 0, 0, 0, 0, 0, 0, 0x04, 0x57, 0xAA}
			reader := interceptor.RTPReaderFunc(
				func(dst []byte, a interceptor.Attributes) (int, interceptor.Attributes, error) {
					return copy(dst, packet), a, nil
				},
			)
			_, track := newRepairTestReceiver(b, false, tt.rtxSSRC, reader, false)
			close(track.receiver.received)
			dst := make([]byte, receiveMTU)
			if _, _, err := track.Read(dst); err != nil {
				b.Fatal(err)
			}

			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				if _, _, err := track.Read(dst); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

type controlledRTPRead struct {
	packet []byte
	err    error
}

type repairTestTrackRead struct {
	packet []byte
	err    error
}

func newRepairTestReceiver(
	tb testing.TB,
	customBuffer bool,
	rtxSSRC SSRC,
	primaryReader interceptor.RTPReader,
	primaryWrapped bool,
) (*RTPReceiver, *TrackRemote) {
	tb.Helper()

	settingEngine := &SettingEngine{}
	if customBuffer {
		settingEngine.BufferFactory = func(packetio.BufferPacketType, uint32) io.ReadWriteCloser {
			return nil
		}
	}
	receiver := &RTPReceiver{
		kind:       RTPCodecTypeVideo,
		api:        &API{settingEngine: settingEngine},
		received:   make(chan any),
		closedChan: make(chan any),
		rtxPool: sync.Pool{New: func() any {
			return make([]byte, receiveMTU)
		}},
	}
	receiver.configureReceive(RTPReceiveParameters{
		Encodings: []RTPDecodingParameters{{
			RTPCodingParameters: RTPCodingParameters{
				RID:  "rid",
				SSRC: 1111,
				RTX:  RTPRtxParameters{SSRC: rtxSSRC},
			},
		}},
	})
	params := RTPParameters{Codecs: []RTPCodecParameters{{
		RTPCodecCapability: RTPCodecCapability{MimeType: MimeTypeVP8},
		PayloadType:        96,
	}}}
	track, err := receiver.receiveForRid(
		"rid",
		params,
		&interceptor.StreamInfo{SSRC: 1111},
		nil,
		primaryReader,
		primaryWrapped,
		nil,
		nil,
		nil,
	)
	require.NoError(tb, err)
	track.mu.Lock()
	track.payloadType = 96
	track.mu.Unlock()
	tb.Cleanup(func() {
		receiver.closed.Store(true)
		close(receiver.closedChan)
	})

	return receiver, track
}

func countingEOFReader(calls *atomic.Int32, returned chan<- struct{}) interceptor.RTPReader {
	return interceptor.RTPReaderFunc(
		func(_ []byte, a interceptor.Attributes) (int, interceptor.Attributes, error) {
			calls.Add(1)
			select {
			case returned <- struct{}{}:
			default:
			}

			return 0, a, io.EOF
		},
	)
}

func assertRepairReaderCalls(t *testing.T, calls *atomic.Int32, expected bool) {
	t.Helper()
	if expected {
		require.Eventually(t, func() bool { return calls.Load() == 1 }, time.Second, time.Millisecond)

		return
	}
	assert.Equal(t, int32(0), calls.Load())
}

func controlledRTPReader(
	started chan<- struct{},
	returned chan<- struct{},
	results <-chan controlledRTPRead,
) interceptor.RTPReader {
	return interceptor.RTPReaderFunc(
		func(b []byte, attributes interceptor.Attributes) (int, interceptor.Attributes, error) {
			if started != nil {
				select {
				case started <- struct{}{}:
				default:
				}
			}

			result, ok := <-results
			if !ok {
				return 0, attributes, io.EOF
			}
			n := copy(b, result.packet)
			if returned != nil {
				select {
				case returned <- struct{}{}:
				default:
				}
			}
			if result.err == nil && n < len(result.packet) {
				result.err = io.ErrShortBuffer
			}

			return n, attributes, result.err
		},
	)
}

func asyncRepairTestTrackRead(track *TrackRemote, size int) <-chan repairTestTrackRead {
	result := make(chan repairTestTrackRead, 1)
	go func() {
		b := make([]byte, size)
		n, _, err := track.Read(b)
		result <- repairTestTrackRead{packet: append([]byte(nil), b[:n]...), err: err}
	}()

	return result
}

func requireRepairTestRead(t *testing.T, result <-chan repairTestTrackRead) repairTestTrackRead {
	t.Helper()
	select {
	case readResult := <-result:
		return readResult
	case <-time.After(time.Second):
		require.FailNow(t, "TrackRemote.Read timed out")
	}

	return repairTestTrackRead{}
}

func requireRepairTestSignal(t *testing.T, signal <-chan struct{}, message string) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(time.Second):
		require.FailNow(t, message)
	}
}

func newRepairTestRTPPacket(
	tb testing.TB,
	ssrc uint32,
	payloadType uint8,
	sequenceNumber uint16,
	payload []byte,
) []byte {
	tb.Helper()
	packet, err := (&rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			PayloadType:    payloadType,
			SequenceNumber: sequenceNumber,
			SSRC:           ssrc,
		},
		Payload: payload,
	}).Marshal()
	require.NoError(tb, err)

	return packet
}

func newRepairTestRTXPacket(
	tb testing.TB,
	ssrc uint32,
	sequenceNumber uint16,
	originalSequenceNumber uint16,
	payload []byte,
) []byte {
	tb.Helper()
	originalSequenceNumberBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(originalSequenceNumberBytes, originalSequenceNumber)

	return newRepairTestRTPPacket(
		tb,
		ssrc,
		97,
		sequenceNumber,
		append(originalSequenceNumberBytes, payload...), //nolint:makezero
	)
}

func assertRepairTestPacket(
	t *testing.T,
	rawPacket []byte,
	sequenceNumber uint16,
	payload []byte,
) {
	t.Helper()
	var packet rtp.Packet
	require.NoError(t, packet.Unmarshal(rawPacket))
	assert.Equal(t, uint32(1111), packet.SSRC)
	assert.Equal(t, uint8(96), packet.PayloadType)
	assert.Equal(t, sequenceNumber, packet.SequenceNumber)
	assert.Equal(t, payload, packet.Payload)
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
		require.NoError(t, receiver.receiveForRtx(SSRC(2222), "", repairStreamInfo, nil, rtpInterceptor, false, nil, nil))
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
	t.Cleanup(func() {
		receiver.closed.Store(true)
		close(receiver.closedChan)
	})

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
	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	// Collect all StreamInfos bound on the remote (receiver) side
	var (
		boundStreamInfos []*interceptor.StreamInfo
	)

	mockInterceptor := &mock_interceptor.Interceptor{
		BindRemoteStreamFn: func(info *interceptor.StreamInfo, reader interceptor.RTPReader) interceptor.RTPReader {
			boundStreamInfos = append(boundStreamInfos, info)

			return reader
		},
	}

	ir := &interceptor.Registry{}
	ir.Add(&mock_interceptor.Factory{
		NewInterceptorFn: func(_ string) (interceptor.Interceptor, error) { return mockInterceptor, nil },
	})

	sender, receiver, err := NewAPI(WithInterceptorRegistry(ir)).newPair(Configuration{})
	assert.NoError(t, err)

	track, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: MimeTypeVP8}, "video", "pion")
	assert.NoError(t, err)

	_, err = sender.AddTrack(track)
	assert.NoError(t, err)

	// Signal and wait until the receiver fires OnTrack (stream is negotiated + receiving)
	trackReceived, trackReceivedCancel := context.WithCancel(context.Background())
	receiver.OnTrack(func(_ *TrackRemote, _ *RTPReceiver) {
		trackReceivedCancel()
	})

	assert.NoError(t, signalPair(sender, receiver))

	// Send samples until the receiver sees the track (RTX SSRC gets registered during Receive)
	func() {
		ticker := time.NewTicker(time.Millisecond * 20)
		defer ticker.Stop()
		for {
			select {
			case <-trackReceived.Done():
				return
			case <-ticker.C:
				assert.NoError(t, track.WriteSample(media.Sample{Data: []byte{0xAA}, Duration: time.Second}))
			}
		}
	}()

	// Assert: exactly one bound stream must have MimeType == MimeTypeRTX
	count := 0
	for _, info := range boundStreamInfos {
		if info.MimeType == MimeTypeRTX {
			count++
		}
	}
	assert.Equal(t, 1, count,
		"expected exactly one RTX StreamInfo with MimeType %q, got %d (all types: %v)",
		MimeTypeRTX, count, mimeTypes(boundStreamInfos))

	closePairNow(t, sender, receiver)
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
