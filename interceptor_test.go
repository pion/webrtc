// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js
// +build !js

package webrtc

//
import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pion/interceptor"
	mock_interceptor "github.com/pion/interceptor/pkg/mock"
	"github.com/pion/rtp"
	"github.com/pion/transport/v2/test"
	"github.com/pion/webrtc/v3/pkg/media"
	"github.com/stretchr/testify/assert"
)

// E2E test of the features of Interceptors
// * Assert an extension can be set on an outbound packet
// * Assert an extension can be read on an outbound packet
// * Assert that attributes set by an interceptor are returned to the Reader
func TestPeerConnection_Interceptor(t *testing.T) {
	to := test.TimeOut(time.Second * 20)
	defer to.Stop()

	report := test.CheckRoutines(t)
	defer report()

	createPC := func() *PeerConnection {
		m := &MediaEngine{}
		assert.NoError(t, m.RegisterDefaultCodecs())

		ir := &interceptor.Registry{}
		ir.Add(&mock_interceptor.Factory{
			NewInterceptorFn: func(_ string) (interceptor.Interceptor, error) {
				return &mock_interceptor.Interceptor{
					BindLocalStreamFn: func(_ *interceptor.StreamInfo, writer interceptor.RTPWriter) interceptor.RTPWriter {
						return interceptor.RTPWriterFunc(func(header *rtp.Header, payload []byte, attributes interceptor.Attributes) (int, error) {
							// set extension on outgoing packet
							header.Extension = true
							header.ExtensionProfile = 0xBEDE
							assert.NoError(t, header.SetExtension(2, []byte("foo")))

							return writer.Write(header, payload, attributes)
						})
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

		pc, err := NewAPI(WithMediaEngine(m), WithInterceptorRegistry(ir)).NewPeerConnection(Configuration{})
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
	answerer.OnTrack(func(track *TrackRemote, receiver *RTPReceiver) {
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

func Test_Interceptor_BindUnbind(t *testing.T) {
	lim := test.TimeOut(time.Second * 10)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	m := &MediaEngine{}
	assert.NoError(t, m.RegisterDefaultCodecs())

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
		BindLocalStreamFn: func(i *interceptor.StreamInfo, writer interceptor.RTPWriter) interceptor.RTPWriter {
			atomic.AddUint32(&cntBindLocalStream, 1)
			return writer
		},
		UnbindLocalStreamFn: func(i *interceptor.StreamInfo) {
			atomic.AddUint32(&cntUnbindLocalStream, 1)
		},
		BindRemoteStreamFn: func(i *interceptor.StreamInfo, reader interceptor.RTPReader) interceptor.RTPReader {
			atomic.AddUint32(&cntBindRemoteStream, 1)
			return reader
		},
		UnbindRemoteStreamFn: func(i *interceptor.StreamInfo) {
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

	sender, receiver, err := NewAPI(WithMediaEngine(m), WithInterceptorRegistry(ir)).newPair(Configuration{})
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

	closePairNow(t, sender, receiver)

	// Bind/UnbindLocal/RemoteStream should be called from one side.
	if cnt := atomic.LoadUint32(&cntBindLocalStream); cnt != 1 {
		t.Errorf("BindLocalStreamFn is expected to be called once, but called %d times", cnt)
	}
	if cnt := atomic.LoadUint32(&cntUnbindLocalStream); cnt != 1 {
		t.Errorf("UnbindLocalStreamFn is expected to be called once, but called %d times", cnt)
	}
	if cnt := atomic.LoadUint32(&cntBindRemoteStream); cnt != 1 {
		t.Errorf("BindRemoteStreamFn is expected to be called once, but called %d times", cnt)
	}
	if cnt := atomic.LoadUint32(&cntUnbindRemoteStream); cnt != 1 {
		t.Errorf("UnbindRemoteStreamFn is expected to be called once, but called %d times", cnt)
	}

	// BindRTCPWriter/Reader and Close should be called from both side.
	if cnt := atomic.LoadUint32(&cntBindRTCPWriter); cnt != 2 {
		t.Errorf("BindRTCPWriterFn is expected to be called twice, but called %d times", cnt)
	}
	if cnt := atomic.LoadUint32(&cntBindRTCPReader); cnt != 2 {
		t.Errorf("BindRTCPReaderFn is expected to be called twice, but called %d times", cnt)
	}
	if cnt := atomic.LoadUint32(&cntClose); cnt != 2 {
		t.Errorf("CloseFn is expected to be called twice, but called %d times", cnt)
	}
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

	peerConnectionA, err := NewAPI(WithInterceptorRegistry(ir)).NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	peerConnectionB, err := NewAPI(WithInterceptorRegistry(ir)).NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	assert.Equal(t, 2, registryBuildCount)
	closePairNow(t, peerConnectionA, peerConnectionB)
}
