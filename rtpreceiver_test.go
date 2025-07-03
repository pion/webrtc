// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js
// +build !js

package webrtc

import (
	"context"
	"testing"
	"time"

	"github.com/pion/interceptor"
	"github.com/pion/srtp/v3"
	"github.com/pion/transport/v3/test"
	"github.com/pion/webrtc/v4/pkg/media"
	"github.com/stretchr/testify/assert"
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

func Test_trackStreams_configureStreams(t *testing.T) {
	t.Run("with RTX", func(t *testing.T) {
		trackStreams := &trackStreams{}
		decodingParams := RTPDecodingParameters{
			RTPCodingParameters: RTPCodingParameters{
				RID:         "",
				SSRC:        0x89b82af4,
				PayloadType: 0x0,
				RTX:         RTPRtxParameters{SSRC: 0x35c9eefc},
				FEC:         RTPFecParameters{SSRC: 0x0},
			},
		}
		codec := RTPCodecCapability{
			MimeType:    "video/AV1",
			ClockRate:   0x15f90,
			Channels:    0x0,
			SDPFmtpLine: "level-idx=5;profile=0;tier=0",
			RTCPFeedback: []RTCPFeedback{
				{Type: "nack", Parameter: ""},
				{Type: "nack", Parameter: "pli"},
				{Type: "transport-cc", Parameter: ""},
			},
		}
		globalParams := RTPParameters{
			HeaderExtensions: []RTPHeaderExtensionParameter{
				{
					URI: "http://www.ietf.org/id/draft-holmer-rmcat-transport-wide-cc-extensions-01",
					ID:  4,
				},
			},
			Codecs: []RTPCodecParameters{
				{
					RTPCodecCapability: RTPCodecCapability{
						MimeType:    "video/AV1",
						ClockRate:   0x15f90,
						Channels:    0x0,
						SDPFmtpLine: "level-idx=5;profile=0;tier=0",
						RTCPFeedback: []RTCPFeedback{
							{Type: "nack", Parameter: ""},
							{Type: "nack", Parameter: "pli"},
							{Type: "transport-cc", Parameter: ""},
						},
					},
					PayloadType: 0x2d,
					statsID:     "",
				},
				{
					RTPCodecCapability: RTPCodecCapability{
						MimeType:     "video/rtx",
						ClockRate:    0x15f90,
						Channels:     0x0,
						SDPFmtpLine:  "apt=45",
						RTCPFeedback: nil,
					},
					PayloadType: 0x2e,
					statsID:     "",
				},
			},
		}
		expectedMediaStreamInfo := &interceptor.StreamInfo{
			ID:                                "",
			Attributes:                        interceptor.Attributes{},
			SSRC:                              0x89b82af4,
			SSRCRetransmission:                0x35c9eefc,
			SSRCForwardErrorCorrection:        0x0,
			PayloadType:                       0x2d,
			PayloadTypeRetransmission:         0x2e,
			PayloadTypeForwardErrorCorrection: 0x0,
			RTPHeaderExtensions: []interceptor.RTPHeaderExtension{
				{URI: "http://www.ietf.org/id/draft-holmer-rmcat-transport-wide-cc-extensions-01", ID: 4},
			},
			MimeType:    "video/AV1",
			ClockRate:   0x15f90,
			Channels:    0x0,
			SDPFmtpLine: "level-idx=5;profile=0;tier=0",
			RTCPFeedback: []interceptor.RTCPFeedback{
				{Type: "nack", Parameter: ""},
				{Type: "nack", Parameter: "pli"},
				{Type: "transport-cc", Parameter: ""},
			},
		}
		expectedRTXStreamInfo := &interceptor.StreamInfo{
			ID:                                "",
			Attributes:                        interceptor.Attributes{},
			SSRC:                              0x35c9eefc,
			SSRCRetransmission:                0x0,
			SSRCForwardErrorCorrection:        0x0,
			PayloadType:                       0x2e,
			PayloadTypeRetransmission:         0x0,
			PayloadTypeForwardErrorCorrection: 0x0,
			RTPHeaderExtensions: []interceptor.RTPHeaderExtension{
				{URI: "http://www.ietf.org/id/draft-holmer-rmcat-transport-wide-cc-extensions-01", ID: 4},
			},
			MimeType:    "video/AV1",
			ClockRate:   0x15f90,
			Channels:    0x0,
			SDPFmtpLine: "level-idx=5;profile=0;tier=0",
			RTCPFeedback: []interceptor.RTCPFeedback{
				{Type: "nack", Parameter: ""},
				{Type: "nack", Parameter: "pli"},
				{Type: "transport-cc", Parameter: ""},
			},
		}
		callCount := 0
		haveRTX, err := trackStreams.configureStreams(
			decodingParams,
			codec,
			globalParams,
			// Mock DTLSTransport.streamsForSSRC here, DTLSTransport.streamsForSSRC calls interceptor.BindRemoteStream,
			// so the interceptor.StreamInfo here is exactly the StreamInfo that will be passed into interceptors.
			func(s SSRC, si interceptor.StreamInfo) (
				*srtp.ReadStreamSRTP,
				interceptor.RTPReader,
				*srtp.ReadStreamSRTCP,
				interceptor.RTCPReader,
				error,
			) {
				switch callCount {
				case 0:
					assert.Equal(t, s, SSRC(0x89b82af4))
					assert.Equal(t, si, *expectedMediaStreamInfo)
				case 1:
					assert.Equal(t, s, SSRC(0x35c9eefc))
					assert.Equal(t, si, *expectedRTXStreamInfo)
				default:
					assert.Fail(t, "streamsForSSRC called more than twice when only video track and rtx track existed")
				}
				callCount++

				return nil, nil, nil, nil, nil
			},
		)
		assert.NoError(t, err)
		assert.True(t, haveRTX)
		assert.Equal(t, trackStreams.mediaStream.streamInfo, expectedMediaStreamInfo)
		assert.Equal(t, trackStreams.rtxStream.streamInfo, expectedRTXStreamInfo)
	})

	t.Run("no RTX", func(t *testing.T) {
		trackStreams := &trackStreams{}
		decodingParams := RTPDecodingParameters{
			RTPCodingParameters: RTPCodingParameters{
				RID:         "",
				SSRC:        0x89b82af4,
				PayloadType: 0x0,
				RTX:         RTPRtxParameters{SSRC: 0x0},
				FEC:         RTPFecParameters{SSRC: 0x0},
			},
		}
		codec := RTPCodecCapability{
			MimeType:    "video/AV1",
			ClockRate:   0x15f90,
			Channels:    0x0,
			SDPFmtpLine: "level-idx=5;profile=0;tier=0",
			RTCPFeedback: []RTCPFeedback{
				{Type: "transport-cc", Parameter: ""},
			},
		}
		globalParams := RTPParameters{
			HeaderExtensions: []RTPHeaderExtensionParameter{
				{
					URI: "http://www.ietf.org/id/draft-holmer-rmcat-transport-wide-cc-extensions-01",
					ID:  4,
				},
			},
			Codecs: []RTPCodecParameters{
				{
					RTPCodecCapability: RTPCodecCapability{
						MimeType:    "video/AV1",
						ClockRate:   0x15f90,
						Channels:    0x0,
						SDPFmtpLine: "level-idx=5;profile=0;tier=0",
						RTCPFeedback: []RTCPFeedback{
							{Type: "transport-cc", Parameter: ""},
						},
					},
					PayloadType: 0x2d,
					statsID:     "",
				},
			},
		}
		expectedMediaStreamInfo := &interceptor.StreamInfo{
			ID:                                "",
			Attributes:                        interceptor.Attributes{},
			SSRC:                              0x89b82af4,
			SSRCRetransmission:                0x0,
			SSRCForwardErrorCorrection:        0x0,
			PayloadType:                       0x2d,
			PayloadTypeRetransmission:         0x0,
			PayloadTypeForwardErrorCorrection: 0x0,
			RTPHeaderExtensions: []interceptor.RTPHeaderExtension{
				{URI: "http://www.ietf.org/id/draft-holmer-rmcat-transport-wide-cc-extensions-01", ID: 4},
			},
			MimeType:    "video/AV1",
			ClockRate:   0x15f90,
			Channels:    0x0,
			SDPFmtpLine: "level-idx=5;profile=0;tier=0",
			RTCPFeedback: []interceptor.RTCPFeedback{
				{Type: "transport-cc", Parameter: ""},
			},
		}
		var expectedRTXStreamInfo *interceptor.StreamInfo
		callCount := 0
		haveRTX, err := trackStreams.configureStreams(
			decodingParams,
			codec,
			globalParams,
			// Mock DTLSTransport.streamsForSSRC here, DTLSTransport.streamsForSSRC calls interceptor.BindRemoteStream,
			// so the interceptor.StreamInfo here is exactly the StreamInfo that will be passed into interceptors.
			func(s SSRC, si interceptor.StreamInfo) (
				*srtp.ReadStreamSRTP,
				interceptor.RTPReader,
				*srtp.ReadStreamSRTCP,
				interceptor.RTCPReader,
				error,
			) {
				switch callCount {
				case 0:
					assert.Equal(t, s, SSRC(0x89b82af4))
					assert.Equal(t, si, *expectedMediaStreamInfo)
				default:
					assert.Fail(t, "streamsForSSRC called more than once when only video track existed")
				}
				callCount++

				return nil, nil, nil, nil, nil
			},
		)
		assert.NoError(t, err)
		assert.False(t, haveRTX)
		assert.Equal(t, trackStreams.mediaStream.streamInfo, expectedMediaStreamInfo)
		assert.Equal(t, trackStreams.rtxStream.streamInfo, expectedRTXStreamInfo)
	})
}
