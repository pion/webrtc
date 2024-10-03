// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js
// +build !js

package webrtc

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/pion/ice/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errReceiveOfferTimeout = fmt.Errorf("timed out waiting to receive offer")

func TestStatsTimestampTime(t *testing.T) {
	for _, test := range []struct {
		Timestamp StatsTimestamp
		WantTime  time.Time
	}{
		{
			Timestamp: 0,
			WantTime:  time.Unix(0, 0),
		},
		{
			Timestamp: 1,
			WantTime:  time.Unix(0, 1e6),
		},
		{
			Timestamp: 0.001,
			WantTime:  time.Unix(0, 1e3),
		},
	} {
		if got, want := test.Timestamp.Time(), test.WantTime.UTC(); got != want {
			t.Fatalf("StatsTimestamp(%v).Time() = %v, want %v", test.Timestamp, got, want)
		}
	}
}

type statSample struct {
	name  string
	stats Stats
	json  string
}

func getStatsSamples() []statSample {
	codecStats := CodecStats{
		Timestamp:      1688978831527.718,
		Type:           StatsTypeCodec,
		ID:             "COT01_111_minptime=10;useinbandfec=1",
		PayloadType:    111,
		CodecType:      CodecTypeEncode,
		TransportID:    "T01",
		MimeType:       "audio/opus",
		ClockRate:      48000,
		Channels:       2,
		SDPFmtpLine:    "minptime=10;useinbandfec=1",
		Implementation: "libvpx",
	}
	codecStatsJSON := `
{
	"timestamp": 1688978831527.718,
	"type": "codec",
	"id": "COT01_111_minptime=10;useinbandfec=1",
	"payloadType": 111,
	"codecType": "encode",
	"transportId": "T01",
	"mimeType": "audio/opus",
	"clockRate": 48000,
	"channels": 2,
	"sdpFmtpLine": "minptime=10;useinbandfec=1",
	"implementation": "libvpx"
}
`
	inboundRTPStreamStats := InboundRTPStreamStats{
		Mid:                            "1",
		Timestamp:                      1688978831527.718,
		ID:                             "IT01A2184088143",
		Type:                           StatsTypeInboundRTP,
		SSRC:                           2184088143,
		Kind:                           "audio",
		TransportID:                    "T01",
		CodecID:                        "CIT01_111_minptime=10;useinbandfec=1",
		FIRCount:                       1,
		PLICount:                       2,
		TotalProcessingDelay:           23,
		NACKCount:                      3,
		JitterBufferDelay:              24,
		JitterBufferTargetDelay:        25,
		JitterBufferEmittedCount:       26,
		JitterBufferMinimumDelay:       27,
		TotalSamplesReceived:           28,
		ConcealedSamples:               29,
		SilentConcealedSamples:         30,
		ConcealmentEvents:              31,
		InsertedSamplesForDeceleration: 32,
		RemovedSamplesForAcceleration:  33,
		AudioLevel:                     34,
		TotalAudioEnergy:               35,
		TotalSamplesDuration:           36,
		SLICount:                       4,
		QPSum:                          5,
		TotalDecodeTime:                37,
		TotalInterFrameDelay:           38,
		TotalSquaredInterFrameDelay:    39,
		PacketsReceived:                6,
		PacketsLost:                    7,
		Jitter:                         8,
		PacketsDiscarded:               9,
		PacketsRepaired:                10,
		BurstPacketsLost:               11,
		BurstPacketsDiscarded:          12,
		BurstLossCount:                 13,
		BurstDiscardCount:              14,
		BurstLossRate:                  15,
		BurstDiscardRate:               16,
		GapLossRate:                    17,
		GapDiscardRate:                 18,
		TrackID:                        "d57dbc4b-484b-4b40-9088-d3150e3a2010",
		ReceiverID:                     "R01",
		RemoteID:                       "ROA2184088143",
		FramesDecoded:                  17,
		KeyFramesDecoded:               40,
		FramesRendered:                 41,
		FramesDropped:                  42,
		FrameWidth:                     43,
		FrameHeight:                    44,
		LastPacketReceivedTimestamp:    1689668364374.181,
		HeaderBytesReceived:            45,
		AverageRTCPInterval:            18,
		FECPacketsReceived:             19,
		FECPacketsDiscarded:            46,
		BytesReceived:                  20,
		FramesReceived:                 47,
		PacketsFailedDecryption:        21,
		PacketsDuplicated:              22,
		PerDSCPPacketsReceived: map[string]uint32{
			"123": 23,
		},
	}
	inboundRTPStreamStatsJSON := `
{
  "mid": "1",
  "timestamp": 1688978831527.718,
  "id": "IT01A2184088143",
  "type": "inbound-rtp",
  "ssrc": 2184088143,
  "kind": "audio",
  "transportId": "T01",
  "codecId": "CIT01_111_minptime=10;useinbandfec=1",
  "firCount": 1,
  "pliCount": 2,
  "totalProcessingDelay": 23,
  "nackCount": 3,
  "jitterBufferDelay": 24,
  "jitterBufferTargetDelay": 25,
  "jitterBufferEmittedCount": 26,
  "jitterBufferMinimumDelay": 27,
  "totalSamplesReceived": 28,
  "concealedSamples": 29,
  "silentConcealedSamples": 30,
  "concealmentEvents": 31,
  "insertedSamplesForDeceleration": 32,
  "removedSamplesForAcceleration": 33,
  "audioLevel": 34,
  "totalAudioEnergy": 35,
  "totalSamplesDuration": 36,
  "sliCount": 4,
  "qpSum": 5,
  "totalDecodeTime": 37,
  "totalInterFrameDelay": 38,
  "totalSquaredInterFrameDelay": 39,
  "packetsReceived": 6,
  "packetsLost": 7,
  "jitter": 8,
  "packetsDiscarded": 9,
  "packetsRepaired": 10,
  "burstPacketsLost": 11,
  "burstPacketsDiscarded": 12,
  "burstLossCount": 13,
  "burstDiscardCount": 14,
  "burstLossRate": 15,
  "burstDiscardRate": 16,
  "gapLossRate": 17,
  "gapDiscardRate": 18,
  "trackId": "d57dbc4b-484b-4b40-9088-d3150e3a2010",
  "receiverId": "R01",
  "remoteId": "ROA2184088143",
  "framesDecoded": 17,
  "keyFramesDecoded": 40,
  "framesRendered": 41,
  "framesDropped": 42,
  "frameWidth": 43,
  "frameHeight": 44,
  "lastPacketReceivedTimestamp": 1689668364374.181,
  "headerBytesReceived": 45,
  "averageRtcpInterval": 18,
  "fecPacketsReceived": 19,
  "fecPacketsDiscarded": 46,
  "bytesReceived": 20,
  "framesReceived": 47,
  "packetsFailedDecryption": 21,
  "packetsDuplicated": 22,
  "perDscpPacketsReceived": {
    "123": 23
  }
}
`
	outboundRTPStreamStats := OutboundRTPStreamStats{
		Mid:                      "1",
		Rid:                      "hi",
		MediaSourceID:            "SA5",
		Timestamp:                1688978831527.718,
		Type:                     StatsTypeOutboundRTP,
		ID:                       "OT01A2184088143",
		SSRC:                     2184088143,
		Kind:                     "audio",
		TransportID:              "T01",
		CodecID:                  "COT01_111_minptime=10;useinbandfec=1",
		HeaderBytesSent:          24,
		RetransmittedPacketsSent: 25,
		RetransmittedBytesSent:   26,
		FIRCount:                 1,
		PLICount:                 2,
		NACKCount:                3,
		SLICount:                 4,
		QPSum:                    5,
		PacketsSent:              6,
		PacketsDiscardedOnSend:   7,
		FECPacketsSent:           8,
		BytesSent:                9,
		BytesDiscardedOnSend:     10,
		TrackID:                  "d57dbc4b-484b-4b40-9088-d3150e3a2010",
		SenderID:                 "S01",
		RemoteID:                 "ROA2184088143",
		LastPacketSentTimestamp:  11,
		TargetBitrate:            12,
		TotalEncodedBytesTarget:  27,
		FrameWidth:               28,
		FrameHeight:              29,
		FramesPerSecond:          30,
		FramesSent:               31,
		HugeFramesSent:           32,
		FramesEncoded:            13,
		KeyFramesEncoded:         33,
		TotalEncodeTime:          14,
		TotalPacketSendDelay:     34,
		AverageRTCPInterval:      15,
		QualityLimitationReason:  "cpu",
		QualityLimitationDurations: map[string]float64{
			"none":      16,
			"cpu":       17,
			"bandwidth": 18,
			"other":     19,
		},
		QualityLimitationResolutionChanges: 35,
		PerDSCPPacketsSent: map[string]uint32{
			"123": 23,
		},
		Active: true,
	}
	outboundRTPStreamStatsJSON := `
{
  "mid": "1",
  "rid": "hi",
  "mediaSourceId": "SA5",
  "timestamp": 1688978831527.718,
  "type": "outbound-rtp",
  "id": "OT01A2184088143",
  "ssrc": 2184088143,
  "kind": "audio",
  "transportId": "T01",
  "codecId": "COT01_111_minptime=10;useinbandfec=1",
  "headerBytesSent": 24,
  "retransmittedPacketsSent": 25,
  "retransmittedBytesSent": 26,
  "firCount": 1,
  "pliCount": 2,
  "nackCount": 3,
  "sliCount": 4,
  "qpSum": 5,
  "packetsSent": 6,
  "packetsDiscardedOnSend": 7,
  "fecPacketsSent": 8,
  "bytesSent": 9,
  "bytesDiscardedOnSend": 10,
  "trackId": "d57dbc4b-484b-4b40-9088-d3150e3a2010",
  "senderId": "S01",
  "remoteId": "ROA2184088143",
  "lastPacketSentTimestamp": 11,
  "targetBitrate": 12,
  "totalEncodedBytesTarget": 27,
  "frameWidth": 28,
  "frameHeight": 29,
  "framesPerSecond": 30,
  "framesSent": 31,
  "hugeFramesSent": 32,
  "framesEncoded": 13,
  "keyFramesEncoded": 33,
  "totalEncodeTime": 14,
  "totalPacketSendDelay": 34,
  "averageRtcpInterval": 15,
  "qualityLimitationReason": "cpu",
  "qualityLimitationDurations": {
    "none": 16,
    "cpu": 17,
    "bandwidth": 18,
    "other": 19
  },
  "qualityLimitationResolutionChanges": 35,
  "perDscpPacketsSent": {
    "123": 23
  },
  "active": true
}
`
	remoteInboundRTPStreamStats := RemoteInboundRTPStreamStats{
		Timestamp:                 1688978831527.718,
		Type:                      StatsTypeRemoteInboundRTP,
		ID:                        "RIA2184088143",
		SSRC:                      2184088143,
		Kind:                      "audio",
		TransportID:               "T01",
		CodecID:                   "COT01_111_minptime=10;useinbandfec=1",
		FIRCount:                  1,
		PLICount:                  2,
		NACKCount:                 3,
		SLICount:                  4,
		QPSum:                     5,
		PacketsReceived:           6,
		PacketsLost:               7,
		Jitter:                    8,
		PacketsDiscarded:          9,
		PacketsRepaired:           10,
		BurstPacketsLost:          11,
		BurstPacketsDiscarded:     12,
		BurstLossCount:            13,
		BurstDiscardCount:         14,
		BurstLossRate:             15,
		BurstDiscardRate:          16,
		GapLossRate:               17,
		GapDiscardRate:            18,
		LocalID:                   "RIA2184088143",
		RoundTripTime:             19,
		TotalRoundTripTime:        21,
		FractionLost:              20,
		RoundTripTimeMeasurements: 22,
	}
	remoteInboundRTPStreamStatsJSON := `
{
  "timestamp": 1688978831527.718,
  "type": "remote-inbound-rtp",
  "id": "RIA2184088143",
  "ssrc": 2184088143,
  "kind": "audio",
  "transportId": "T01",
  "codecId": "COT01_111_minptime=10;useinbandfec=1",
  "firCount": 1,
  "pliCount": 2,
  "nackCount": 3,
  "sliCount": 4,
  "qpSum": 5,
  "packetsReceived": 6,
  "packetsLost": 7,
  "jitter": 8,
  "packetsDiscarded": 9,
  "packetsRepaired": 10,
  "burstPacketsLost": 11,
  "burstPacketsDiscarded": 12,
  "burstLossCount": 13,
  "burstDiscardCount": 14,
  "burstLossRate": 15,
  "burstDiscardRate": 16,
  "gapLossRate": 17,
  "gapDiscardRate": 18,
  "localId": "RIA2184088143",
  "roundTripTime": 19,
  "totalRoundTripTime": 21,
  "fractionLost": 20,
  "roundTripTimeMeasurements": 22
}
`
	remoteOutboundRTPStreamStats := RemoteOutboundRTPStreamStats{
		Timestamp:                 1688978831527.718,
		Type:                      StatsTypeRemoteOutboundRTP,
		ID:                        "ROA2184088143",
		SSRC:                      2184088143,
		Kind:                      "audio",
		TransportID:               "T01",
		CodecID:                   "CIT01_111_minptime=10;useinbandfec=1",
		FIRCount:                  1,
		PLICount:                  2,
		NACKCount:                 3,
		SLICount:                  4,
		QPSum:                     5,
		PacketsSent:               1259,
		PacketsDiscardedOnSend:    6,
		FECPacketsSent:            7,
		BytesSent:                 92654,
		BytesDiscardedOnSend:      8,
		LocalID:                   "IT01A2184088143",
		RemoteTimestamp:           1689668361298,
		ReportsSent:               9,
		RoundTripTime:             10,
		TotalRoundTripTime:        11,
		RoundTripTimeMeasurements: 12,
	}
	remoteOutboundRTPStreamStatsJSON := `
{
  "timestamp": 1688978831527.718,
  "type": "remote-outbound-rtp",
  "id": "ROA2184088143",
  "ssrc": 2184088143,
  "kind": "audio",
  "transportId": "T01",
  "codecId": "CIT01_111_minptime=10;useinbandfec=1",
  "firCount": 1,
  "pliCount": 2,
  "nackCount": 3,
  "sliCount": 4,
  "qpSum": 5,
  "packetsSent": 1259,
  "packetsDiscardedOnSend": 6,
  "fecPacketsSent": 7,
  "bytesSent": 92654,
  "bytesDiscardedOnSend": 8,
  "localId": "IT01A2184088143",
  "remoteTimestamp": 1689668361298,
  "reportsSent": 9,
  "roundTripTime": 10,
  "totalRoundTripTime": 11,
  "roundTripTimeMeasurements": 12
}
`
	csrcStats := RTPContributingSourceStats{
		Timestamp:            1688978831527.718,
		Type:                 StatsTypeCSRC,
		ID:                   "ROA2184088143",
		ContributorSSRC:      2184088143,
		InboundRTPStreamID:   "IT01A2184088143",
		PacketsContributedTo: 5,
		AudioLevel:           0.3,
	}
	csrcStatsJSON := `
{
  "timestamp": 1688978831527.718,
  "type": "csrc",
  "id": "ROA2184088143",
  "contributorSsrc": 2184088143,
  "inboundRtpStreamId": "IT01A2184088143",
  "packetsContributedTo": 5,
  "audioLevel": 0.3
}
`
	audioSourceStats := AudioSourceStats{
		Timestamp:                 1689668364374.479,
		Type:                      StatsTypeMediaSource,
		ID:                        "SA5",
		TrackIdentifier:           "d57dbc4b-484b-4b40-9088-d3150e3a2010",
		Kind:                      "audio",
		AudioLevel:                0.0030518509475997192,
		TotalAudioEnergy:          0.0024927631236904358,
		TotalSamplesDuration:      28.360000000001634,
		EchoReturnLoss:            -30,
		EchoReturnLossEnhancement: 0.17551203072071075,
		DroppedSamplesDuration:    0.1,
		DroppedSamplesEvents:      2,
		TotalCaptureDelay:         0.3,
		TotalSamplesCaptured:      4,
	}
	audioSourceStatsJSON := `
{
  "timestamp": 1689668364374.479,
  "type": "media-source",
  "id": "SA5",
  "trackIdentifier": "d57dbc4b-484b-4b40-9088-d3150e3a2010",
  "kind": "audio",
  "audioLevel": 0.0030518509475997192,
  "totalAudioEnergy": 0.0024927631236904358,
  "totalSamplesDuration": 28.360000000001634,
  "echoReturnLoss": -30,
  "echoReturnLossEnhancement": 0.17551203072071075,
  "droppedSamplesDuration": 0.1,
  "droppedSamplesEvents": 2,
  "totalCaptureDelay": 0.3,
  "totalSamplesCaptured": 4
}
`
	videoSourceStats := VideoSourceStats{
		Timestamp:       1689668364374.479,
		Type:            StatsTypeMediaSource,
		ID:              "SV6",
		TrackIdentifier: "d7f11739-d395-42e9-af87-5dfa1cc10ee0",
		Kind:            "video",
		Width:           640,
		Height:          480,
		Frames:          850,
		FramesPerSecond: 30,
	}
	videoSourceStatsJSON := `
{
  "timestamp": 1689668364374.479,
  "type": "media-source",
  "id": "SV6",
  "trackIdentifier": "d7f11739-d395-42e9-af87-5dfa1cc10ee0",
  "kind": "video",
  "width": 640,
  "height": 480,
  "frames": 850,
  "framesPerSecond": 30
}
`
	audioPlayoutStats := AudioPlayoutStats{
		Timestamp:                  1689668364374.181,
		Type:                       StatsTypeMediaPlayout,
		ID:                         "AP",
		Kind:                       "audio",
		SynthesizedSamplesDuration: 1,
		SynthesizedSamplesEvents:   2,
		TotalSamplesDuration:       593.5,
		TotalPlayoutDelay:          1062194.11536,
		TotalSamplesCount:          28488000,
	}
	audioPlayoutStatsJSON := `
{
  "timestamp": 1689668364374.181,
  "type": "media-playout",
  "id": "AP",
  "kind": "audio",
  "synthesizedSamplesDuration": 1,
  "synthesizedSamplesEvents": 2,
  "totalSamplesDuration": 593.5,
  "totalPlayoutDelay": 1062194.11536,
  "totalSamplesCount": 28488000
}
`
	peerConnectionStats := PeerConnectionStats{
		Timestamp:             1688978831527.718,
		Type:                  StatsTypePeerConnection,
		ID:                    "P",
		DataChannelsOpened:    1,
		DataChannelsClosed:    2,
		DataChannelsRequested: 3,
		DataChannelsAccepted:  4,
	}
	peerConnectionStatsJSON := `
{
  "timestamp": 1688978831527.718,
  "type": "peer-connection",
  "id": "P",
  "dataChannelsOpened": 1,
  "dataChannelsClosed": 2,
  "dataChannelsRequested": 3,
  "dataChannelsAccepted": 4
}
`
	dataChannelStats := DataChannelStats{
		Timestamp:             1688978831527.718,
		Type:                  StatsTypeDataChannel,
		ID:                    "D1",
		Label:                 "display",
		Protocol:              "protocol",
		DataChannelIdentifier: 1,
		TransportID:           "T1",
		State:                 DataChannelStateOpen,
		MessagesSent:          1,
		BytesSent:             16,
		MessagesReceived:      2,
		BytesReceived:         20,
	}
	dataChannelStatsJSON := `
{
  "timestamp": 1688978831527.718,
  "type": "data-channel",
  "id": "D1",
  "label": "display",
  "protocol": "protocol",
  "dataChannelIdentifier": 1,
  "transportId": "T1",
  "state": "open",
  "messagesSent": 1,
  "bytesSent": 16,
  "messagesReceived": 2,
  "bytesReceived": 20
}
`
	streamStats := MediaStreamStats{
		Timestamp:        1688978831527.718,
		Type:             StatsTypeStream,
		ID:               "ROA2184088143",
		StreamIdentifier: "S1",
		TrackIDs:         []string{"d57dbc4b-484b-4b40-9088-d3150e3a2010"},
	}
	streamStatsJSON := `
{
  "timestamp": 1688978831527.718,
  "type": "stream",
  "id": "ROA2184088143",
  "streamIdentifier": "S1",
  "trackIds": [
    "d57dbc4b-484b-4b40-9088-d3150e3a2010"
  ]
}
`
	senderVideoTrackAttachmentStats := SenderVideoTrackAttachmentStats{
		Timestamp:      1688978831527.718,
		Type:           StatsTypeTrack,
		ID:             "S2",
		Kind:           "video",
		FramesCaptured: 1,
		FramesSent:     2,
		HugeFramesSent: 3,
		KeyFramesSent:  4,
	}
	senderVideoTrackAttachmentStatsJSON := `
{
  "timestamp": 1688978831527.718,
  "type": "track",
  "id": "S2",
  "kind": "video",
  "framesCaptured": 1,
  "framesSent": 2,
  "hugeFramesSent": 3,
  "keyFramesSent": 4
}
`
	senderAudioTrackAttachmentStats := SenderAudioTrackAttachmentStats{
		Timestamp:                 1688978831527.718,
		Type:                      StatsTypeTrack,
		ID:                        "S1",
		TrackIdentifier:           "audio",
		RemoteSource:              true,
		Ended:                     true,
		Kind:                      "audio",
		AudioLevel:                0.1,
		TotalAudioEnergy:          0.2,
		VoiceActivityFlag:         true,
		TotalSamplesDuration:      0.3,
		EchoReturnLoss:            0.4,
		EchoReturnLossEnhancement: 0.5,
		TotalSamplesSent:          200,
	}
	senderAudioTrackAttachmentStatsJSON := `
{
  "timestamp": 1688978831527.718,
  "type": "track",
  "id": "S1",
  "trackIdentifier": "audio",
  "remoteSource": true,
  "ended": true,
  "kind": "audio",
  "audioLevel": 0.1,
  "totalAudioEnergy": 0.2,
  "voiceActivityFlag": true,
  "totalSamplesDuration": 0.3,
  "echoReturnLoss": 0.4,
  "echoReturnLossEnhancement": 0.5,
  "totalSamplesSent": 200
}
`
	videoSenderStats := VideoSenderStats{
		Timestamp:      1688978831527.718,
		Type:           StatsTypeSender,
		ID:             "S2",
		Kind:           "video",
		FramesCaptured: 1,
		FramesSent:     2,
		HugeFramesSent: 3,
		KeyFramesSent:  4,
	}
	videoSenderStatsJSON := `
{
  "timestamp": 1688978831527.718,
  "type": "sender",
  "id": "S2",
  "kind": "video",
  "framesCaptured": 1,
  "framesSent": 2,
  "hugeFramesSent": 3,
  "keyFramesSent": 4
}
`
	audioSenderStats := AudioSenderStats{
		Timestamp:                 1688978831527.718,
		Type:                      StatsTypeSender,
		ID:                        "S1",
		TrackIdentifier:           "audio",
		RemoteSource:              true,
		Ended:                     true,
		Kind:                      "audio",
		AudioLevel:                0.1,
		TotalAudioEnergy:          0.2,
		VoiceActivityFlag:         true,
		TotalSamplesDuration:      0.3,
		EchoReturnLoss:            0.4,
		EchoReturnLossEnhancement: 0.5,
		TotalSamplesSent:          200,
	}
	audioSenderStatsJSON := `
{
  "timestamp": 1688978831527.718,
  "type": "sender",
  "id": "S1",
  "trackIdentifier": "audio",
  "remoteSource": true,
  "ended": true,
  "kind": "audio",
  "audioLevel": 0.1,
  "totalAudioEnergy": 0.2,
  "voiceActivityFlag": true,
  "totalSamplesDuration": 0.3,
  "echoReturnLoss": 0.4,
  "echoReturnLossEnhancement": 0.5,
  "totalSamplesSent": 200
}
`
	videoReceiverStats := VideoReceiverStats{
		Timestamp:                 1688978831527.718,
		Type:                      StatsTypeReceiver,
		ID:                        "ROA2184088143",
		Kind:                      "video",
		FrameWidth:                720,
		FrameHeight:               480,
		FramesPerSecond:           30.0,
		EstimatedPlayoutTimestamp: 1688978831527.718,
		JitterBufferDelay:         0.1,
		JitterBufferEmittedCount:  1,
		FramesReceived:            79,
		KeyFramesReceived:         10,
		FramesDecoded:             10,
		FramesDropped:             10,
		PartialFramesLost:         5,
		FullFramesLost:            5,
	}
	videoReceiverStatsJSON := `
{
  "timestamp": 1688978831527.718,
  "type": "receiver",
  "id": "ROA2184088143",
  "kind": "video",
  "frameWidth": 720,
  "frameHeight": 480,
  "framesPerSecond": 30.0,
  "estimatedPlayoutTimestamp": 1688978831527.718,
  "jitterBufferDelay": 0.1,
  "jitterBufferEmittedCount": 1,
  "framesReceived": 79,
  "keyFramesReceived": 10,
  "framesDecoded": 10,
  "framesDropped": 10,
  "partialFramesLost": 5,
  "fullFramesLost": 5
}
`
	audioReceiverStats := AudioReceiverStats{
		Timestamp:                 1688978831527.718,
		Type:                      StatsTypeReceiver,
		ID:                        "R1",
		Kind:                      "audio",
		AudioLevel:                0.1,
		TotalAudioEnergy:          0.2,
		VoiceActivityFlag:         true,
		TotalSamplesDuration:      0.3,
		EstimatedPlayoutTimestamp: 1688978831527.718,
		JitterBufferDelay:         0.5,
		JitterBufferEmittedCount:  6,
		TotalSamplesReceived:      7,
		ConcealedSamples:          8,
		ConcealmentEvents:         9,
	}
	audioReceiverStatsJSON := `
{
  "timestamp": 1688978831527.718,
  "type": "receiver",
  "id": "R1",
  "kind": "audio",
  "audioLevel": 0.1,
  "totalAudioEnergy": 0.2,
  "voiceActivityFlag": true,
  "totalSamplesDuration": 0.3,
  "estimatedPlayoutTimestamp": 1688978831527.718,
  "jitterBufferDelay": 0.5,
  "jitterBufferEmittedCount": 6,
  "totalSamplesReceived": 7,
  "concealedSamples": 8,
  "concealmentEvents": 9
}
`
	transportStats := TransportStats{
		Timestamp:               1688978831527.718,
		Type:                    StatsTypeTransport,
		ID:                      "T01",
		PacketsSent:             60,
		PacketsReceived:         8,
		BytesSent:               6517,
		BytesReceived:           1159,
		RTCPTransportStatsID:    "T01",
		ICERole:                 ICERoleControlling,
		DTLSState:               DTLSTransportStateConnected,
		ICEState:                ICETransportStateConnected,
		SelectedCandidatePairID: "CPxIhBDNnT_sPDhy1TB",
		LocalCertificateID:      "CFF4:4F:C4:C7:F3:31:6C:B9:D5:AD:19:64:05:9F:2F:E9:00:70:56:1E:BA:92:29:3A:08:CE:1B:27:CF:2D:AB:24",
		RemoteCertificateID:     "CF62:AF:88:F7:F3:0F:D6:C4:93:91:1E:AD:52:F0:A4:12:04:F9:48:E7:06:16:BA:A3:86:26:8F:1E:38:1C:48:49",
		DTLSCipher:              "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256",
		SRTPCipher:              "AES_CM_128_HMAC_SHA1_80",
	}
	transportStatsJSON := `
{
  "timestamp": 1688978831527.718,
  "type": "transport",
  "id": "T01",
  "packetsSent": 60,
  "packetsReceived": 8,
  "bytesSent": 6517,
  "bytesReceived": 1159,
  "rtcpTransportStatsId": "T01",
  "iceRole": "controlling",
  "dtlsState": "connected",
  "iceState": "connected",
  "selectedCandidatePairId": "CPxIhBDNnT_sPDhy1TB",
  "localCertificateId": "CFF4:4F:C4:C7:F3:31:6C:B9:D5:AD:19:64:05:9F:2F:E9:00:70:56:1E:BA:92:29:3A:08:CE:1B:27:CF:2D:AB:24",
  "remoteCertificateId": "CF62:AF:88:F7:F3:0F:D6:C4:93:91:1E:AD:52:F0:A4:12:04:F9:48:E7:06:16:BA:A3:86:26:8F:1E:38:1C:48:49",
  "dtlsCipher": "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256",
  "srtpCipher": "AES_CM_128_HMAC_SHA1_80"
}
`
	iceCandidatePairStats := ICECandidatePairStats{
		Timestamp:                   1688978831527.718,
		Type:                        StatsTypeCandidatePair,
		ID:                          "CPxIhBDNnT_LlMJOnBv",
		TransportID:                 "T01",
		LocalCandidateID:            "IxIhBDNnT",
		RemoteCandidateID:           "ILlMJOnBv",
		State:                       "waiting",
		Nominated:                   true,
		PacketsSent:                 1,
		PacketsReceived:             2,
		BytesSent:                   3,
		BytesReceived:               4,
		LastPacketSentTimestamp:     5,
		LastPacketReceivedTimestamp: 6,
		FirstRequestTimestamp:       7,
		LastRequestTimestamp:        8,
		LastResponseTimestamp:       9,
		TotalRoundTripTime:          10,
		CurrentRoundTripTime:        11,
		AvailableOutgoingBitrate:    12,
		AvailableIncomingBitrate:    13,
		CircuitBreakerTriggerCount:  14,
		RequestsReceived:            15,
		RequestsSent:                16,
		ResponsesReceived:           17,
		ResponsesSent:               18,
		RetransmissionsReceived:     19,
		RetransmissionsSent:         20,
		ConsentRequestsSent:         21,
		ConsentExpiredTimestamp:     22,
		PacketsDiscardedOnSend:      23,
		BytesDiscardedOnSend:        24,
	}
	iceCandidatePairStatsJSON := `
{
  "timestamp": 1688978831527.718,
  "type": "candidate-pair",
  "id": "CPxIhBDNnT_LlMJOnBv",
  "transportId": "T01",
  "localCandidateId": "IxIhBDNnT",
  "remoteCandidateId": "ILlMJOnBv",
  "state": "waiting",
  "nominated": true,
  "packetsSent": 1,
  "packetsReceived": 2,
  "bytesSent": 3,
  "bytesReceived": 4,
  "lastPacketSentTimestamp": 5,
  "lastPacketReceivedTimestamp": 6,
  "firstRequestTimestamp": 7,
  "lastRequestTimestamp": 8,
  "lastResponseTimestamp": 9,
  "totalRoundTripTime": 10,
  "currentRoundTripTime": 11,
  "availableOutgoingBitrate": 12,
  "availableIncomingBitrate": 13,
  "circuitBreakerTriggerCount": 14,
  "requestsReceived": 15,
  "requestsSent": 16,
  "responsesReceived": 17,
  "responsesSent": 18,
  "retransmissionsReceived": 19,
  "retransmissionsSent": 20,
  "consentRequestsSent": 21,
  "consentExpiredTimestamp": 22,
  "packetsDiscardedOnSend": 23,
  "bytesDiscardedOnSend": 24
}
`
	localIceCandidateStats := ICECandidateStats{
		Timestamp:     1688978831527.718,
		Type:          StatsTypeLocalCandidate,
		ID:            "ILO8S8KYr",
		TransportID:   "T01",
		NetworkType:   "wifi",
		IP:            "192.168.0.36",
		Port:          65400,
		Protocol:      "udp",
		CandidateType: ICECandidateTypeHost,
		Priority:      2122260223,
		URL:           "example.com",
		RelayProtocol: "tcp",
		Deleted:       true,
	}
	localIceCandidateStatsJSON := `
{
  "timestamp": 1688978831527.718,
  "type": "local-candidate",
  "id": "ILO8S8KYr",
  "transportId": "T01",
  "networkType": "wifi",
  "ip": "192.168.0.36",
  "port": 65400,
  "protocol": "udp",
  "candidateType": "host",
  "priority": 2122260223,
  "url": "example.com",
  "relayProtocol": "tcp",
  "deleted": true
}
`
	remoteIceCandidateStats := ICECandidateStats{
		Timestamp:     1689668364374.181,
		Type:          StatsTypeRemoteCandidate,
		ID:            "IGPGeswsH",
		TransportID:   "T01",
		IP:            "10.213.237.226",
		Port:          50618,
		Protocol:      "udp",
		CandidateType: ICECandidateTypeHost,
		Priority:      2122194687,
		URL:           "example.com",
		RelayProtocol: "tcp",
		Deleted:       true,
	}
	remoteIceCandidateStatsJSON := `
{
  "timestamp": 1689668364374.181,
  "type": "remote-candidate",
  "id": "IGPGeswsH",
  "transportId": "T01",
  "ip": "10.213.237.226",
  "port": 50618,
  "protocol": "udp",
  "candidateType": "host",
  "priority": 2122194687,
  "url": "example.com",
  "relayProtocol": "tcp",
  "deleted": true
}
`
	certificateStats := CertificateStats{
		Timestamp:            1689668364374.479,
		Type:                 StatsTypeCertificate,
		ID:                   "CF23:AB:FA:0B:0E:DF:12:34:D3:6C:EA:83:43:BD:79:39:87:39:11:49:41:8A:63:0E:17:B1:3F:94:FA:E3:62:20",
		Fingerprint:          "23:AB:FA:0B:0E:DF:12:34:D3:6C:EA:83:43:BD:79:39:87:39:11:49:41:8A:63:0E:17:B1:3F:94:FA:E3:62:20",
		FingerprintAlgorithm: "sha-256",
		Base64Certificate:    "MIIBFjCBvKADAgECAggAwlrxojpmgTAKBggqhkjOPQQDAjARMQ8wDQYDVQQDDAZXZWJSVEMwHhcNMjMwNzE3MDgxODU2WhcNMjMwODE3MDgxODU2WjARMQ8wDQYDVQQDDAZXZWJSVEMwWTATBgcqhkjOPQIBBggqhkjOPQMBBwNCAARKETeS9qNGe3ltwp+q2KgsYWsJLFCJGap4L2aa862sPijHeuzLgO2bju/mosJN0Li7mXhuKBOsCkCMU7vZHVVVMAoGCCqGSM49BAMCA0kAMEYCIQDXyuyMMrgzd+w3c4h3vPn9AzLcf9CHVHRGYyy5ReI/hgIhALkXfaZ96TQRf5FI2mBJJUX9O/q4Poe3wNZxxWeDcYN+",
		IssuerCertificateID:  "CF62:AF:88:F7:F3:0F:D6:C4:93:91:1E:AD:52:F0:A4:12:04:F9:48:E7:06:16:BA:A3:86:26:8F:1E:38:1C:48:49",
	}
	certificateStatsJSON := `
{
  "timestamp": 1689668364374.479,
  "type": "certificate",
  "id": "CF23:AB:FA:0B:0E:DF:12:34:D3:6C:EA:83:43:BD:79:39:87:39:11:49:41:8A:63:0E:17:B1:3F:94:FA:E3:62:20",
  "fingerprint": "23:AB:FA:0B:0E:DF:12:34:D3:6C:EA:83:43:BD:79:39:87:39:11:49:41:8A:63:0E:17:B1:3F:94:FA:E3:62:20",
  "fingerprintAlgorithm": "sha-256",
  "base64Certificate": "MIIBFjCBvKADAgECAggAwlrxojpmgTAKBggqhkjOPQQDAjARMQ8wDQYDVQQDDAZXZWJSVEMwHhcNMjMwNzE3MDgxODU2WhcNMjMwODE3MDgxODU2WjARMQ8wDQYDVQQDDAZXZWJSVEMwWTATBgcqhkjOPQIBBggqhkjOPQMBBwNCAARKETeS9qNGe3ltwp+q2KgsYWsJLFCJGap4L2aa862sPijHeuzLgO2bju/mosJN0Li7mXhuKBOsCkCMU7vZHVVVMAoGCCqGSM49BAMCA0kAMEYCIQDXyuyMMrgzd+w3c4h3vPn9AzLcf9CHVHRGYyy5ReI/hgIhALkXfaZ96TQRf5FI2mBJJUX9O/q4Poe3wNZxxWeDcYN+",
  "issuerCertificateId": "CF62:AF:88:F7:F3:0F:D6:C4:93:91:1E:AD:52:F0:A4:12:04:F9:48:E7:06:16:BA:A3:86:26:8F:1E:38:1C:48:49"
}
`

	return []statSample{
		{
			name:  "codec_stats",
			stats: codecStats,
			json:  codecStatsJSON,
		},
		{
			name:  "inbound_rtp_stream_stats",
			stats: inboundRTPStreamStats,
			json:  inboundRTPStreamStatsJSON,
		},
		{
			name:  "outbound_rtp_stream_stats",
			stats: outboundRTPStreamStats,
			json:  outboundRTPStreamStatsJSON,
		},
		{
			name:  "remote_inbound_rtp_stream_stats",
			stats: remoteInboundRTPStreamStats,
			json:  remoteInboundRTPStreamStatsJSON,
		},
		{
			name:  "remote_outbound_rtp_stream_stats",
			stats: remoteOutboundRTPStreamStats,
			json:  remoteOutboundRTPStreamStatsJSON,
		},
		{
			name:  "rtp_contributing_source_stats",
			stats: csrcStats,
			json:  csrcStatsJSON,
		},
		{
			name:  "audio_source_stats",
			stats: audioSourceStats,
			json:  audioSourceStatsJSON,
		},
		{
			name:  "video_source_stats",
			stats: videoSourceStats,
			json:  videoSourceStatsJSON,
		},
		{
			name:  "audio_playout_stats",
			stats: audioPlayoutStats,
			json:  audioPlayoutStatsJSON,
		},
		{
			name:  "peer_connection_stats",
			stats: peerConnectionStats,
			json:  peerConnectionStatsJSON,
		},
		{
			name:  "data_channel_stats",
			stats: dataChannelStats,
			json:  dataChannelStatsJSON,
		},
		{
			name:  "media_stream_stats",
			stats: streamStats,
			json:  streamStatsJSON,
		},
		{
			name:  "sender_video_track_stats",
			stats: senderVideoTrackAttachmentStats,
			json:  senderVideoTrackAttachmentStatsJSON,
		},
		{
			name:  "sender_audio_track_stats",
			stats: senderAudioTrackAttachmentStats,
			json:  senderAudioTrackAttachmentStatsJSON,
		},
		{
			name:  "receiver_video_track_stats",
			stats: videoSenderStats,
			json:  videoSenderStatsJSON,
		},
		{
			name:  "receiver_audio_track_stats",
			stats: audioSenderStats,
			json:  audioSenderStatsJSON,
		},
		{
			name:  "receiver_video_track_stats",
			stats: videoReceiverStats,
			json:  videoReceiverStatsJSON,
		},
		{
			name:  "receiver_audio_track_stats",
			stats: audioReceiverStats,
			json:  audioReceiverStatsJSON,
		},
		{
			name:  "transport_stats",
			stats: transportStats,
			json:  transportStatsJSON,
		},
		{
			name:  "ice_candidate_pair_stats",
			stats: iceCandidatePairStats,
			json:  iceCandidatePairStatsJSON,
		},
		{
			name:  "local_ice_candidate_stats",
			stats: localIceCandidateStats,
			json:  localIceCandidateStatsJSON,
		},
		{
			name:  "remote_ice_candidate_stats",
			stats: remoteIceCandidateStats,
			json:  remoteIceCandidateStatsJSON,
		},
		{
			name:  "certificate_stats",
			stats: certificateStats,
			json:  certificateStatsJSON,
		},
	}
}

