// +build !js

package webrtc

import (
	"strings"
	"testing"
	"time"

	"github.com/pion/sdp/v3"
	"github.com/pion/transport/test"
	"github.com/stretchr/testify/assert"
)

func TestSDPSemantics_String(t *testing.T) {
	testCases := []struct {
		value          SDPSemantics
		expectedString string
	}{
		{SDPSemantics(42), unknownStr},
		{SDPSemanticsUnifiedPlanWithFallback, "unified-plan-with-fallback"},
		{SDPSemanticsPlanB, "plan-b"},
		{SDPSemanticsUnifiedPlan, "unified-plan"},
	}

	for i, testCase := range testCases {
		assert.Equal(t,
			testCase.expectedString,
			testCase.value.String(),
			"testCase: %d %v", i, testCase,
		)
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
