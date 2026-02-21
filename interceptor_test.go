// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js
// +build !js

package webrtc

//
import (
	"context"
	"errors"
	"fmt"
	"io"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pion/interceptor"
	mock_interceptor "github.com/pion/interceptor/pkg/mock"
	"github.com/pion/logging"
	"github.com/pion/rtcp"
	"github.com/pion/rtp"
	"github.com/pion/transport/v4/test"
	"github.com/pion/transport/v4/vnet"
	"github.com/pion/webrtc/v4/pkg/media"
	"github.com/stretchr/testify/assert"
)

// E2E test of the features of Interceptors
// * Assert an extension can be set on an outbound packet
// * Assert an extension can be read on an outbound packet
// * Assert that attributes set by an interceptor are returned to the Reader.
func TestPeerConnection_Interceptor(t *testing.T) {
	to := test.TimeOut(time.Second * 20)
	defer to.Stop()

	report := test.CheckRoutines(t)
	defer report()

	createPC := func() *PeerConnection {
		ir := &interceptor.Registry{}
		ir.Add(&mock_interceptor.Factory{
			NewInterceptorFn: func(_ string) (interceptor.Interceptor, error) {
				return &mock_interceptor.Interceptor{
					BindLocalStreamFn: func(_ *interceptor.StreamInfo, writer interceptor.RTPWriter) interceptor.RTPWriter {
						return interceptor.RTPWriterFunc(
							func(header *rtp.Header, payload []byte, attributes interceptor.Attributes) (int, error) {
								// set extension on outgoing packet
								header.Extension = true
								header.ExtensionProfile = 0xBEDE
								assert.NoError(t, header.SetExtension(2, []byte("foo")))

								return writer.Write(header, payload, attributes)
							},
						)
					},
					BindRemoteStreamFn: func(_ *interceptor.StreamInfo, reader interceptor.RTPReader) interceptor.RTPReader {
						return interceptor.RTPReaderFunc(func(b []byte, a interceptor.Attributes) (int, interceptor.Attributes, error) {
							if a == nil {
								a = interceptor.Attributes{}
							}

							a.Set("attribute", "value")

							return reader.Read(b, a)
						})
					},
				}, nil
			},
		})

		pc, err := NewAPI(WithInterceptorRegistry(ir)).NewPeerConnection(Configuration{})
		assert.NoError(t, err)

		return pc
	}

	offerer := createPC()
	answerer := createPC()

	track, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: MimeTypeVP8}, "video", "pion")
	assert.NoError(t, err)

	_, err = offerer.AddTrack(track)
	assert.NoError(t, err)

	seenRTP, seenRTPCancel := context.WithCancel(context.Background())
	answerer.OnTrack(func(track *TrackRemote, _ *RTPReceiver) {
		p, attributes, readErr := track.ReadRTP()
		assert.NoError(t, readErr)

		assert.Equal(t, p.Extension, true)
		assert.Equal(t, "foo", string(p.GetExtension(2)))
		assert.Equal(t, "value", attributes.Get("attribute"))

		seenRTPCancel()
	})

	assert.NoError(t, signalPair(offerer, answerer))

	func() {
		ticker := time.NewTicker(time.Millisecond * 20)
		defer ticker.Stop()
		for {
			select {
			case <-seenRTP.Done():
				return
			case <-ticker.C:
				assert.NoError(t, track.WriteSample(media.Sample{Data: []byte{0x00}, Duration: time.Second}))
			}
		}
	}()

	closePairNow(t, offerer, answerer)
}

