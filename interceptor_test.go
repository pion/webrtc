// +build !js

package webrtc

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pion/interceptor"
	"github.com/pion/rtcp"
	"github.com/pion/rtp"
	"github.com/pion/transport/test"
	"github.com/pion/webrtc/v3/pkg/media"
	"github.com/stretchr/testify/assert"
)

type testInterceptor struct {
	t           *testing.T
	extensionID uint8
	rtcpWriter  atomic.Value
	lastRTCP    atomic.Value
	interceptor.NoOp
}

func (t *testInterceptor) BindLocalStream(_ *interceptor.StreamInfo, writer interceptor.RTPWriter) interceptor.RTPWriter {
	return interceptor.RTPWriterFunc(func(ctx context.Context, p *rtp.Packet, attributes interceptor.Attributes) (int, error) {
		// set extension on outgoing packet
		p.Header.Extension = true
		p.Header.ExtensionProfile = 0xBEDE
		assert.NoError(t.t, p.Header.SetExtension(t.extensionID, []byte("write")))

		return writer.Write(ctx, p, attributes)
	})
}

func (t *testInterceptor) BindRemoteStream(info *interceptor.StreamInfo, reader interceptor.RTPReader) interceptor.RTPReader {
	return interceptor.RTPReaderFunc(func(ctx context.Context) (*rtp.Packet, interceptor.Attributes, error) {
		p, attributes, err := reader.Read(ctx)
		if err != nil {
			return nil, nil, err
		}
		// set extension on incoming packet
		p.Header.Extension = true
		p.Header.ExtensionProfile = 0xBEDE
		assert.NoError(t.t, p.Header.SetExtension(t.extensionID, []byte("read")))

		// write back a pli
		rtcpWriter := t.rtcpWriter.Load().(interceptor.RTCPWriter)
		pli := &rtcp.PictureLossIndication{SenderSSRC: info.SSRC, MediaSSRC: info.SSRC}
		_, err = rtcpWriter.Write(ctx, []rtcp.Packet{pli}, make(interceptor.Attributes))
		assert.NoError(t.t, err)

		return p, attributes, nil
	})
}

func (t *testInterceptor) BindRTCPReader(reader interceptor.RTCPReader) interceptor.RTCPReader {
	return interceptor.RTCPReaderFunc(func(ctx context.Context) ([]rtcp.Packet, interceptor.Attributes, error) {
		pkts, attributes, err := reader.Read(ctx)
		if err != nil {
			return nil, nil, err
		}

		t.lastRTCP.Store(pkts[0])

		return pkts, attributes, nil
	})
}

func (t *testInterceptor) lastReadRTCP() rtcp.Packet {
	p, _ := t.lastRTCP.Load().(rtcp.Packet)
	return p
}

func (t *testInterceptor) BindRTCPWriter(writer interceptor.RTCPWriter) interceptor.RTCPWriter {
	t.rtcpWriter.Store(writer)
	return writer
}

func TestPeerConnection_Interceptor(t *testing.T) {
	to := test.TimeOut(time.Second * 20)
	defer to.Stop()

	report := test.CheckRoutines(t)
	defer report()

	createPC := func(i interceptor.Interceptor) *PeerConnection {
		m := &MediaEngine{}
		err := m.RegisterDefaultCodecs()
		if err != nil {
			t.Fatal(err)
		}
		ir := &interceptor.Registry{}
		ir.Add(i)
		pc, err := NewAPI(WithMediaEngine(m), WithInterceptorRegistry(ir)).NewPeerConnection(Configuration{})
		if err != nil {
			t.Fatal(err)
		}

		return pc
	}

	sendInterceptor := &testInterceptor{t: t, extensionID: 1}
	senderPC := createPC(sendInterceptor)
	receiverPC := createPC(&testInterceptor{t: t, extensionID: 2})

	track, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: "video/vp8"}, "video", "pion")
	if err != nil {
		t.Fatal(err)
	}

	sender, err := senderPC.AddTrack(track)
	if err != nil {
		t.Fatal(err)
	}

	pending := new(int32)
	wg := &sync.WaitGroup{}

	wg.Add(1)
	*pending++
	receiverPC.OnTrack(func(track *TrackRemote, receiver *RTPReceiver) {
		p, readErr := track.ReadRTP(context.Background())
		if readErr != nil {
			t.Fatal(readErr)
		}
		assert.Equal(t, p.Extension, true)
		assert.Equal(t, "write", string(p.GetExtension(1)))
		assert.Equal(t, "read", string(p.GetExtension(2)))
		atomic.AddInt32(pending, -1)
		wg.Done()

		for {
			_, readErr = track.ReadRTP(context.Background())
			if readErr != nil {
				return
			}
		}
	})

	wg.Add(1)
	*pending++
	go func() {
		_, readErr := sender.ReadRTCP(context.Background())
		assert.NoError(t, readErr)
		atomic.AddInt32(pending, -1)
		wg.Done()

		for {
			_, readErr = sender.ReadRTCP(context.Background())
			if readErr != nil {
				return
			}
		}
	}()

	err = signalPair(senderPC, receiverPC)
	if err != nil {
		t.Fatal(err)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			time.Sleep(time.Millisecond * 100)
			if routineErr := track.WriteSample(
				context.Background(), media.Sample{Data: []byte{0x00}, Duration: time.Second},
			); routineErr != nil {
				t.Error(routineErr)
				return
			}

			if atomic.LoadInt32(pending) == 0 {
				return
			}
		}
	}()

	wg.Wait()
	assert.NoError(t, senderPC.Close())
	assert.NoError(t, receiverPC.Close())

	pli, _ := sendInterceptor.lastReadRTCP().(*rtcp.PictureLossIndication)
	if pli == nil || pli.SenderSSRC == 0 {
		t.Errorf("pli not found by send interceptor")
	}
}
