// +build !js

package webrtc

import (
	"bytes"
	"context"
	"errors"
	"io"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pion/transport/test"
	"github.com/pion/webrtc/v3/pkg/media"
	"github.com/stretchr/testify/assert"
)

func Test_RTPSender_ReplaceTrack(t *testing.T) {
	lim := test.TimeOut(time.Second * 10)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	t.Run("Basic", func(t *testing.T) {
		s := SettingEngine{}
		s.DisableSRTPReplayProtection(true)

		m := &MediaEngine{}
		assert.NoError(t, m.RegisterDefaultCodecs())

		sender, receiver, err := NewAPI(WithMediaEngine(m), WithSettingEngine(s)).newPair(Configuration{})
		assert.NoError(t, err)

		trackA, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: "video/vp8"}, "video", "pion")
		assert.NoError(t, err)

		trackB, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: "video/vp8"}, "video", "pion")
		assert.NoError(t, err)

		rtpSender, err := sender.AddTrack(trackA)
		assert.NoError(t, err)

		seenPacketA, seenPacketACancel := context.WithCancel(context.Background())
		seenPacketB, seenPacketBCancel := context.WithCancel(context.Background())

		var onTrackCount uint64
		receiver.OnTrack(func(track *TrackRemote, _ *RTPReceiver) {
			assert.Equal(t, uint64(1), atomic.AddUint64(&onTrackCount, 1))

			for {
				pkt, err := track.ReadRTP()
				if err != nil {
					assert.True(t, errors.Is(io.EOF, err))
					return
				}

				switch {
				case bytes.Equal(pkt.Payload, []byte{0x10, 0xAA}):
					seenPacketACancel()
				case bytes.Equal(pkt.Payload, []byte{0x10, 0xBB}):
					seenPacketBCancel()
				}
			}
		})

		assert.NoError(t, signalPair(sender, receiver))

		// Block Until packet with 0xAA has been seen
		func() {
			for range time.Tick(time.Millisecond * 20) {
				select {
				case <-seenPacketA.Done():
					return
				default:
					assert.NoError(t, trackA.WriteSample(media.Sample{Data: []byte{0xAA}, Duration: time.Second}))
				}
			}
		}()

		assert.NoError(t, rtpSender.ReplaceTrack(trackB))

		// Block Until packet with 0xBB has been seen
		func() {
			for range time.Tick(time.Millisecond * 20) {
				select {
				case <-seenPacketB.Done():
					return
				default:
					assert.NoError(t, trackB.WriteSample(media.Sample{Data: []byte{0xBB}, Duration: time.Second}))
				}
			}
		}()

		assert.NoError(t, sender.Close())
		assert.NoError(t, receiver.Close())
	})

	t.Run("Invalid Codec Change", func(t *testing.T) {
		sender, receiver, err := newPair()
		assert.NoError(t, err)

		trackA, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: "video/vp8"}, "video", "pion")
		assert.NoError(t, err)

		trackB, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: "video/h264", SDPFmtpLine: "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42001f"}, "video", "pion")
		assert.NoError(t, err)

		rtpSender, err := sender.AddTrack(trackA)
		assert.NoError(t, err)

		assert.NoError(t, signalPair(sender, receiver))

		seenPacket, seenPacketCancel := context.WithCancel(context.Background())
		receiver.OnTrack(func(_ *TrackRemote, _ *RTPReceiver) {
			seenPacketCancel()
		})

		func() {
			for range time.Tick(time.Millisecond * 20) {
				select {
				case <-seenPacket.Done():
					return
				default:
					assert.NoError(t, trackA.WriteSample(media.Sample{Data: []byte{0xAA}, Duration: time.Second}))
				}
			}
		}()

		assert.True(t, errors.Is(rtpSender.ReplaceTrack(trackB), ErrUnsupportedCodec))

		assert.NoError(t, sender.Close())
		assert.NoError(t, receiver.Close())
	})
}