func Test_Interceptor_BindUnbind(t *testing.T) { //nolint:cyclop
	lim := test.TimeOut(time.Second * 10)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	var (
		cntBindRTCPReader     uint32
		cntBindRTCPWriter     uint32
		cntBindLocalStream    uint32
		cntUnbindLocalStream  uint32
		cntBindRemoteStream   uint32
		cntUnbindRemoteStream uint32
		cntClose              uint32
	)
	mockInterceptor := &mock_interceptor.Interceptor{
		BindRTCPReaderFn: func(reader interceptor.RTCPReader) interceptor.RTCPReader {
			atomic.AddUint32(&cntBindRTCPReader, 1)

			return reader
		},
		BindRTCPWriterFn: func(writer interceptor.RTCPWriter) interceptor.RTCPWriter {
			atomic.AddUint32(&cntBindRTCPWriter, 1)

			return writer
		},
		BindLocalStreamFn: func(_ *interceptor.StreamInfo, writer interceptor.RTPWriter) interceptor.RTPWriter {
			atomic.AddUint32(&cntBindLocalStream, 1)

			return writer
		},
		UnbindLocalStreamFn: func(*interceptor.StreamInfo) {
			atomic.AddUint32(&cntUnbindLocalStream, 1)
		},
		BindRemoteStreamFn: func(_ *interceptor.StreamInfo, reader interceptor.RTPReader) interceptor.RTPReader {
			atomic.AddUint32(&cntBindRemoteStream, 1)

			return reader
		},
		UnbindRemoteStreamFn: func(_ *interceptor.StreamInfo) {
			atomic.AddUint32(&cntUnbindRemoteStream, 1)
		},
		CloseFn: func() error {
			atomic.AddUint32(&cntClose, 1)

			return nil
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

	receiverReady, receiverReadyFn := context.WithCancel(context.Background())
	receiver.OnTrack(func(track *TrackRemote, _ *RTPReceiver) {
		_, _, readErr := track.ReadRTP()
		assert.NoError(t, readErr)
		receiverReadyFn()
	})

	assert.NoError(t, signalPair(sender, receiver))

	ticker := time.NewTicker(time.Millisecond * 20)
	defer ticker.Stop()
	func() {
		for {
			select {
			case <-receiverReady.Done():
				return
			case <-ticker.C:
				// Send packet to make receiver track actual creates RTPReceiver.
				assert.NoError(t, track.WriteSample(media.Sample{Data: []byte{0xAA}, Duration: time.Second}))
			}
		}
	}()

	assert.NoError(t, sender.GracefulClose())
	assert.NoError(t, receiver.GracefulClose())

	// Bind/UnbindLocal/RemoteStream should be called from one side.
	assert.Equal(t, uint32(1), atomic.LoadUint32(&cntBindLocalStream), "BindLocalStreamFn is expected to be called once")
	assert.Equal(
		t, uint32(1), atomic.LoadUint32(&cntUnbindLocalStream), "UnbindLocalStreamFn is expected to be called once",
	)
	assert.Equal(
		t, uint32(2), atomic.LoadUint32(&cntBindRemoteStream), "BindRemoteStreamFn is expected to be called twice",
	)
	assert.Equal(
		t, uint32(2), atomic.LoadUint32(&cntUnbindRemoteStream), "UnbindRemoteStreamFn is expected to be called twice",
	)

	// BindRTCPWriter/Reader and Close should be called from both side.
	assert.Equal(t, uint32(2), atomic.LoadUint32(&cntBindRTCPWriter), "BindRTCPWriterFn is expected to be called twice")
	assert.Equal(t, uint32(3), atomic.LoadUint32(&cntBindRTCPReader), "BindRTCPReaderFn is expected to be called thrice")
	assert.Equal(t, uint32(2), atomic.LoadUint32(&cntClose), "CloseFn is expected to be called twice")
}

func Test_InterceptorRegistry_Build(t *testing.T) {
	registryBuildCount := 0

	ir := &interceptor.Registry{}
	ir.Add(&mock_interceptor.Factory{
		NewInterceptorFn: func(_ string) (interceptor.Interceptor, error) {
			registryBuildCount++

			return &interceptor.NoOp{}, nil
		},
	})

	peerConnectionA, peerConnectionB, err := NewAPI(WithInterceptorRegistry(ir)).newPair(Configuration{})
	assert.NoError(t, err)

	assert.Equal(t, 2, registryBuildCount)
	closePairNow(t, peerConnectionA, peerConnectionB)
}

// TestConfigureFlexFEC03_FECParameters tests only that FEC parameters are correctly set and that SDP contains FEC info.
// FEC between 2 Pion clients is not currently supported and cannot be negotiated due to the blocking issue:
// https://github.com/pion/webrtc/issues/3109
func TestConfigureFlexFEC03_FECParameters(t *testing.T) {
	to := test.TimeOut(time.Second * 20)
	defer to.Stop()

	report := test.CheckRoutines(t)
	defer report()

	mediaEngine := &MediaEngine{}

	assert.NoError(t, mediaEngine.RegisterCodec(RTPCodecParameters{
		RTPCodecCapability: RTPCodecCapability{MimeType: MimeTypeVP8, ClockRate: 90000},
		PayloadType:        96,
	}, RTPCodecTypeVideo))

	interceptorRegistry := &interceptor.Registry{}

	fecPayloadType := PayloadType(120)
	assert.NoError(t, ConfigureFlexFEC03(fecPayloadType, mediaEngine, interceptorRegistry))

	// Register default interceptors. Options are passed to make codecov happy.
	assert.NoError(t, RegisterDefaultInterceptorsWithOptions(mediaEngine, interceptorRegistry,
		WithInterceptorLoggerFactory(logging.NewDefaultLoggerFactory()),
		WithNackGeneratorOptions(),
		WithNackResponderOptions(),
		WithReportReceiverOptions(),
		WithReportSenderOptions(),
		WithStatsInterceptorOptions(),
		WithTWCCOptions(),
	))

	api := NewAPI(WithMediaEngine(mediaEngine), WithInterceptorRegistry(interceptorRegistry))

	pc, err := api.NewPeerConnection(Configuration{})
	assert.NoError(t, err)
	defer func() { assert.NoError(t, pc.Close()) }()

	track, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: MimeTypeVP8}, "video", "pion")
	assert.NoError(t, err)

	sender, err := pc.AddTrack(track)
	assert.NoError(t, err)

	offer, err := pc.CreateOffer(nil)
	assert.NoError(t, err)

	assert.Contains(t, offer.SDP, "a=rtpmap:120 flexfec-03/90000")

	assert.NoError(t, pc.SetLocalDescription(offer))

	params := sender.GetParameters()
	assert.NotZero(t, params.Encodings[0].FEC.SSRC, "FEC SSRC should be non-zero")

	expectedFECGroup := fmt.Sprintf("FEC-FR %d %d", params.Encodings[0].SSRC, params.Encodings[0].FEC.SSRC)
	assert.Contains(t, offer.SDP, expectedFECGroup, "SDP should contain FEC-FR ssrc-group")

	var fecCodecFound bool
	for _, codec := range params.Codecs {
		if codec.MimeType == MimeTypeFlexFEC03 && codec.PayloadType == fecPayloadType {
			fecCodecFound = true
			assert.Equal(t, uint32(90000), codec.ClockRate)
			assert.Equal(t, "repair-window=10000000", codec.SDPFmtpLine)

			break
		}
	}
	assert.True(t, fecCodecFound, "FlexFEC-03 codec should be registered")
}

