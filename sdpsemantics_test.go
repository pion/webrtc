// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js
// +build !js

package webrtc

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/pion/sdp/v3"
	"github.com/pion/transport/v2/test"
	"github.com/stretchr/testify/assert"
)

func TestSDPSemantics_String(t *testing.T) {
	testCases := []struct {
		value          SDPSemantics
		expectedString string
	}{
		{SDPSemanticsUnifiedPlanWithFallback, "unified-plan-with-fallback"},
		{SDPSemanticsPlanB, "plan-b"},
		{SDPSemanticsUnifiedPlan, "unified-plan"},
	}

	assert.Equal(t,
		unknownStr,
		SDPSemantics(42).String(),
	)

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.expectedString,
			testCase.value.String(),
			"testCase: %d %v", i, testCase,
		)
		assert.Equal(t,
			testCase.value,
			newSDPSemantics(testCase.expectedString),
			"testCase: %d %v", i, testCase,
		)
	}
}

func TestSDPSemantics_JSON(t *testing.T) {
	testCases := []struct {
		value SDPSemantics
		JSON  []byte
	}{
		{SDPSemanticsUnifiedPlanWithFallback, []byte("\"unified-plan-with-fallback\"")},
		{SDPSemanticsPlanB, []byte("\"plan-b\"")},
		{SDPSemanticsUnifiedPlan, []byte("\"unified-plan\"")},
	}

	for i, testCase := range testCases {
		res, err := json.Marshal(testCase.value)
		assert.NoError(t, err)
		assert.Equal(t,
			testCase.JSON,
			res,
			"testCase: %d %v", i, testCase,
		)

		var v SDPSemantics
		err = json.Unmarshal(testCase.JSON, &v)
		assert.NoError(t, err)
		assert.Equal(t, v, testCase.value)
	}
}

// The following tests are for non-standard SDP semantics
// (i.e. not unified-unified)

func getMdNames(sdp *sdp.SessionDescription) []string {
	mdNames := make([]string, 0, len(sdp.MediaDescriptions))
	for _, media := range sdp.MediaDescriptions {
		mdNames = append(mdNames, media.MediaName.Media)
	}
	return mdNames
}

func extractSsrcList(md *sdp.MediaDescription) []string {
	ssrcMap := map[string]struct{}{}
	for _, attr := range md.Attributes {
		if attr.Key == sdp.AttrKeySSRC {
			ssrc := strings.Fields(attr.Value)[0]
			ssrcMap[ssrc] = struct{}{}
		}
	}
	ssrcList := make([]string, 0, len(ssrcMap))
	for ssrc := range ssrcMap {
		ssrcList = append(ssrcList, ssrc)
	}
	return ssrcList
}

func TestSDPSemantics_PlanBOfferTransceivers(t *testing.T) {
	report := test.CheckRoutines(t)
	defer report()

	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	opc, err := NewPeerConnection(Configuration{
		SDPSemantics: SDPSemanticsPlanB,
	})
	assert.NoError(t, err)

	_, err = opc.AddTransceiverFromKind(RTPCodecTypeVideo, RTPTransceiverInit{
		Direction: RTPTransceiverDirectionSendrecv,
	})
	assert.NoError(t, err)

	_, err = opc.AddTransceiverFromKind(RTPCodecTypeVideo, RTPTransceiverInit{
		Direction: RTPTransceiverDirectionSendrecv,
	})
	assert.NoError(t, err)

	_, err = opc.AddTransceiverFromKind(RTPCodecTypeAudio, RTPTransceiverInit{
		Direction: RTPTransceiverDirectionSendrecv,
	})
	assert.NoError(t, err)

	_, err = opc.AddTransceiverFromKind(RTPCodecTypeAudio, RTPTransceiverInit{
		Direction: RTPTransceiverDirectionSendrecv,
	})
	assert.NoError(t, err)

	offer, err := opc.CreateOffer(nil)
	assert.NoError(t, err)

	mdNames := getMdNames(offer.parsed)
	assert.ObjectsAreEqual(mdNames, []string{"video", "audio", "data"})

	// Verify that each section has 2 SSRCs (one for each transceiver)
	for _, section := range []string{"video", "audio"} {
		for _, media := range offer.parsed.MediaDescriptions {
			if media.MediaName.Media == section {
				assert.Len(t, extractSsrcList(media), 2)
			}
		}
	}

	apc, err := NewPeerConnection(Configuration{
		SDPSemantics: SDPSemanticsPlanB,
	})
	assert.NoError(t, err)

	assert.NoError(t, apc.SetRemoteDescription(offer))

	answer, err := apc.CreateAnswer(nil)
	assert.NoError(t, err)

	mdNames = getMdNames(answer.parsed)
	assert.ObjectsAreEqual(mdNames, []string{"video", "audio", "data"})

	closePairNow(t, apc, opc)
}

