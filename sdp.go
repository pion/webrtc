// +build !js

package webrtc

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/pion/logging"
	"github.com/pion/sdp/v2"
)

type trackDetails struct {
	mid   string
	kind  RTPCodecType
	label string
	id    string
	ssrc  uint32
}

// SDPSectionType specifies media type sections
type SDPSectionType string

// Common SDP sections
const (
	SDPSectionGlobal = SDPSectionType("global")
	SDPSectionVideo  = SDPSectionType("video")
	SDPSectionAudio  = SDPSectionType("audio")
)

// extract all trackDetails from an SDP.
func trackDetailsFromSDP(log logging.LeveledLogger, s *sdp.SessionDescription) map[uint32]trackDetails {
	incomingTracks := map[uint32]trackDetails{}
	rtxRepairFlows := map[uint32]bool{}

	for _, media := range s.MediaDescriptions {
		// Plan B can have multiple tracks in a signle media section
		trackLabel := ""
		trackID := ""

		// If media section is recvonly or inactive skip
		if _, ok := media.Attribute(sdp.AttrKeyRecvOnly); ok {
			continue
		} else if _, ok := media.Attribute(sdp.AttrKeyInactive); ok {
			continue
		}

		midValue := getMidValue(media)
		if midValue == "" {
			continue
		}

		codecType := NewRTPCodecType(media.MediaName.Media)
		if codecType == 0 {
			continue
		}

		for _, attr := range media.Attributes {
			switch attr.Key {
			case sdp.AttrKeySSRCGroup:
				split := strings.Split(attr.Value, " ")
				if split[0] == sdp.SemanticTokenFlowIdentification {
					// Add rtx ssrcs to blacklist, to avoid adding them as tracks
					// Essentially lines like `a=ssrc-group:FID 2231627014 632943048` are processed by this section
					// as this declares that the second SSRC (632943048) is a rtx repair flow (RFC4588) for the first
					// (2231627014) as specified in RFC5576
					if len(split) == 3 {
						_, err := strconv.ParseUint(split[1], 10, 32)
						if err != nil {
							log.Warnf("Failed to parse SSRC: %v", err)
							continue
						}
						rtxRepairFlow, err := strconv.ParseUint(split[2], 10, 32)
						if err != nil {
							log.Warnf("Failed to parse SSRC: %v", err)
							continue
						}
						rtxRepairFlows[uint32(rtxRepairFlow)] = true
						delete(incomingTracks, uint32(rtxRepairFlow)) // Remove if rtx was added as track before
					}
				}

			// Handle `a=msid:<stream_id> <track_label>` for Unified plan. The first value is the same as MediaStream.id
			// in the browser and can be used to figure out which tracks belong to the same stream. The browser should
			// figure this out automatically when an ontrack event is emitted on RTCPeerConnection.
			case sdp.AttrKeyMsid:
				split := strings.Split(attr.Value, " ")
				if len(split) == 2 {
					trackLabel = split[0]
					trackID = split[1]
				}

			case sdp.AttrKeySSRC:
				split := strings.Split(attr.Value, " ")
				ssrc, err := strconv.ParseUint(split[0], 10, 32)
				if err != nil {
					log.Warnf("Failed to parse SSRC: %v", err)
					continue
				}
				if rtxRepairFlow := rtxRepairFlows[uint32(ssrc)]; rtxRepairFlow {
					continue // This ssrc is a RTX repair flow, ignore
				}
				if existingValues, ok := incomingTracks[uint32(ssrc)]; ok && existingValues.label != "" && existingValues.id != "" {
					continue // This ssrc is already fully defined
				}

				if len(split) == 3 && strings.HasPrefix(split[1], "msid:") {
					trackLabel = split[1][len("msid:"):]
					trackID = split[2]
				}

				// Plan B might send multiple a=ssrc lines under a single m= section. This is also why a single trackDetails{}
				// is not defined at the top of the loop over s.MediaDescriptions.
				incomingTracks[uint32(ssrc)] = trackDetails{midValue, codecType, trackLabel, trackID, uint32(ssrc)}
			}
		}
	}

	return incomingTracks
}