func Test_Interceptor_ZeroSSRC(t *testing.T) {
	to := test.TimeOut(time.Second * 20)
	defer to.Stop()

	report := test.CheckRoutines(t)
	defer report()

	track, err := NewTrackLocalStaticRTP(RTPCodecCapability{MimeType: MimeTypeVP8}, "video", "pion")
	assert.NoError(t, err)

	offerer, answerer, err := newPair()
	assert.NoError(t, err)

	_, err = offerer.AddTrack(track)
	assert.NoError(t, err)

	probeReceiverCreated := make(chan struct{})

	go func() {
		sequenceNumber := uint16(0)
		ticker := time.NewTicker(time.Millisecond * 20)
		defer ticker.Stop()
		for range ticker.C {
			track.mu.Lock()
			if len(track.bindings) == 1 {
				_, err = track.bindings[0].writeStream.WriteRTP(&rtp.Header{
					Version:        2,
					SSRC:           0,
					SequenceNumber: sequenceNumber,
				}, []byte{0, 1, 2, 3, 4, 5})
				assert.NoError(t, err)
			}
			sequenceNumber++
			track.mu.Unlock()

			if nonMediaBandwidthProbe, ok := answerer.nonMediaBandwidthProbe.Load().(*RTPReceiver); ok {
				assert.Equal(t, len(nonMediaBandwidthProbe.Tracks()), 1)
				close(probeReceiverCreated)

				return
			}
		}
	}()

	assert.NoError(t, signalPair(offerer, answerer))

	peerConnectionConnected := untilConnectionState(PeerConnectionStateConnected, offerer, answerer)
	peerConnectionConnected.Wait()

	<-probeReceiverCreated
	closePairNow(t, offerer, answerer)
}

