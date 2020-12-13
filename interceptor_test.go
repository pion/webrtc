// +build !js

package webrtc

//
import (
	"context"
	"testing"
	"time"

	"github.com/pion/interceptor"
	"github.com/pion/rtp"
	"github.com/pion/transport/test"
	"github.com/pion/webrtc/v3/pkg/media"
	"github.com/stretchr/testify/assert"
)

type testInterceptor struct {
	interceptor.NoOp

	t *testing.T
}

func (t *testInterceptor) BindLocalStream(_ *interceptor.StreamInfo, writer interceptor.RTPWriter) interceptor.RTPWriter {
	return interceptor.RTPWriterFunc(func(header *rtp.Header, payload []byte, attributes interceptor.Attributes) (int, error) {
		// set extension on outgoing packet
		header.Extension = true
		header.ExtensionProfile = 0xBEDE
		assert.NoError(t.t, header.SetExtension(2, []byte("foo")))

		return writer.Write(header, payload, attributes)
	})
}

func (t *testInterceptor) BindRemoteStream(_ *interceptor.StreamInfo, reader interceptor.RTPReader) interceptor.RTPReader {
	return interceptor.RTPReaderFunc(func(b []byte, a interceptor.Attributes) (int, interceptor.Attributes, error) {
		if a == nil {
			a = interceptor.Attributes{}
		}

		a.Set("attribute", "value")
		return reader.Read(b, a)
	})
}

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
		ir.Add(&testInterceptor{t: t})

		pc, err := NewAPI(WithMediaEngine(m), WithInterceptorRegistry(ir)).NewPeerConnection(Configuration{})
		assert.NoError(t, err)

		return pc
	}

	offerer := createPC()
	answerer := createPC()

	track, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: "video/vp8"}, "video", "pion")
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

	assert.NoError(t, offerer.Close())
	assert.NoError(t, answerer.Close())
}