func TestStatsMarshal(t *testing.T) {
	for _, test := range getStatsSamples() {
		t.Run(test.name+"_marshal", func(t *testing.T) {
			actualJSON, err := json.Marshal(test.stats)
			require.NoError(t, err)

			assert.JSONEq(t, test.json, string(actualJSON))
		})
	}
}

func TestStatsUnmarshal(t *testing.T) {
	for _, test := range getStatsSamples() {
		t.Run(test.name+"_unmarshal", func(t *testing.T) {
			actualStats, err := UnmarshalStatsJSON([]byte(test.json))
			require.NoError(t, err)

			assert.Equal(t, test.stats, actualStats)
		})
	}
}

func waitWithTimeout(t *testing.T, wg *sync.WaitGroup) {
	// Wait for all of the event handlers to be triggered.
	done := make(chan struct{})
	go func() {
		wg.Wait()
		done <- struct{}{}
	}()
	timeout := time.After(5 * time.Second)
	select {
	case <-done:
		break
	case <-timeout:
		t.Fatal("timed out waiting for waitgroup")
	}
}

func getConnectionStats(t *testing.T, report StatsReport, pc *PeerConnection) PeerConnectionStats {
	stats, ok := report.GetConnectionStats(pc)
	assert.True(t, ok)
	assert.Equal(t, stats.Type, StatsTypePeerConnection)
	return stats
}