// TestStatsInterceptorIsAddedByDefault tests that the stats interceptor
// is automatically added when creating a PeerConnection with the default API
// and that its Getter is properly captured.
func TestStatsInterceptorIsAddedByDefault(t *testing.T) {
	lim := test.TimeOut(time.Second * 10)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	pc, err := NewPeerConnection(Configuration{})
	assert.NoError(t, err)
	defer func() {
		assert.NoError(t, pc.Close())
	}()

	assert.NotNil(t, pc.statsGetter, "statsGetter should be non-nil with NewPeerConnection")

	// Also assert that the getter stored during interceptor Build matches
	// the one attached to this PeerConnection.
	getter, ok := lookupStats(pc.id)
	assert.True(t, ok, "lookupStats should return a getter for this statsID")
	assert.NotNil(t, getter)
	assert.Equal(t,
		reflect.ValueOf(getter).Pointer(),
		reflect.ValueOf(pc.statsGetter).Pointer(),
		"getter returned by lookup should match pc.statsGetter",
	)
}

// TestStatsGetterCleanup tests that statsGetter is properly cleaned up to prevent memory leaks.
func TestStatsGetterCleanup(t *testing.T) {
	api := NewAPI()
	pc, err := api.NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	assert.NotNil(t, pc.statsGetter, "statsGetter should be non-nil after creation")

	statsID := pc.id
	getter, exists := lookupStats(statsID)
	assert.True(t, exists, "global statsGetter map should contain entry for this PC")
	assert.NotNil(t, getter, "looked up getter should not be nil")
	assert.Equal(t, pc.statsGetter, getter, "field and global map getter should match")

	assert.NoError(t, pc.Close())

	assert.Nil(t, pc.statsGetter, "statsGetter field should be nil after close")

	getter, exists = lookupStats(statsID)
	assert.False(t, exists, "global statsGetter map should not contain entry after close")
	assert.Nil(t, getter, "looked up getter should be nil after close")
}

// TestInterceptorNack is an end-to-end test for the NACK sender.
// It tests that:
//   - we get a NACK if we negotiated generic NACks;
//   - we don't get a NACK if we did not negotiate generick NACKs;
//   - the NACK corresponds to the missing packet.
func TestInterceptorNack(t *testing.T) {
	to := test.TimeOut(time.Second * 20)
	defer to.Stop()

	t.Run("Nack", func(t *testing.T) { testInterceptorNack(t, true) })
	t.Run("NoNack", func(t *testing.T) { testInterceptorNack(t, false) })
}