func TestSDPSemantics_PlanBAnswerSenders(t *testing.T) {
	report := test.CheckRoutines(t)
	defer report()

	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	opc, err := NewPeerConnection(Configuration{
		SDPSemantics: SDPSemanticsPlanB,
	})
	assert.NoError(t, err)

	_, err = opc.AddTransceiverFromKind(RTPCodecTypeVideo, RTPTransceiverInit{
		Direction: RTPTransceiverDirectionRecvonly,
	})
	assert.NoError(t, err)

	_, err = opc.AddTransceiverFromKind(RTPCodecTypeAudio, RTPTransceiverInit{
		Direction: RTPTransceiverDirectionRecvonly,
	})
	assert.NoError(t, err)

	offer, err := opc.CreateOffer(nil)
	assert.NoError(t, err)

	assert.ObjectsAreEqual(getMdNames(offer.parsed), []string{"video", "audio", "data"})

	apc, err := NewPeerConnection(Configuration{
		SDPSemantics: SDPSemanticsPlanB,
	})
	assert.NoError(t, err)

	video1, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: MimeTypeH264, SDPFmtpLine: "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42001f"}, "1", "1")
	assert.NoError(t, err)

	_, err = apc.AddTrack(video1)
	assert.NoError(t, err)

	video2, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: MimeTypeH264, SDPFmtpLine: "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42001f"}, "2", "2")
	assert.NoError(t, err)

	_, err = apc.AddTrack(video2)
	assert.NoError(t, err)

	audio1, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: MimeTypeOpus}, "3", "3")
	assert.NoError(t, err)

	_, err = apc.AddTrack(audio1)
	assert.NoError(t, err)

	audio2, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: MimeTypeOpus}, "4", "4")
	assert.NoError(t, err)

	_, err = apc.AddTrack(audio2)
	assert.NoError(t, err)

	assert.NoError(t, apc.SetRemoteDescription(offer))

	answer, err := apc.CreateAnswer(nil)
	assert.NoError(t, err)

	assert.ObjectsAreEqual(getMdNames(answer.parsed), []string{"video", "audio", "data"})

	// Verify that each section has 2 SSRCs (one for each sender)
	for _, section := range []string{"video", "audio"} {
		for _, media := range answer.parsed.MediaDescriptions {
			if media.MediaName.Media == section {
				assert.Lenf(t, extractSsrcList(media), 2, "%q should have 2 SSRCs in Plan-B mode", section)
			}
		}
	}

	closePairNow(t, apc, opc)
}

func TestSDPSemantics_UnifiedPlanWithFallback(t *testing.T) {
	report := test.CheckRoutines(t)
	defer report()

	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	opc, err := NewPeerConnection(Configuration{
		SDPSemantics: SDPSemanticsPlanB,
	})
	assert.NoError(t, err)

	_, err = opc.AddTransceiverFromKind(RTPCodecTypeVideo, RTPTransceiverInit{
		Direction: RTPTransceiverDirectionRecvonly,
	})
	assert.NoError(t, err)

	_, err = opc.AddTransceiverFromKind(RTPCodecTypeAudio, RTPTransceiverInit{
		Direction: RTPTransceiverDirectionRecvonly,
	})
	assert.NoError(t, err)

	offer, err := opc.CreateOffer(nil)
	assert.NoError(t, err)

	assert.ObjectsAreEqual(getMdNames(offer.parsed), []string{"video", "audio", "data"})

	apc, err := NewPeerConnection(Configuration{
		SDPSemantics: SDPSemanticsUnifiedPlanWithFallback,
	})
	assert.NoError(t, err)

	video1, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: MimeTypeH264, SDPFmtpLine: "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42001f"}, "1", "1")
	assert.NoError(t, err)

	_, err = apc.AddTrack(video1)
	assert.NoError(t, err)

	video2, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: MimeTypeH264, SDPFmtpLine: "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42001f"}, "2", "2")
	assert.NoError(t, err)

	_, err = apc.AddTrack(video2)
	assert.NoError(t, err)

	audio1, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: MimeTypeOpus}, "3", "3")
	assert.NoError(t, err)

	_, err = apc.AddTrack(audio1)
	assert.NoError(t, err)

	audio2, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: MimeTypeOpus}, "4", "4")
	assert.NoError(t, err)

	_, err = apc.AddTrack(audio2)
	assert.NoError(t, err)

	assert.NoError(t, apc.SetRemoteDescription(offer))

	answer, err := apc.CreateAnswer(nil)
	assert.NoError(t, err)

	assert.ObjectsAreEqual(getMdNames(answer.parsed), []string{"video", "audio", "data"})

	extractSsrcList := func(md *sdp.MediaDescription) []string {
		ssrcMap := map[string]struct{}{}
		for _, attr := range md.Attributes {
			if attr.Key == sdp.AttrKeySSRC {
				ssrc := strings.Fields(attr.Value)[0]
				ssrcMap[ssrc] = struct{}{}
			}
		}
		ssrcList := make([]string, 0, len(ssrcMap))
		for ssrc := range ssrcMap {
			ssrcList = append(ssrcList, ssrc)
		}
		return ssrcList
	}
	// Verify that each section has 2 SSRCs (one for each sender)
	for _, section := range []string{"video", "audio"} {
		for _, media := range answer.parsed.MediaDescriptions {
			if media.MediaName.Media == section {
				assert.Lenf(t, extractSsrcList(media), 2, "%q should have 2 SSRCs in Plan-B fallback mode", section)
			}
		}
	}

	closePairNow(t, apc, opc)
}