func addCandidatesToMediaDescriptions(candidates []ICECandidate, m *sdp.MediaDescription, iceGatheringState ICEGatheringState) {
	appendCandidateIfNew := func(c sdp.ICECandidate, attributes []sdp.Attribute) {
		marshaled := c.Marshal()
		for _, a := range attributes {
			if marshaled == a.Value {
				return
			}
		}

		m.WithICECandidate(c)
	}

	for _, c := range candidates {
		sdpCandidate := iceCandidateToSDP(c)
		sdpCandidate.ExtensionAttributes = append(sdpCandidate.ExtensionAttributes, sdp.ICECandidateAttribute{Key: "generation", Value: "0"})
		sdpCandidate.Component = 1
		appendCandidateIfNew(sdpCandidate, m.Attributes)

		sdpCandidate.Component = 2
		appendCandidateIfNew(sdpCandidate, m.Attributes)
	}

	if iceGatheringState != ICEGatheringStateComplete {
		return
	}
	for _, a := range m.Attributes {
		if a.Key == "end-of-candidates" {
			return
		}
	}

	m.WithPropertyAttribute("end-of-candidates")
}

func addDataMediaSection(d *sdp.SessionDescription, dtlsFingerprints []DTLSFingerprint, midValue string, iceParams ICEParameters, candidates []ICECandidate, dtlsRole sdp.ConnectionRole, iceGatheringState ICEGatheringState) {
	media := (&sdp.MediaDescription{
		MediaName: sdp.MediaName{
			Media:   mediaSectionApplication,
			Port:    sdp.RangedPort{Value: 9},
			Protos:  []string{"DTLS", "SCTP"},
			Formats: []string{"5000"},
		},
		ConnectionInformation: &sdp.ConnectionInformation{
			NetworkType: "IN",
			AddressType: "IP4",
			Address: &sdp.Address{
				Address: "0.0.0.0",
			},
		},
	}).
		WithValueAttribute(sdp.AttrKeyConnectionSetup, dtlsRole.String()).
		WithValueAttribute(sdp.AttrKeyMID, midValue).
		WithPropertyAttribute(RTPTransceiverDirectionSendrecv.String()).
		WithPropertyAttribute("sctpmap:5000 webrtc-datachannel 1024").
		WithICECredentials(iceParams.UsernameFragment, iceParams.Password)

	for _, f := range dtlsFingerprints {
		media = media.WithFingerprint(f.Algorithm, strings.ToUpper(f.Value))
	}

	addCandidatesToMediaDescriptions(candidates, media, iceGatheringState)
	d.WithMedia(media)
}

func populateLocalCandidates(sessionDescription *SessionDescription, i *ICEGatherer, iceGatheringState ICEGatheringState) *SessionDescription {
	if sessionDescription == nil || i == nil {
		return sessionDescription
	}

	candidates, err := i.GetLocalCandidates()
	if err != nil {
		return sessionDescription
	}

	parsed := sessionDescription.parsed
	for _, m := range parsed.MediaDescriptions {
		addCandidatesToMediaDescriptions(candidates, m, iceGatheringState)
	}
	sdp, err := parsed.Marshal()
	if err != nil {
		return sessionDescription
	}

	return &SessionDescription{
		SDP:  string(sdp),
		Type: sessionDescription.Type,
	}
}