func testInterceptorNack(t *testing.T, requestNack bool) { //nolint:cyclop
	t.Helper()

	const numPackets = 20

	ir := interceptor.Registry{}
	mediaEngine := MediaEngine{}
	var feedback []RTCPFeedback
	if requestNack {
		feedback = append(feedback, RTCPFeedback{"nack", ""})
	}
	err := mediaEngine.RegisterCodec(
		RTPCodecParameters{
			RTPCodecCapability: RTPCodecCapability{
				"video/VP8", 90000, 0,
				"",
				feedback,
			},
			PayloadType: 96,
		},
		RTPCodecTypeVideo,
	)
	assert.NoError(t, err)
	api := NewAPI(
		WithMediaEngine(&mediaEngine),
		WithInterceptorRegistry(&ir),
	)

	pc1, err := api.NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	pc1Connected := make(chan struct{})
	pc1.OnConnectionStateChange(func(state PeerConnectionState) {
		if state == PeerConnectionStateConnected {
			close(pc1Connected)
		}
	})

	track1, err := NewTrackLocalStaticRTP(
		RTPCodecCapability{MimeType: MimeTypeVP8},
		"video", "pion",
	)
	assert.NoError(t, err)
	sender, err := pc1.AddTrack(track1)
	assert.NoError(t, err)

	pc2, err := NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	offer, err := pc1.CreateOffer(nil)
	assert.NoError(t, err)
	err = pc1.SetLocalDescription(offer)
	assert.NoError(t, err)
	<-GatheringCompletePromise(pc1)

	err = pc2.SetRemoteDescription(*pc1.LocalDescription())
	assert.NoError(t, err)
	answer, err := pc2.CreateAnswer(nil)
	assert.NoError(t, err)
	err = pc2.SetLocalDescription(answer)
	assert.NoError(t, err)
	<-GatheringCompletePromise(pc2)

	err = pc1.SetRemoteDescription(*pc2.LocalDescription())
	assert.NoError(t, err)

	<-pc1Connected

	var gotNack atomic.Bool
	rtcpDone := make(chan struct{})
	go func() {
		defer close(rtcpDone)
		buf := make([]byte, 1500)
		for {
			n, _, err2 := sender.Read(buf)
			// nolint
			if err2 == io.EOF {
				break
			}
			assert.NoError(t, err2)
			ps, err2 := rtcp.Unmarshal(buf[:n])
			assert.NoError(t, err2)
			for _, p := range ps {
				if pn, ok := p.(*rtcp.TransportLayerNack); ok {
					assert.Equal(t, len(pn.Nacks), 1)
					assert.Equal(t,
						pn.Nacks[0].PacketID, uint16(1),
					)
					assert.Equal(t,
						pn.Nacks[0].LostPackets,
						rtcp.PacketBitmap(0),
					)
					gotNack.Store(true)
				}
			}
		}
	}()

	done := make(chan struct{})
	pc2.OnTrack(func(track2 *TrackRemote, _ *RTPReceiver) {
		for i := 0; i < numPackets; i++ {
			if i == 1 {
				continue
			}
			p, _, err2 := track2.ReadRTP()
			assert.NoError(t, err2)
			assert.Equal(t, p.SequenceNumber, uint16(i)) //nolint:gosec //G115
		}
		close(done)
	})

	go func() {
		for i := 0; i < numPackets; i++ {
			time.Sleep(20 * time.Millisecond)
			if i == 1 {
				continue
			}
			var p rtp.Packet
			p.Version = 2
			p.Marker = true
			p.PayloadType = 96
			p.SequenceNumber = uint16(i)         //nolint:gosec // G115
			p.Timestamp = uint32(i * 90000 / 50) //nolint:gosec // G115
			p.Payload = []byte{42}
			err2 := track1.WriteRTP(&p)
			assert.NoError(t, err2)
		}
	}()

	<-done
	err = pc1.Close()
	assert.NoError(t, err)
	err = pc2.Close()
	assert.NoError(t, err)

	if requestNack {
		assert.True(t, gotNack.Load(), "Expected to get a NACK, got none")
	} else {
		assert.False(t, gotNack.Load(), "Expected to get no NACK, got one")
	}
}