// Assert that we can catch Remote SessionDescription that don't match our Semantics
func TestSDPSemantics_SetRemoteDescription_Mismatch(t *testing.T) {
	planBOffer := "v=0\r\no=- 4648475892259889561 3 IN IP4 127.0.0.1\r\ns=-\r\nt=0 0\r\na=group:BUNDLE video audio\r\na=ice-ufrag:1hhfzwf0ijpzm\r\na=ice-pwd:jm5puo2ab1op3vs59ca53bdk7s\r\na=fingerprint:sha-256 40:42:FB:47:87:52:BF:CB:EC:3A:DF:EB:06:DA:2D:B7:2F:59:42:10:23:7B:9D:4C:C9:58:DD:FF:A2:8F:17:67\r\nm=video 9 UDP/TLS/RTP/SAVPF 96\r\nc=IN IP4 0.0.0.0\r\na=rtcp:9 IN IP4 0.0.0.0\r\na=setup:passive\r\na=mid:video\r\na=sendonly\r\na=rtcp-mux\r\na=rtpmap:96 H264/90000\r\na=rtcp-fb:96 nack\r\na=rtcp-fb:96 goog-remb\r\na=fmtp:96 packetization-mode=1;profile-level-id=42e01f\r\na=ssrc:1505338584 cname:10000000b5810aac\r\na=ssrc:1 cname:trackB\r\nm=audio 9 UDP/TLS/RTP/SAVPF 111\r\nc=IN IP4 0.0.0.0\r\na=rtcp:9 IN IP4 0.0.0.0\r\na=setup:passive\r\na=mid:audio\r\na=sendonly\r\na=rtcp-mux\r\na=rtpmap:111 opus/48000/2\r\na=ssrc:697641945 cname:10000000b5810aac\r\n"
	unifiedPlanOffer := "v=0\r\no=- 4648475892259889561 3 IN IP4 127.0.0.1\r\ns=-\r\nt=0 0\r\na=group:BUNDLE 0 1\r\na=ice-ufrag:1hhfzwf0ijpzm\r\na=ice-pwd:jm5puo2ab1op3vs59ca53bdk7s\r\na=fingerprint:sha-256 40:42:FB:47:87:52:BF:CB:EC:3A:DF:EB:06:DA:2D:B7:2F:59:42:10:23:7B:9D:4C:C9:58:DD:FF:A2:8F:17:67\r\nm=video 9 UDP/TLS/RTP/SAVPF 96\r\nc=IN IP4 0.0.0.0\r\na=rtcp:9 IN IP4 0.0.0.0\r\na=setup:passive\r\na=mid:0\r\na=sendonly\r\na=rtcp-mux\r\na=rtpmap:96 H264/90000\r\na=rtcp-fb:96 nack\r\na=rtcp-fb:96 goog-remb\r\na=fmtp:96 packetization-mode=1;profile-level-id=42e01f\r\na=ssrc:1505338584 cname:10000000b5810aac\r\nm=audio 9 UDP/TLS/RTP/SAVPF 111\r\nc=IN IP4 0.0.0.0\r\na=rtcp:9 IN IP4 0.0.0.0\r\na=setup:passive\r\na=mid:1\r\na=sendonly\r\na=rtcp-mux\r\na=rtpmap:111 opus/48000/2\r\na=ssrc:697641945 cname:10000000b5810aac\r\n"

	report := test.CheckRoutines(t)
	defer report()

	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	t.Run("PlanB", func(t *testing.T) {
		pc, err := NewPeerConnection(Configuration{
			SDPSemantics: SDPSemanticsUnifiedPlan,
		})
		assert.NoError(t, err)

		err = pc.SetRemoteDescription(SessionDescription{SDP: planBOffer, Type: SDPTypeOffer})
		assert.NoError(t, err)

		_, err = pc.CreateAnswer(nil)
		assert.True(t, errors.Is(err, ErrIncorrectSDPSemantics))

		assert.NoError(t, pc.Close())
	})

	t.Run("UnifiedPlan", func(t *testing.T) {
		pc, err := NewPeerConnection(Configuration{
			SDPSemantics: SDPSemanticsPlanB,
		})
		assert.NoError(t, err)

		err = pc.SetRemoteDescription(SessionDescription{SDP: unifiedPlanOffer, Type: SDPTypeOffer})
		assert.NoError(t, err)

		_, err = pc.CreateAnswer(nil)
		assert.True(t, errors.Is(err, ErrIncorrectSDPSemantics))

		assert.NoError(t, pc.Close())
	})
}
