// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js
// +build !js

package webrtc

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/pion/rtp"
	"github.com/pion/sdp/v3"
	"github.com/pion/transport/v3/test"
	"github.com/pion/webrtc/v4/pkg/media"
	"github.com/stretchr/testify/assert"
)

func TestSetRTPParameters(t *testing.T) {
	sender, receiver, wan := createVNetPair(t)

	outgoingTrack, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: MimeTypeVP8}, "video", "pion")
	assert.NoError(t, err)

	_, err = sender.AddTrack(outgoingTrack)
	assert.NoError(t, err)

	// Those parameters wouldn't make sense in a real application,
	// but for the sake of the test we just need different values.
	p := RTPParameters{
		Codecs: []RTPCodecParameters{
			{
				RTPCodecCapability: RTPCodecCapability{MimeTypeOpus, 48000, 2, "minptime=10;useinbandfec=1", []RTCPFeedback{{"nack", ""}}},
				PayloadType:        111,
			},
		},
		HeaderExtensions: []RTPHeaderExtensionParameter{
			{URI: sdp.SDESMidURI},
			{URI: sdp.SDESRTPStreamIDURI},
			{URI: sdesRepairRTPStreamIDURI},
		},
	}

	seenPacket, seenPacketCancel := context.WithCancel(context.Background())
	receiver.OnTrack(func(_ *TrackRemote, r *RTPReceiver) {
		r.SetRTPParameters(p)

		incomingTrackCodecs := r.Track().Codec()

		assert.EqualValues(t, p.HeaderExtensions, r.Track().params.HeaderExtensions)

		assert.EqualValues(t, p.Codecs[0].MimeType, incomingTrackCodecs.MimeType)
		assert.EqualValues(t, p.Codecs[0].ClockRate, incomingTrackCodecs.ClockRate)
		assert.EqualValues(t, p.Codecs[0].Channels, incomingTrackCodecs.Channels)
		assert.EqualValues(t, p.Codecs[0].SDPFmtpLine, incomingTrackCodecs.SDPFmtpLine)
		assert.EqualValues(t, p.Codecs[0].RTCPFeedback, incomingTrackCodecs.RTCPFeedback)
		assert.EqualValues(t, p.Codecs[0].PayloadType, incomingTrackCodecs.PayloadType)

		seenPacketCancel()
	})

	peerConnectionsConnected := untilConnectionState(PeerConnectionStateConnected, sender, receiver)

	assert.NoError(t, signalPair(sender, receiver))

	peerConnectionsConnected.Wait()
	assert.NoError(t, outgoingTrack.WriteSample(media.Sample{Data: []byte{0xAA}, Duration: time.Second}))

	<-seenPacket.Done()
	assert.NoError(t, wan.Stop())
	closePairNow(t, sender, receiver)
}

// Assert the behavior of reading a RTX with a distinct SSRC
// All the attributes should be populated and the packet unpacked
func Test_RTX_Read(t *testing.T) {
	defer test.TimeOut(time.Second * 30).Stop()

	pcOffer, pcAnswer, err := newPair()
	assert.NoError(t, err)

	track, err := NewTrackLocalStaticRTP(RTPCodecCapability{MimeType: MimeTypeVP8}, "track-id", "stream-id")
	assert.NoError(t, err)

	rtpSender, err := pcOffer.AddTrack(track)
	assert.NoError(t, err)

	rtxSsrc := rtpSender.GetParameters().Encodings[0].RTX.SSRC
	ssrc := rtpSender.GetParameters().Encodings[0].SSRC

	rtxRead, rtxReadCancel := context.WithCancel(context.Background())
	pcAnswer.OnTrack(func(track *TrackRemote, _ *RTPReceiver) {
		for {
			pkt, attributes, readRTPErr := track.ReadRTP()
			if errors.Is(readRTPErr, io.EOF) {
				return
			} else if pkt.PayloadType == 0 {
				continue
			}

			assert.NoError(t, readRTPErr)
			assert.NotNil(t, pkt)
			assert.Equal(t, pkt.SSRC, uint32(ssrc))
			assert.Equal(t, pkt.PayloadType, uint8(96))
			assert.Equal(t, pkt.Payload, []byte{0xB, 0xA, 0xD})

			rtxPayloadType := attributes.Get(AttributeRtxPayloadType)
			rtxSequenceNumber := attributes.Get(AttributeRtxSequenceNumber)
			rtxSSRC := attributes.Get(AttributeRtxSsrc)
			if rtxPayloadType != nil && rtxSequenceNumber != nil && rtxSSRC != nil {
				assert.Equal(t, rtxPayloadType, uint8(97))
				assert.Equal(t, rtxSSRC, uint32(rtxSsrc))
				assert.Equal(t, rtxSequenceNumber, pkt.SequenceNumber+500)

				rtxReadCancel()
			}
		}
	})

	assert.NoError(t, signalPair(pcOffer, pcAnswer))

	func() {
		for i := uint16(0); ; i++ {
			pkt := rtp.Packet{
				Header: rtp.Header{
					Version:        2,
					SSRC:           uint32(ssrc),
					PayloadType:    96,
					SequenceNumber: i,
				},
				Payload: []byte{0xB, 0xA, 0xD},
			}

			select {
			case <-time.After(20 * time.Millisecond):
				// Send the original packet
				err = track.WriteRTP(&pkt)
				assert.NoError(t, err)

				rtxPayload := []byte{0x0, 0x0, 0xB, 0xA, 0xD}
				binary.BigEndian.PutUint16(rtxPayload[0:2], pkt.Header.SequenceNumber)

				// Send the RTX
				_, err = track.bindings[0].writeStream.WriteRTP(&rtp.Header{
					Version:        2,
					SSRC:           uint32(rtxSsrc),
					PayloadType:    97,
					SequenceNumber: i + 500,
				}, rtxPayload)
				assert.NoError(t, err)
			case <-rtxRead.Done():
				return
			}
		}
	}()

	closePairNow(t, pcOffer, pcAnswer)
}