func getDataChannelStats(t *testing.T, report StatsReport, dc *DataChannel) DataChannelStats {
	stats, ok := report.GetDataChannelStats(dc)
	assert.True(t, ok)
	assert.Equal(t, stats.Type, StatsTypeDataChannel)
	return stats
}

func getCodecStats(t *testing.T, report StatsReport, c *RTPCodecParameters) CodecStats {
	stats, ok := report.GetCodecStats(c)
	assert.True(t, ok)
	assert.Equal(t, stats.Type, StatsTypeCodec)
	return stats
}

func getTransportStats(t *testing.T, report StatsReport, statsID string) TransportStats {
	stats, ok := report[statsID]
	assert.True(t, ok)
	transportStats, ok := stats.(TransportStats)
	assert.True(t, ok)
	assert.Equal(t, transportStats.Type, StatsTypeTransport)
	return transportStats
}

func getSctpTransportStats(t *testing.T, report StatsReport) SCTPTransportStats {
	stats, ok := report["sctpTransport"]
	assert.True(t, ok)
	transportStats, ok := stats.(SCTPTransportStats)
	assert.True(t, ok)
	assert.Equal(t, transportStats.Type, StatsTypeSCTPTransport)
	return transportStats
}

func getCertificateStats(t *testing.T, report StatsReport, certificate *Certificate) CertificateStats {
	certificateStats, ok := report.GetCertificateStats(certificate)
	assert.True(t, ok)
	assert.Equal(t, certificateStats.Type, StatsTypeCertificate)
	return certificateStats
}