// Verifies correct NACK/RTX behavior and reproduces the scenario from
// Pion Issue #3063. The second RTP packet is intentionally dropped; the test
// expects exactly one NACK and one RTX for this lost packet. After both events
// occur, a short grace period ensures no duplicate NACK/RTX messages appear.
// Any additional NACK or RTX triggers a test failure.
func TestNackTriggersSingleRTX(t *testing.T) { //nolint:cyclop
	t.Skip()
	defer test.TimeOut(time.Second * 10).Stop()

	type RTXTestState struct {
		sync.Mutex

		lostSeq   uint16
		nackCount int
		rtxCount  int

		inGrace    bool
		isClosed   bool
		graceTimer chan struct{}
		done       chan struct{}

		errorMessage string
	}

	state := &RTXTestState{
		done:       make(chan struct{}, 1),
		graceTimer: make(chan struct{}, 1),
	}

	pcOffer, pcAnswer, wan := createVNetPair(t, nil)

	mediaPacketCount := 0

	wan.AddChunkFilter(func(c vnet.Chunk) bool {
		h := &rtp.Header{}
		if _, err := h.Unmarshal(c.UserData()); err != nil {
			return true
		}

		if h.PayloadType == 96 {
			state.Lock()
			mediaPacketCount++
			state.Unlock()

			if mediaPacketCount == 2 {
				state.Lock()
				state.lostSeq = h.SequenceNumber
				state.Unlock()

				return false
			}
		}

		return true
	})

	track, err := NewTrackLocalStaticSample(
		RTPCodecCapability{MimeType: MimeTypeVP8, ClockRate: 90000}, "video", "pion")
	assert.NoError(t, err)

	rtpSender, err := pcOffer.AddTrack(track)
	assert.NoError(t, err)

	startGracePeriod := func() {
		if state.inGrace {
			return
		}

		state.inGrace = true

		go func() {
			time.Sleep(500 * time.Millisecond)
			close(state.graceTimer)
		}()
	}

	triggerFailure := func(msg string) {
		if state.isClosed {
			return
		}
		state.errorMessage = msg
		state.isClosed = true
		close(state.done)
	}

	go func() {
		rtcpBuf := make([]byte, 1500)
		for {
			n, _, rtcpErr := rtpSender.Read(rtcpBuf)
			if rtcpErr != nil {
				return
			}

			ps, err2 := rtcp.Unmarshal(rtcpBuf[:n])
			assert.NoError(t, err2)

			for _, p := range ps {
				pn, ok := p.(*rtcp.TransportLayerNack)
				if !ok {
					continue
				}

				packetID := pn.Nacks[0].PacketID

				state.Lock()
				if packetID == state.lostSeq {
					state.nackCount++

					if state.nackCount == 1 && state.rtxCount == 1 && !state.inGrace {
						startGracePeriod()
					}

					if state.nackCount > 1 && state.inGrace {
						triggerFailure(
							fmt.Sprintf("received multiple NACKs for lost packet (seq=%d)", state.lostSeq))
						state.Unlock()

						return
					}
				}
				state.Unlock()
			}
		}
	}()

	pcAnswer.OnTrack(func(track *TrackRemote, _ *RTPReceiver) {
		for {
			pkt, _, readRTPErr := track.ReadRTP()
			if errors.Is(readRTPErr, io.EOF) {
				return
			} else if pkt.PayloadType == 0 {
				continue
			}

			state.Lock()
			if pkt.SequenceNumber == state.lostSeq {
				state.rtxCount++

				if state.nackCount == 1 && state.rtxCount == 1 && !state.inGrace {
					startGracePeriod()
				}

				if state.rtxCount > 1 && state.inGrace {
					triggerFailure(fmt.Sprintf(
						"received multiple RTX retransmissions for lost packet (seq=%d)", state.lostSeq))
					state.Unlock()

					return
				}
			}
			state.Unlock()
		}
	})

	assert.NoError(t, signalPair(pcOffer, pcAnswer))

	func() {
		for {
			select {
			case <-time.After(20 * time.Millisecond):
				writeErr := track.WriteSample(media.Sample{Data: []byte{0x00}, Duration: time.Second})
				assert.NoError(t, writeErr)

			case <-state.done:
				state.Lock()
				msg := state.errorMessage
				state.Unlock()

				assert.FailNow(t, msg)

				return
			case <-state.graceTimer:

				return
			}
		}
	}()

	assert.NoError(t, wan.Stop())
	closePairNow(t, pcOffer, pcAnswer)

	state.Lock()
	assert.Equal(t, 1, state.nackCount)
	assert.Equal(t, 1, state.rtxCount)
	state.Unlock()
}