func addTransceiverSDP(d *sdp.SessionDescription, isPlanB bool, dtlsFingerprints []DTLSFingerprint, mediaEngine *MediaEngine, midValue string, iceParams ICEParameters, candidates []ICECandidate, dtlsRole sdp.ConnectionRole, iceGatheringState ICEGatheringState, extMaps map[SDPSectionType][]sdp.ExtMap, transceivers ...*RTPTransceiver) (bool, error) {
	if len(transceivers) < 1 {
		return false, fmt.Errorf("addTransceiverSDP() called with 0 transceivers")
	}
	// Use the first transceiver to generate the section attributes
	t := transceivers[0]
	media := sdp.NewJSEPMediaDescription(t.kind.String(), []string{}).
		WithValueAttribute(sdp.AttrKeyConnectionSetup, dtlsRole.String()).
		WithValueAttribute(sdp.AttrKeyMID, midValue).
		WithICECredentials(iceParams.UsernameFragment, iceParams.Password).
		WithPropertyAttribute(sdp.AttrKeyRTCPMux).
		WithPropertyAttribute(sdp.AttrKeyRTCPRsize)

	codecs := mediaEngine.GetCodecsByKind(t.kind)
	for _, codec := range codecs {
		media.WithCodec(codec.PayloadType, codec.Name, codec.ClockRate, codec.Channels, codec.SDPFmtpLine)

		for _, feedback := range codec.RTPCodecCapability.RTCPFeedback {
			media.WithValueAttribute("rtcp-fb", fmt.Sprintf("%d %s %s", codec.PayloadType, feedback.Type, feedback.Parameter))
		}
	}
	if len(codecs) == 0 {
		// Explicitly reject track if we don't have the codec
		d.WithMedia(&sdp.MediaDescription{
			MediaName: sdp.MediaName{
				Media:   t.kind.String(),
				Port:    sdp.RangedPort{Value: 0},
				Protos:  []string{"UDP", "TLS", "RTP", "SAVPF"},
				Formats: []string{"0"},
			},
		})
		return false, nil
	}

	// Add extmaps
	if maps, ok := extMaps[SDPSectionType(t.kind.String())]; ok {
		for _, m := range maps {
			media.WithExtMap(m)
		}
	}

	for _, mt := range transceivers {
		if mt.Sender() != nil && mt.Sender().track != nil {
			track := mt.Sender().track
			media = media.WithMediaSource(track.SSRC(), track.Label() /* cname */, track.Label() /* streamLabel */, track.ID())
			if !isPlanB {
				media = media.WithPropertyAttribute("msid:" + track.Label() + " " + track.ID())
				break
			}
		}
	}

	media = media.WithPropertyAttribute(t.Direction().String())

	for _, fingerprint := range dtlsFingerprints {
		media = media.WithFingerprint(fingerprint.Algorithm, strings.ToUpper(fingerprint.Value))
	}

	addCandidatesToMediaDescriptions(candidates, media, iceGatheringState)
	d.WithMedia(media)

	return true, nil
}

type mediaSection struct {
	id           string
	transceivers []*RTPTransceiver
	data         bool
}

// populateSDP serializes a PeerConnections state into an SDP
func populateSDP(d *sdp.SessionDescription, isPlanB bool, dtlsFingerprints []DTLSFingerprint, mediaDescriptionFingerprint bool, isICELite bool, mediaEngine *MediaEngine, connectionRole sdp.ConnectionRole, candidates []ICECandidate, iceParams ICEParameters, mediaSections []mediaSection, iceGatheringState ICEGatheringState, extMaps map[SDPSectionType][]sdp.ExtMap) (*sdp.SessionDescription, error) {
	var err error
	mediaDtlsFingerprints := []DTLSFingerprint{}

	if mediaDescriptionFingerprint {
		mediaDtlsFingerprints = dtlsFingerprints
	}

	bundleValue := "BUNDLE"
	bundleCount := 0
	appendBundle := func(midValue string) {
		bundleValue += " " + midValue
		bundleCount++
	}

	for _, m := range mediaSections {
		if m.data && len(m.transceivers) != 0 {
			return nil, fmt.Errorf("invalid Media Section. Media + DataChannel both enabled")
		} else if !isPlanB && len(m.transceivers) > 1 {
			return nil, fmt.Errorf("invalid Media Section. Can not have multiple tracks in one MediaSection in UnifiedPlan")
		}

		shouldAddID := true
		if m.data {
			addDataMediaSection(d, mediaDtlsFingerprints, m.id, iceParams, candidates, connectionRole, iceGatheringState)
		} else {
			shouldAddID, err = addTransceiverSDP(d, isPlanB, mediaDtlsFingerprints, mediaEngine, m.id, iceParams, candidates, connectionRole, iceGatheringState, extMaps, m.transceivers...)
			if err != nil {
				return nil, err
			}
		}

		if shouldAddID {
			appendBundle(m.id)
		}
	}

	if !mediaDescriptionFingerprint {
		for _, fingerprint := range dtlsFingerprints {
			d.WithFingerprint(fingerprint.Algorithm, strings.ToUpper(fingerprint.Value))
		}
	}

	if isICELite {
		// RFC 5245 S15.3
		d = d.WithValueAttribute(sdp.AttrKeyICELite, sdp.AttrKeyICELite)
	}

	// Add global exts
	if maps, ok := extMaps[SDPSectionGlobal]; ok {
		for _, m := range maps {
			d.WithPropertyAttribute(m.Marshal())
		}
	}

	return d.WithValueAttribute(sdp.AttrKeyGroup, bundleValue), nil
}

