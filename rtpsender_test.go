// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js
// +build !js

package webrtc

import (
	"context"
	"errors"
	"io"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pion/transport/v2/test"
	"github.com/pion/webrtc/v3/pkg/media"
	"github.com/stretchr/testify/assert"
)

func Test_RTPSender_ReplaceTrack(t *testing.T) {
	lim := test.TimeOut(time.Second * 10)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	s := SettingEngine{}
	s.DisableSRTPReplayProtection(true)

	m := &MediaEngine{}
	assert.NoError(t, m.RegisterDefaultCodecs())

	sender, receiver, err := NewAPI(WithMediaEngine(m), WithSettingEngine(s)).newPair(Configuration{})
	assert.NoError(t, err)

	trackA, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: MimeTypeVP8}, "video", "pion")
	assert.NoError(t, err)

	trackB, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: MimeTypeH264}, "video", "pion")
	assert.NoError(t, err)

	rtpSender, err := sender.AddTrack(trackA)
	assert.NoError(t, err)

	seenPacketA, seenPacketACancel := context.WithCancel(context.Background())
	seenPacketB, seenPacketBCancel := context.WithCancel(context.Background())

	var onTrackCount uint64
	receiver.OnTrack(func(track *TrackRemote, _ *RTPReceiver) {
		assert.Equal(t, uint64(1), atomic.AddUint64(&onTrackCount, 1))

		for {
			pkt, _, err := track.ReadRTP()
			if err != nil {
				assert.True(t, errors.Is(io.EOF, err))
				return
			}

			switch {
			case pkt.Payload[len(pkt.Payload)-1] == 0xAA:
				assert.Equal(t, track.Codec().MimeType, MimeTypeVP8)
				seenPacketACancel()
			case pkt.Payload[len(pkt.Payload)-1] == 0xBB:
				assert.Equal(t, track.Codec().MimeType, MimeTypeH264)
				seenPacketBCancel()
			default:
				t.Fatalf("Unexpected RTP Data % 02x", pkt.Payload[len(pkt.Payload)-1])
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

	closePairNow(t, sender, receiver)
}

func Test_RTPSender_GetParameters(t *testing.T) {
	lim := test.TimeOut(time.Second * 10)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	offerer, answerer, err := newPair()
	assert.NoError(t, err)

	rtpTransceiver, err := offerer.AddTransceiverFromKind(RTPCodecTypeVideo)
	assert.NoError(t, err)

	assert.NoError(t, signalPair(offerer, answerer))

	parameters := rtpTransceiver.Sender().GetParameters()
	assert.NotEqual(t, 0, len(parameters.Codecs))
	assert.Equal(t, 1, len(parameters.Encodings))
	assert.Equal(t, rtpTransceiver.Sender().trackEncodings[0].ssrc, parameters.Encodings[0].SSRC)
	assert.Equal(t, "", parameters.Encodings[0].RID)

	closePairNow(t, offerer, answerer)
}

func Test_RTPSender_GetParameters_WithRID(t *testing.T) {
	lim := test.TimeOut(time.Second * 10)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	offerer, answerer, err := newPair()
	assert.NoError(t, err)

	rtpTransceiver, err := offerer.AddTransceiverFromKind(RTPCodecTypeVideo)
	assert.NoError(t, err)

	assert.NoError(t, signalPair(offerer, answerer))

	track, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: MimeTypeVP8}, "video", "pion", WithRTPStreamID("moo"))
	assert.NoError(t, err)

	err = rtpTransceiver.setSendingTrack(track)
	assert.NoError(t, err)

	parameters := rtpTransceiver.Sender().GetParameters()
	assert.Equal(t, track.RID(), parameters.Encodings[0].RID)

	closePairNow(t, offerer, answerer)
}

func Test_RTPSender_SetReadDeadline(t *testing.T) {
	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	sender, receiver, wan := createVNetPair(t)

	track, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: MimeTypeVP8}, "video", "pion")
	assert.NoError(t, err)

	rtpSender, err := sender.AddTrack(track)
	assert.NoError(t, err)

	peerConnectionsConnected := untilConnectionState(PeerConnectionStateConnected, sender, receiver)

	assert.NoError(t, signalPair(sender, receiver))

	peerConnectionsConnected.Wait()

	assert.NoError(t, rtpSender.SetReadDeadline(time.Now().Add(1*time.Second)))
	_, _, err = rtpSender.ReadRTCP()
	assert.Error(t, err)

	assert.NoError(t, wan.Stop())
	closePairNow(t, sender, receiver)
}

func Test_RTPSender_ReplaceTrack_InvalidTrackKindChange(t *testing.T) {
	lim := test.TimeOut(time.Second * 10)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	sender, receiver, err := newPair()
	assert.NoError(t, err)

	trackA, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: MimeTypeVP8}, "video", "pion")
	assert.NoError(t, err)

	trackB, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: MimeTypeOpus}, "audio", "pion")
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

	assert.True(t, errors.Is(rtpSender.ReplaceTrack(trackB), ErrRTPSenderNewTrackHasIncorrectKind))

	closePairNow(t, sender, receiver)
}

func Test_RTPSender_ReplaceTrack_InvalidCodecChange(t *testing.T) {
	lim := test.TimeOut(time.Second * 10)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	sender, receiver, err := newPair()
	assert.NoError(t, err)

	trackA, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: MimeTypeVP8}, "video", "pion")
	assert.NoError(t, err)

	trackB, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: MimeTypeVP9}, "video", "pion")
	assert.NoError(t, err)

	rtpSender, err := sender.AddTrack(trackA)
	assert.NoError(t, err)

	err = rtpSender.rtpTransceiver.SetCodecPreferences([]RTPCodecParameters{{
		RTPCodecCapability: RTPCodecCapability{MimeType: MimeTypeVP8},
		PayloadType:        96,
	}})
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

	closePairNow(t, sender, receiver)
}