func findLocalCandidateStats(report StatsReport) []ICECandidateStats {
	result := []ICECandidateStats{}
	for _, s := range report {
		stats, ok := s.(ICECandidateStats)
		if ok && stats.Type == StatsTypeLocalCandidate {
			result = append(result, stats)
		}
	}
	return result
}

func findRemoteCandidateStats(report StatsReport) []ICECandidateStats {
	result := []ICECandidateStats{}
	for _, s := range report {
		stats, ok := s.(ICECandidateStats)
		if ok && stats.Type == StatsTypeRemoteCandidate {
			result = append(result, stats)
		}
	}
	return result
}

func findCandidatePairStats(t *testing.T, report StatsReport) []ICECandidatePairStats {
	result := []ICECandidatePairStats{}
	for _, s := range report {
		stats, ok := s.(ICECandidatePairStats)
		if ok {
			assert.Equal(t, StatsTypeCandidatePair, stats.Type)
			result = append(result, stats)
		}
	}
	return result
}

func signalPairForStats(pcOffer *PeerConnection, pcAnswer *PeerConnection) error {
	offerChan := make(chan SessionDescription)
	pcOffer.OnICECandidate(func(candidate *ICECandidate) {
		if candidate == nil {
			offerChan <- *pcOffer.PendingLocalDescription()
		}
	})

	offer, err := pcOffer.CreateOffer(nil)
	if err != nil {
		return err
	}
	if err := pcOffer.SetLocalDescription(offer); err != nil {
		return err
	}

	timeout := time.After(3 * time.Second)
	select {
	case <-timeout:
		return errReceiveOfferTimeout
	case offer := <-offerChan:
		if err := pcAnswer.SetRemoteDescription(offer); err != nil {
			return err
		}

		answer, err := pcAnswer.CreateAnswer(nil)
		if err != nil {
			return err
		}

		if err = pcAnswer.SetLocalDescription(answer); err != nil {
			return err
		}

		err = pcOffer.SetRemoteDescription(answer)
		if err != nil {
			return err
		}
		return nil
	}
}