func getMidValue(media *sdp.MediaDescription) string {
	for _, attr := range media.Attributes {
		if attr.Key == "mid" {
			return attr.Value
		}
	}
	return ""
}

func descriptionIsPlanB(desc *SessionDescription) bool {
	if desc == nil || desc.parsed == nil {
		return false
	}

	detectionRegex := regexp.MustCompile(`(?i)^(audio|video|data)$`)
	for _, media := range desc.parsed.MediaDescriptions {
		if len(detectionRegex.FindStringSubmatch(getMidValue(media))) == 2 {
			return true
		}
	}
	return false
}

func getPeerDirection(media *sdp.MediaDescription) RTPTransceiverDirection {
	for _, a := range media.Attributes {
		if direction := NewRTPTransceiverDirection(a.Key); direction != RTPTransceiverDirection(Unknown) {
			return direction
		}
	}
	return RTPTransceiverDirection(Unknown)
}

func extractFingerprint(desc *sdp.SessionDescription) (string, string, error) {
	fingerprints := []string{}

	if fingerprint, haveFingerprint := desc.Attribute("fingerprint"); haveFingerprint {
		fingerprints = append(fingerprints, fingerprint)
	}

	for _, m := range desc.MediaDescriptions {
		if fingerprint, haveFingerprint := m.Attribute("fingerprint"); haveFingerprint {
			fingerprints = append(fingerprints, fingerprint)
		}
	}

	if len(fingerprints) < 1 {
		return "", "", ErrSessionDescriptionNoFingerprint
	}

	for _, m := range fingerprints {
		if m != fingerprints[0] {
			return "", "", ErrSessionDescriptionConflictingFingerprints
		}
	}

	parts := strings.Split(fingerprints[0], " ")
	if len(parts) != 2 {
		return "", "", ErrSessionDescriptionInvalidFingerprint
	}
	return parts[1], parts[0], nil
}