func Test_RTPSender_GetParameters_NilTrack(t *testing.T) {
	track, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: MimeTypeVP8}, "video", "pion")
	assert.NoError(t, err)

	peerConnection, err := NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	rtpSender, err := peerConnection.AddTrack(track)
	assert.NoError(t, err)

	assert.NoError(t, rtpSender.ReplaceTrack(nil))
	rtpSender.GetParameters()

	assert.NoError(t, peerConnection.Close())
}

func Test_RTPSender_Send(t *testing.T) {
	track, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: MimeTypeVP8}, "video", "pion")
	assert.NoError(t, err)

	peerConnection, err := NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	rtpSender, err := peerConnection.AddTrack(track)
	assert.NoError(t, err)

	parameter := rtpSender.GetParameters()
	err = rtpSender.Send(parameter)
	<-rtpSender.sendCalled
	assert.NoError(t, err)

	assert.NoError(t, peerConnection.Close())
}

func Test_RTPSender_Send_Called_Once(t *testing.T) {
	track, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: MimeTypeVP8}, "video", "pion")
	assert.NoError(t, err)

	peerConnection, err := NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	rtpSender, err := peerConnection.AddTrack(track)
	assert.NoError(t, err)

	parameter := rtpSender.GetParameters()
	err = rtpSender.Send(parameter)
	<-rtpSender.sendCalled
	assert.NoError(t, err)

	err = rtpSender.Send(parameter)
	assert.Equal(t, errRTPSenderSendAlreadyCalled, err)

	assert.NoError(t, peerConnection.Close())
}

func Test_RTPSender_Send_Track_Removed(t *testing.T) {
	track, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: MimeTypeVP8}, "video", "pion")
	assert.NoError(t, err)

	peerConnection, err := NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	rtpSender, err := peerConnection.AddTrack(track)
	assert.NoError(t, err)

	parameter := rtpSender.GetParameters()
	assert.NoError(t, peerConnection.RemoveTrack(rtpSender))
	assert.Equal(t, errRTPSenderTrackRemoved, rtpSender.Send(parameter))

	assert.NoError(t, peerConnection.Close())
}

func Test_RTPSender_Add_Encoding(t *testing.T) {
	track, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: MimeTypeVP8}, "video", "pion")
	assert.NoError(t, err)

	peerConnection, err := NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	rtpSender, err := peerConnection.AddTrack(track)
	assert.NoError(t, err)

	assert.Equal(t, errRTPSenderTrackNil, rtpSender.AddEncoding(nil))

	track1, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: MimeTypeVP8}, "video", "pion")
	assert.NoError(t, err)
	assert.Equal(t, errRTPSenderRidNil, rtpSender.AddEncoding(track1))

	track1, err = NewTrackLocalStaticSample(RTPCodecCapability{MimeType: MimeTypeVP8}, "video", "pion", WithRTPStreamID("h"))
	assert.NoError(t, err)
	assert.Equal(t, errRTPSenderNoBaseEncoding, rtpSender.AddEncoding(track1))

	track, err = NewTrackLocalStaticSample(RTPCodecCapability{MimeType: MimeTypeVP8}, "video", "pion", WithRTPStreamID("q"))
	assert.NoError(t, err)

	rtpSender, err = peerConnection.AddTrack(track)
	assert.NoError(t, err)

	track1, err = NewTrackLocalStaticSample(RTPCodecCapability{MimeType: MimeTypeVP8}, "video1", "pion", WithRTPStreamID("h"))
	assert.NoError(t, err)
	assert.Equal(t, errRTPSenderBaseEncodingMismatch, rtpSender.AddEncoding(track1))

	track1, err = NewTrackLocalStaticSample(RTPCodecCapability{MimeType: MimeTypeVP8}, "video", "pion1", WithRTPStreamID("h"))
	assert.NoError(t, err)
	assert.Equal(t, errRTPSenderBaseEncodingMismatch, rtpSender.AddEncoding(track1))

	track1, err = NewTrackLocalStaticSample(RTPCodecCapability{MimeType: MimeTypeOpus}, "video", "pion", WithRTPStreamID("h"))
	assert.NoError(t, err)
	assert.Equal(t, errRTPSenderBaseEncodingMismatch, rtpSender.AddEncoding(track1))

	track1, err = NewTrackLocalStaticSample(RTPCodecCapability{MimeType: MimeTypeVP8}, "video", "pion", WithRTPStreamID("q"))
	assert.NoError(t, err)
	assert.Equal(t, errRTPSenderRIDCollision, rtpSender.AddEncoding(track1))

	track1, err = NewTrackLocalStaticSample(RTPCodecCapability{MimeType: MimeTypeVP8}, "video", "pion", WithRTPStreamID("h"))
	assert.NoError(t, err)
	assert.NoError(t, rtpSender.AddEncoding(track1))

	err = rtpSender.Send(rtpSender.GetParameters())
	assert.NoError(t, err)

	track1, err = NewTrackLocalStaticSample(RTPCodecCapability{MimeType: MimeTypeVP8}, "video", "pion", WithRTPStreamID("f"))
	assert.NoError(t, err)
	assert.Equal(t, errRTPSenderSendAlreadyCalled, rtpSender.AddEncoding(track1))

	err = rtpSender.Stop()
	assert.NoError(t, err)

	assert.Equal(t, errRTPSenderStopped, rtpSender.AddEncoding(track1))

	assert.NoError(t, peerConnection.Close())
}