func TestStatsConvertState(t *testing.T) {
	testCases := []struct {
		ice   ice.CandidatePairState
		stats StatsICECandidatePairState
	}{
		{
			ice.CandidatePairStateWaiting,
			StatsICECandidatePairStateWaiting,
		},
		{
			ice.CandidatePairStateInProgress,
			StatsICECandidatePairStateInProgress,
		},
		{
			ice.CandidatePairStateFailed,
			StatsICECandidatePairStateFailed,
		},
		{
			ice.CandidatePairStateSucceeded,
			StatsICECandidatePairStateSucceeded,
		},
	}

	s, err := toStatsICECandidatePairState(ice.CandidatePairState(42))

	assert.Error(t, err)
	assert.Equal(t,
		StatsICECandidatePairState("Unknown"),
		s)
	for i, testCase := range testCases {
		s, err := toStatsICECandidatePairState(testCase.ice)
		assert.NoError(t, err)
		assert.Equal(t,
			testCase.stats,
			s,
			"testCase: %d %v", i, testCase,
		)
	}
}

func TestPeerConnection_GetStats(t *testing.T) {
	offerPC, answerPC, err := newPair()
	assert.NoError(t, err)

	track1, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: MimeTypeVP8}, "video", "pion1")
	require.NoError(t, err)

	_, err = offerPC.AddTrack(track1)
	require.NoError(t, err)

	baseLineReportPCOffer := offerPC.GetStats()
	baseLineReportPCAnswer := answerPC.GetStats()

	connStatsOffer := getConnectionStats(t, baseLineReportPCOffer, offerPC)
	connStatsAnswer := getConnectionStats(t, baseLineReportPCAnswer, answerPC)

	for _, connStats := range []PeerConnectionStats{connStatsOffer, connStatsAnswer} {
		assert.Equal(t, uint32(0), connStats.DataChannelsOpened)
		assert.Equal(t, uint32(0), connStats.DataChannelsClosed)
		assert.Equal(t, uint32(0), connStats.DataChannelsRequested)
		assert.Equal(t, uint32(0), connStats.DataChannelsAccepted)
	}

	// Create a DC, open it and send a message
	offerDC, err := offerPC.CreateDataChannel("offerDC", nil)
	assert.NoError(t, err)

	msg := []byte("a classic test message")
	offerDC.OnOpen(func() {
		assert.NoError(t, offerDC.Send(msg))
	})

	dcWait := sync.WaitGroup{}
	dcWait.Add(1)

	answerDCChan := make(chan *DataChannel)
	answerPC.OnDataChannel(func(d *DataChannel) {
		d.OnOpen(func() {
			answerDCChan <- d
		})
		d.OnMessage(func(DataChannelMessage) {
			dcWait.Done()
		})
	})

	assert.NoError(t, signalPairForStats(offerPC, answerPC))
	waitWithTimeout(t, &dcWait)

	answerDC := <-answerDCChan

	reportPCOffer := offerPC.GetStats()
	reportPCAnswer := answerPC.GetStats()

	connStatsOffer = getConnectionStats(t, reportPCOffer, offerPC)
	assert.Equal(t, uint32(1), connStatsOffer.DataChannelsOpened)
	assert.Equal(t, uint32(0), connStatsOffer.DataChannelsClosed)
	assert.Equal(t, uint32(1), connStatsOffer.DataChannelsRequested)
	assert.Equal(t, uint32(0), connStatsOffer.DataChannelsAccepted)
	dcStatsOffer := getDataChannelStats(t, reportPCOffer, offerDC)
	assert.Equal(t, DataChannelStateOpen, dcStatsOffer.State)
	assert.Equal(t, uint32(1), dcStatsOffer.MessagesSent)
	assert.Equal(t, uint64(len(msg)), dcStatsOffer.BytesSent)
	assert.NotEmpty(t, findLocalCandidateStats(reportPCOffer))
	assert.NotEmpty(t, findRemoteCandidateStats(reportPCOffer))
	assert.NotEmpty(t, findCandidatePairStats(t, reportPCOffer))

	connStatsAnswer = getConnectionStats(t, reportPCAnswer, answerPC)
	assert.Equal(t, uint32(1), connStatsAnswer.DataChannelsOpened)
	assert.Equal(t, uint32(0), connStatsAnswer.DataChannelsClosed)
	assert.Equal(t, uint32(0), connStatsAnswer.DataChannelsRequested)
	assert.Equal(t, uint32(1), connStatsAnswer.DataChannelsAccepted)
	dcStatsAnswer := getDataChannelStats(t, reportPCAnswer, answerDC)
	assert.Equal(t, DataChannelStateOpen, dcStatsAnswer.State)
	assert.Equal(t, uint32(1), dcStatsAnswer.MessagesReceived)
	assert.Equal(t, uint64(len(msg)), dcStatsAnswer.BytesReceived)
	assert.NotEmpty(t, findLocalCandidateStats(reportPCAnswer))
	assert.NotEmpty(t, findRemoteCandidateStats(reportPCAnswer))
	assert.NotEmpty(t, findCandidatePairStats(t, reportPCAnswer))
	assert.NoError(t, err)
	for i := range offerPC.api.mediaEngine.videoCodecs {
		codecStat := getCodecStats(t, reportPCOffer, &(offerPC.api.mediaEngine.videoCodecs[i]))
		assert.NotEmpty(t, codecStat)
	}
	for i := range offerPC.api.mediaEngine.audioCodecs {
		codecStat := getCodecStats(t, reportPCOffer, &(offerPC.api.mediaEngine.audioCodecs[i]))
		assert.NotEmpty(t, codecStat)
	}

	// Close answer DC now
	dcWait = sync.WaitGroup{}
	dcWait.Add(1)
	offerDC.OnClose(func() {
		dcWait.Done()
	})
	assert.NoError(t, answerDC.Close())
	waitWithTimeout(t, &dcWait)
	time.Sleep(10 * time.Millisecond)

	reportPCOffer = offerPC.GetStats()
	reportPCAnswer = answerPC.GetStats()

	connStatsOffer = getConnectionStats(t, reportPCOffer, offerPC)
	assert.Equal(t, uint32(1), connStatsOffer.DataChannelsOpened)
	assert.Equal(t, uint32(1), connStatsOffer.DataChannelsClosed)
	assert.Equal(t, uint32(1), connStatsOffer.DataChannelsRequested)
	assert.Equal(t, uint32(0), connStatsOffer.DataChannelsAccepted)
	dcStatsOffer = getDataChannelStats(t, reportPCOffer, offerDC)
	assert.Equal(t, DataChannelStateClosed, dcStatsOffer.State)

	connStatsAnswer = getConnectionStats(t, reportPCAnswer, answerPC)
	assert.Equal(t, uint32(1), connStatsAnswer.DataChannelsOpened)
	assert.Equal(t, uint32(1), connStatsAnswer.DataChannelsClosed)
	assert.Equal(t, uint32(0), connStatsAnswer.DataChannelsRequested)
	assert.Equal(t, uint32(1), connStatsAnswer.DataChannelsAccepted)
	dcStatsAnswer = getDataChannelStats(t, reportPCAnswer, answerDC)
	assert.Equal(t, DataChannelStateClosed, dcStatsAnswer.State)

	answerICETransportStats := getTransportStats(t, reportPCAnswer, "iceTransport")
	offerICETransportStats := getTransportStats(t, reportPCOffer, "iceTransport")
	assert.GreaterOrEqual(t, offerICETransportStats.BytesSent, answerICETransportStats.BytesReceived)
	assert.GreaterOrEqual(t, answerICETransportStats.BytesSent, offerICETransportStats.BytesReceived)

	answerSCTPTransportStats := getSctpTransportStats(t, reportPCAnswer)
	offerSCTPTransportStats := getSctpTransportStats(t, reportPCOffer)
	assert.GreaterOrEqual(t, offerSCTPTransportStats.BytesSent, answerSCTPTransportStats.BytesReceived)
	assert.GreaterOrEqual(t, answerSCTPTransportStats.BytesSent, offerSCTPTransportStats.BytesReceived)

	certificates := offerPC.configuration.Certificates

	for i := range certificates {
		assert.NotEmpty(t, getCertificateStats(t, reportPCOffer, &certificates[i]))
	}

	closePairNow(t, offerPC, answerPC)
}

func TestPeerConnection_GetStats_Closed(t *testing.T) {
	pc, err := NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	assert.NoError(t, pc.Close())

	pc.GetStats()
}