func extractICEDetails(desc *sdp.SessionDescription) (string, string, []ICECandidate, error) {
	candidates := []ICECandidate{}
	remotePwds := []string{}
	remoteUfrags := []string{}

	if ufrag, haveUfrag := desc.Attribute("ice-ufrag"); haveUfrag {
		remoteUfrags = append(remoteUfrags, ufrag)
	}
	if pwd, havePwd := desc.Attribute("ice-pwd"); havePwd {
		remotePwds = append(remotePwds, pwd)
	}

	for _, m := range desc.MediaDescriptions {
		if ufrag, haveUfrag := m.Attribute("ice-ufrag"); haveUfrag {
			remoteUfrags = append(remoteUfrags, ufrag)
		}
		if pwd, havePwd := m.Attribute("ice-pwd"); havePwd {
			remotePwds = append(remotePwds, pwd)
		}

		for _, a := range m.Attributes {
			if a.IsICECandidate() {
				sdpCandidate, err := a.ToICECandidate()
				if err != nil {
					return "", "", nil, err
				}

				candidate, err := newICECandidateFromSDP(sdpCandidate)
				if err != nil {
					return "", "", nil, err
				}

				candidates = append(candidates, candidate)
			}
		}
	}

	if len(remoteUfrags) == 0 {
		return "", "", nil, ErrSessionDescriptionMissingIceUfrag
	} else if len(remotePwds) == 0 {
		return "", "", nil, ErrSessionDescriptionMissingIcePwd
	}

	for _, m := range remoteUfrags {
		if m != remoteUfrags[0] {
			return "", "", nil, ErrSessionDescriptionConflictingIceUfrag
		}
	}

	for _, m := range remotePwds {
		if m != remotePwds[0] {
			return "", "", nil, ErrSessionDescriptionConflictingIcePwd
		}
	}

	return remoteUfrags[0], remotePwds[0], candidates, nil
}

func haveApplicationMediaSection(desc *sdp.SessionDescription) bool {
	for _, m := range desc.MediaDescriptions {
		if m.MediaName.Media == mediaSectionApplication {
			return true
		}
	}

	return false
}

func matchedAnswerExt(descriptions *sdp.SessionDescription, localMaps map[SDPSectionType][]sdp.ExtMap) (map[SDPSectionType][]sdp.ExtMap, error) {
	remoteExtMaps, err := remoteExts(descriptions)
	if err != nil {
		return nil, err
	}
	return answerExtMaps(remoteExtMaps, localMaps), nil
}

func answerExtMaps(remoteExtMaps map[SDPSectionType]map[int]sdp.ExtMap, localMaps map[SDPSectionType][]sdp.ExtMap) map[SDPSectionType][]sdp.ExtMap {
	ret := map[SDPSectionType][]sdp.ExtMap{}
	for mediaType, remoteExtMap := range remoteExtMaps {
		if _, ok := ret[mediaType]; !ok {
			ret[mediaType] = []sdp.ExtMap{}
		}
		for _, extItem := range remoteExtMap {
			// add remote ext that match locally available ones
			for _, extMap := range localMaps[mediaType] {
				if extMap.URI.String() == extItem.URI.String() {
					ret[mediaType] = append(ret[mediaType], extItem)
				}
			}
		}
	}
	return ret
}

func remoteExts(session *sdp.SessionDescription) (map[SDPSectionType]map[int]sdp.ExtMap, error) {
	remoteExtMaps := map[SDPSectionType]map[int]sdp.ExtMap{}

	maybeAddExt := func(attr sdp.Attribute, mediaType SDPSectionType) error {
		if attr.Key != "extmap" {
			return nil
		}
		em := &sdp.ExtMap{}
		if err := em.Unmarshal("extmap:" + attr.Value); err != nil {
			return fmt.Errorf("failed to parse ExtMap: %v", err)
		}
		if remoteExtMap, ok := remoteExtMaps[mediaType][em.Value]; ok {
			if remoteExtMap.Value != em.Value {
				return fmt.Errorf("RemoteDescription changed some extmaps values")
			}
		} else {
			remoteExtMaps[mediaType][em.Value] = *em
		}
		return nil
	}

	// populate the extmaps from the current remote description
	for _, media := range session.MediaDescriptions {
		mediaType := SDPSectionType(media.MediaName.Media)
		// populate known remote extmap and handle conflicts.
		if _, ok := remoteExtMaps[mediaType]; !ok {
			remoteExtMaps[mediaType] = map[int]sdp.ExtMap{}
		}
		for _, attr := range media.Attributes {
			if err := maybeAddExt(attr, mediaType); err != nil {
				return nil, err
			}
		}
	}

	// Add global exts
	for _, attr := range session.Attributes {
		if err := maybeAddExt(attr, SDPSectionGlobal); err != nil {
			return nil, err
		}
	}
	return remoteExtMaps, nil
}
