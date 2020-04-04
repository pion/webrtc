// +build !js

package webrtc

import (
	"strings"
	"testing"

	"github.com/pion/sdp/v2"

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
		if attr.Key == ssrcStr {
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
	opc, err := NewPeerConnection(Configuration{
		SDPSemantics: SDPSemanticsPlanB,
	})
	if err != nil {
		t.Errorf("NewPeerConnection failed: %v", err)
	}

	if _, err = opc.AddTransceiverFromKind(RTPCodecTypeVideo, RtpTransceiverInit{
		Direction: RTPTransceiverDirectionSendrecv,
	}); err != nil {
		t.Errorf("AddTransceiver failed: %v", err)
	}
	if _, err = opc.AddTransceiverFromKind(RTPCodecTypeVideo, RtpTransceiverInit{
		Direction: RTPTransceiverDirectionSendrecv,
	}); err != nil {
		t.Errorf("AddTransceiver failed: %v", err)
	}
	if _, err = opc.AddTransceiverFromKind(RTPCodecTypeAudio, RtpTransceiverInit{
		Direction: RTPTransceiverDirectionSendrecv,
	}); err != nil {
		t.Errorf("AddTransceiver failed: %v", err)
	}
	if _, err = opc.AddTransceiverFromKind(RTPCodecTypeAudio, RtpTransceiverInit{
		Direction: RTPTransceiverDirectionSendrecv,
	}); err != nil {
		t.Errorf("AddTransceiver failed: %v", err)
	}

	offer, err := opc.CreateOffer(nil)
	if err != nil {
		t.Errorf("Plan B CreateOffer failed: %s", err)
	}

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
	if err != nil {
		t.Errorf("NewPeerConnection failed: %v", err)
	}

	if err = apc.SetRemoteDescription(offer); err != nil {
		t.Errorf("SetRemoteDescription failed: %s", err)
	}

	answer, err := apc.CreateAnswer(nil)
	if err != nil {
		t.Errorf("Plan B CreateAnswer failed: %s", err)
	}

	mdNames = getMdNames(answer.parsed)
	assert.ObjectsAreEqual(mdNames, []string{"video", "audio", "data"})
}

func TestSDPSemantics_PlanBAnswerSenders(t *testing.T) {
	opc, err := NewPeerConnection(Configuration{
		SDPSemantics: SDPSemanticsPlanB,
	})
	if err != nil {
		t.Errorf("NewPeerConnection failed: %v", err)
	}

	if _, err = opc.AddTransceiverFromKind(RTPCodecTypeVideo, RtpTransceiverInit{
		Direction: RTPTransceiverDirectionRecvonly,
	}); err != nil {
		t.Errorf("Failed to add transceiver")
	}
	if _, err = opc.AddTransceiverFromKind(RTPCodecTypeAudio, RtpTransceiverInit{
		Direction: RTPTransceiverDirectionRecvonly,
	}); err != nil {
		t.Errorf("Failed to add transceiver")
	}

	offer, err := opc.CreateOffer(nil)
	if err != nil {
		t.Errorf("Plan B CreateOffer failed: %s", err)
	}

	mdNames := getMdNames(offer.parsed)
	assert.ObjectsAreEqual(mdNames, []string{"video", "audio", "data"})

	apc, err := NewPeerConnection(Configuration{
		SDPSemantics: SDPSemanticsPlanB,
	})
	if err != nil {
		t.Errorf("NewPeerConnection failed: %v", err)
	}

	video1, err := NewTrack(DefaultPayloadTypeH264, 1, "1", "1", NewRTPH264Codec(DefaultPayloadTypeH264, 90000))
	if err != nil {
		t.Errorf("Failed to create video track")
	}
	if _, err = apc.AddTrack(video1); err != nil {
		t.Errorf("Failed to add video track")
	}
	video2, err := NewTrack(DefaultPayloadTypeH264, 2, "2", "2", NewRTPH264Codec(DefaultPayloadTypeH264, 90000))
	if err != nil {
		t.Errorf("Failed to create video track")
	}
	if _, err = apc.AddTrack(video2); err != nil {
		t.Errorf("Failed to add video track")
	}
	audio1, err := NewTrack(DefaultPayloadTypeOpus, 3, "3", "3", NewRTPOpusCodec(DefaultPayloadTypeOpus, 48000))
	if err != nil {
		t.Errorf("Failed to create audio track")
	}
	if _, err = apc.AddTrack(audio1); err != nil {
		t.Errorf("Failed to add audio track")
	}
	audio2, err := NewTrack(DefaultPayloadTypeOpus, 4, "4", "4", NewRTPOpusCodec(DefaultPayloadTypeOpus, 48000))
	if err != nil {
		t.Errorf("Failed to create audio track")
	}
	if _, err = apc.AddTrack(audio2); err != nil {
		t.Errorf("Failed to add audio track")
	}

	if err = apc.SetRemoteDescription(offer); err != nil {
		t.Errorf("SetRemoteDescription failed: %s", err)
	}

	answer, err := apc.CreateAnswer(nil)
	if err != nil {
		t.Errorf("Plan B CreateAnswer failed: %s", err)
	}

	mdNames = getMdNames(answer.parsed)
	assert.ObjectsAreEqual(mdNames, []string{"video", "audio", "data"})

	// Verify that each section has 2 SSRCs (one for each sender)
	for _, section := range []string{"video", "audio"} {
		for _, media := range answer.parsed.MediaDescriptions {
			if media.MediaName.Media == section {
				assert.Lenf(t, extractSsrcList(media), 2, "%q should have 2 SSRCs in Plan-B mode", section)
			}
		}
	}
}

func TestSDPSemantics_UnifiedPlanWithFallback(t *testing.T) {
	opc, err := NewPeerConnection(Configuration{
		SDPSemantics: SDPSemanticsPlanB,
	})
	if err != nil {
		t.Errorf("NewPeerConnection failed: %v", err)
	}

	if _, err = opc.AddTransceiverFromKind(RTPCodecTypeVideo, RtpTransceiverInit{
		Direction: RTPTransceiverDirectionRecvonly,
	}); err != nil {
		t.Errorf("Failed to add transceiver")
	}
	if _, err = opc.AddTransceiverFromKind(RTPCodecTypeAudio, RtpTransceiverInit{
		Direction: RTPTransceiverDirectionRecvonly,
	}); err != nil {
		t.Errorf("Failed to add transceiver")
	}

	offer, err := opc.CreateOffer(nil)
	if err != nil {
		t.Errorf("Plan B CreateOffer failed: %s", err)
	}

	mdNames := getMdNames(offer.parsed)
	assert.ObjectsAreEqual(mdNames, []string{"video", "audio", "data"})

	apc, err := NewPeerConnection(Configuration{
		SDPSemantics: SDPSemanticsUnifiedPlanWithFallback,
	})
	if err != nil {
		t.Errorf("NewPeerConnection failed: %v", err)
	}

	video1, err := NewTrack(DefaultPayloadTypeH264, 1, "1", "1", NewRTPH264Codec(DefaultPayloadTypeH264, 90000))
	if err != nil {
		t.Errorf("Failed to create video track")
	}
	if _, err = apc.AddTrack(video1); err != nil {
		t.Errorf("Failed to add video track")
	}
	video2, err := NewTrack(DefaultPayloadTypeH264, 2, "2", "2", NewRTPH264Codec(DefaultPayloadTypeH264, 90000))
	if err != nil {
		t.Errorf("Failed to create video track")
	}
	if _, err = apc.AddTrack(video2); err != nil {
		t.Errorf("Failed to add video track")
	}
	audio1, err := NewTrack(DefaultPayloadTypeOpus, 3, "3", "3", NewRTPOpusCodec(DefaultPayloadTypeOpus, 48000))
	if err != nil {
		t.Errorf("Failed to create audio track")
	}
	if _, err = apc.AddTrack(audio1); err != nil {
		t.Errorf("Failed to add audio track")
	}
	audio2, err := NewTrack(DefaultPayloadTypeOpus, 4, "4", "4", NewRTPOpusCodec(DefaultPayloadTypeOpus, 48000))
	if err != nil {
		t.Errorf("Failed to create audio track")
	}
	if _, err = apc.AddTrack(audio2); err != nil {
		t.Errorf("Failed to add audio track")
	}

	if err = apc.SetRemoteDescription(offer); err != nil {
		t.Errorf("SetRemoteDescription failed: %s", err)
	}

	answer, err := apc.CreateAnswer(nil)
	if err != nil {
		t.Errorf("Plan B CreateAnswer failed: %s", err)
	}

	mdNames = getMdNames(answer.parsed)
	assert.ObjectsAreEqual(mdNames, []string{"video", "audio", "data"})

	extractSsrcList := func(md *sdp.MediaDescription) []string {
		ssrcMap := map[string]struct{}{}
		for _, attr := range md.Attributes {
			if attr.Key == ssrcStr {
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
}
