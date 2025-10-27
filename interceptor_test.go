// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js
// +build !js

package webrtc

//
import (
	"context"
	"fmt"
	"io"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pion/interceptor"
	mock_interceptor "github.com/pion/interceptor/pkg/mock"
	"github.com/pion/rtcp"
	"github.com/pion/rtp"
	"github.com/pion/transport/v3/test"
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
					BindRemoteStreamFn: func(_ *interceptor.StreamInfo, reader interceptor.RTPProcessor) interceptor.RTPProcessor {
						return interceptor.RTPProcessorFunc(func(i int, b []byte, a interceptor.Attributes) (int, interceptor.Attributes, error) {
							if a == nil {
								a = interceptor.Attributes{}
							}

							a.Set("attribute", "value")

							return reader.Process(i, b, a)
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
		BindRemoteStreamFn: func(_ *interceptor.StreamInfo, reader interceptor.RTPProcessor) interceptor.RTPProcessor {
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

	assert.NoError(t, RegisterDefaultInterceptors(mediaEngine, interceptorRegistry))

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
	getter, ok := lookupStats(pc.statsID)
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

	statsID := pc.statsID
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

	var gotNack bool
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
					gotNack = true
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

	pcOfferConnected := make(chan struct{})
	pcAnswerConnected := make(chan struct{})

	pc1.OnConnectionStateChange(func(state PeerConnectionState) {
		if state == PeerConnectionStateConnected {
			close(pcOfferConnected)
		}
	})

	pc2.OnConnectionStateChange(func(state PeerConnectionState) {
		if state == PeerConnectionStateConnected {
			close(pcAnswerConnected)
		}
	})

	<-pcOfferConnected
	<-pcAnswerConnected

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
		assert.True(t, gotNack, "Expected to get a NACK, got none")
	} else {
		assert.False(t, gotNack, "Expected to get no NACK, got one")
	}
}