// TestNackNotSentForRTX verifies that NACKs are never emitted with
// MediaSSRC equal to the RTX SSRC. See RFC 4588 section 6.3, NACKs
// MUST be sent only for the original RTP stream.
func TestNackNotSentForRTX(t *testing.T) { //nolint:cyclop
	defer test.TimeOut(time.Second * 20).Stop()
	report := test.CheckRoutines(t)
	defer report()

	const numPackets = 20

	var (
		nackCount    uint32
		badNackCount uint32
		rtxSSRC      atomic.Uint32
	)

	me := &MediaEngine{}
	assert.NoError(t, me.RegisterDefaultCodecs())
	ir := &interceptor.Registry{}
	ir.Add(&mock_interceptor.Factory{
		NewInterceptorFn: func(_ string) (interceptor.Interceptor, error) {
			return &mock_interceptor.Interceptor{
				BindRTCPWriterFn: func(writer interceptor.RTCPWriter) interceptor.RTCPWriter {
					return interceptor.RTCPWriterFunc(
						func(pkts []rtcp.Packet, attrs interceptor.Attributes) (int, error) {
							for _, p := range pkts {
								if pn, ok := p.(*rtcp.TransportLayerNack); ok {
									atomic.AddUint32(&nackCount, 1)
									if pn.MediaSSRC == rtxSSRC.Load() {
										atomic.AddUint32(&badNackCount, 1)
									}
								}
							}

							return writer.Write(pkts, attrs)
						},
					)
				},
			}, nil
		},
	})
	assert.NoError(t, RegisterDefaultInterceptors(me, ir))

	pcOffer, pcAnswer, wan := createVNetPair(t, ir)

	track, err := NewTrackLocalStaticRTP(
		RTPCodecCapability{MimeType: MimeTypeVP8}, "video", "pion",
	)
	assert.NoError(t, err)
	rtpSender, err := pcOffer.AddTrack(track)
	assert.NoError(t, err)

	assert.NoError(t, signalPair(pcOffer, pcAnswer))

	params := rtpSender.GetParameters()
	mediaPayloadType := uint8(params.Codecs[0].PayloadType)
	rtxSSRC.Store(uint32(params.Encodings[0].RTX.SSRC))

	// Drop every 4th media packet and every 2nd RTX packet.
	var mediaCount, rtxPktCount atomic.Uint32
	wan.AddChunkFilter(func(c vnet.Chunk) bool {
		h := &rtp.Header{}
		if _, parseErr := h.Unmarshal(c.UserData()); parseErr != nil {
			return true
		}
		if h.PayloadType == mediaPayloadType {
			if mediaCount.Add(1)%4 == 0 {
				return false
			}
		}
		if h.SSRC == rtxSSRC.Load() {
			if rtxPktCount.Add(1)%2 == 0 {
				return false
			}
		}

		return true
	})

	go func() {
		buf := make([]byte, 1500)
		for {
			if _, _, readErr := rtpSender.Read(buf); readErr != nil {
				return
			}
		}
	}()

	done := make(chan struct{})
	pcAnswer.OnTrack(func(remote *TrackRemote, _ *RTPReceiver) {
		for {
			if _, _, readErr := remote.ReadRTP(); readErr != nil {
				break
			}
		}
		close(done)
	})

	go func() {
		for i := 0; i < numPackets; i++ {
			time.Sleep(20 * time.Millisecond)
			var p rtp.Packet
			p.Version = 2
			p.Marker = true
			p.PayloadType = 96
			p.SequenceNumber = uint16(i)         //nolint:gosec // G115
			p.Timestamp = uint32(i * 90000 / 50) //nolint:gosec // G115
			p.Payload = []byte{42}
			assert.NoError(t, track.WriteRTP(&p))
		}
	}()

	time.Sleep(time.Second)

	assert.True(t, atomic.LoadUint32(&nackCount) > 0, "expected at least one NACK")
	assert.Equal(t, uint32(0), atomic.LoadUint32(&badNackCount),
		"NACKs must never target the RTX SSRC")

	assert.NoError(t, wan.Stop())
	closePairNow(t, pcOffer, pcAnswer)
	<-done
}
