package webrtc

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/pions/sdp/v2"
)

// Port of the SDPUtils logic used in
// https://github.com/webrtcHacks/adapter

// sdpParseRTPDecodingParameters parses the SDP media section and returns
// an array of RTPDecodingParameters.
func sdpParseRTPDecodingParameters(d *sdp.MediaDescription) ([]RTPDecodingParameters, error) {
	res := make([]RTPDecodingParameters, 0)

	ssrcs := make([]sdpSSRCMedia, 0)
	err := sdpMatchAttributePrefixFunc(d, sdp.AttrKeySSRC, "", func(a sdp.Attribute) error {
		ssrc, pErr := sdpParseSSRCMedia(a)
		if pErr != nil {
			return pErr
		}
		// filter a=ssrc:... cname:, ignore PlanB-msid
		if ssrc.Attribute == "cname" {
			ssrcs = append(ssrcs, ssrc)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to parse ssrcs: %v", err)
	}

	if len(ssrcs) == 0 {
		return nil, errors.New("no ssrc found in media description")
	}

	primarySSRC := ssrcs[0].SSRC

	res = append(res, RTPDecodingParameters{
		RTPCodingParameters{
			SSRC: primarySSRC,
		},
	})

	return res, nil
}

// Parses the SDP media section and returns RTPParameters.
func sdpParseRTPParameters(d *sdp.MediaDescription) (RTPParameters, error) {
	res := RTPParameters{}

	// Parse codecs
	for _, format := range d.MediaName.Formats {
		err := sdpMatchAttributePrefixFunc(d, "rtpmap", format+" ", sdpRTPCodecParser(d, &res))
		if err != nil {
			return RTPParameters{}, fmt.Errorf("failed to parse codec: %v", err)
		}
	}

	// Parse header extensions
	err := sdpMatchAttributePrefixFunc(d, "extmap", "", sdpExtmapParser(d, &res))
	if err != nil {
		return RTPParameters{}, fmt.Errorf("failed to parse header extensions: %v", err)
	}

	return res, nil
}

func sdpRTPCodecParser(d *sdp.MediaDescription, p *RTPParameters) sdpAttributeParser {
	return func(a sdp.Attribute) error {
		codec, err := sdpParseRtpMap(a)
		if err != nil {
			return err
		}

		prefix := fmt.Sprintf("%d ", codec.PayloadType)
		err = sdpMatchAttributePrefixFunc(d, "fmtp", prefix, sdpRTPFmtpParser(d, &codec))
		if err != nil {
			return fmt.Errorf("failed to parse fmtp: %v", err)
		}

		prefix = fmt.Sprintf("%d ", codec.PayloadType)
		err = sdpMatchAttributePrefixFunc(d, "rtcp-fb", prefix, sdpRtpRtcpFeedbackParser(d, &codec))
		if err != nil {
			return fmt.Errorf("failed to parse rtcp feedback: %v", err)
		}

		p.Codecs = append(p.Codecs, codec)

		return nil
	}
}

func sdpRTPFmtpParser(d *sdp.MediaDescription, p *RTPCodecParameters) sdpAttributeParser {
	return func(a sdp.Attribute) error {
		params, err := sdpParseFmtp(a)
		if err != nil {
			return err
		}
		p.Parameters = params

		return nil
	}
}

// Parses an ftmp line, returns dictionary. Sample input:
// a=fmtp:96 vbr=on;cng=on
// Also deals with vbr=on; cng=on
func sdpParseFmtp(a sdp.Attribute) (map[string]string, error) {
	sp := strings.Index(a.Value, " ")
	if sp < 1 {
		return nil, fmt.Errorf("attribute to short: %s", a.Value)
	}

	paramsStr := a.Value[sp+1:]
	return sdpParseFmtpString(paramsStr)
}

// Parses an ftmp value, returns dictionary. Sample input:
// vbr=on;cng=on
func sdpParseFmtpString(paramsStr string) (map[string]string, error) {
	res := make(map[string]string)

	params := strings.Split(paramsStr, ";")
	for _, param := range params {
		if len(strings.TrimSpace(param)) == 0 {
			continue
		}
		parts := strings.Split(param, "=")
		if len(parts) < 2 {
			return nil, fmt.Errorf("invalid parameter: %s", param)
		}
		k := strings.TrimSpace(parts[0])
		v := strings.TrimSpace(parts[1])
		res[k] = v
	}

	return res, nil
}

func sdpRtpRtcpFeedbackParser(d *sdp.MediaDescription, p *RTPCodecParameters) sdpAttributeParser {
	return func(a sdp.Attribute) error {
		fb, err := sdpParseRtcpFeedback(a)
		if err != nil {
			return err
		}
		p.RTCPFeedback = append(p.RTCPFeedback, fb)

		return nil
	}
}

// Parses an rtcp-fb line, returns RTCPRtcpFeedback object. Sample input:
// a=rtcp-fb:98 nack rpsi
func sdpParseRtcpFeedback(a sdp.Attribute) (RTCPFeedback, error) {
	sp := strings.Index(a.Value, " ")
	if sp < 1 {
		return RTCPFeedback{}, fmt.Errorf("rtcp-fb attribute to short: %s", a.Value)
	}
	fbStr := a.Value[sp+1:]
	sp = strings.Index(fbStr, " ")

	typ := fbStr
	param := ""
	if sp > 0 {
		typ = fbStr[:sp]
		param = fbStr[sp+1:]
	}

	return RTCPFeedback{
		Type:      typ,
		Parameter: param,
	}, nil
}

func sdpExtmapParser(d *sdp.MediaDescription, p *RTPParameters) sdpAttributeParser {
	return func(a sdp.Attribute) error {
		ext, err := sdpParseExtmap(a)
		if err != nil {
			return err
		}
		p.HeaderExtensions = append(p.HeaderExtensions, ext)

		return nil
	}
}

// Parses an a=extmap line (headerextension from RFC 5285). Sample input:
// a=extmap:2 urn:ietf:params:rtp-hdrext:toffset
// a=extmap:2/sendonly urn:ietf:params:rtp-hdrext:toffset
func sdpParseExtmap(a sdp.Attribute) (RTPHeaderExtensionParameters, error) {
	sp := strings.Index(a.Value, " ")
	if sp < 1 {
		return RTPHeaderExtensionParameters{}, fmt.Errorf("extmap attribute to short: %s", a.Value)
	}

	idParts := strings.Split(a.Value[:sp], "/")

	idStr := idParts[0]
	id, err := strconv.ParseUint(idStr, 10, 16)
	if err != nil {
		return RTPHeaderExtensionParameters{}, fmt.Errorf("invalid id: %s", idStr)
	}

	direction := "sendrecv"
	if len(idParts) > 1 {
		direction = idParts[1]
	}

	uri := a.Value[sp+1:]

	return RTPHeaderExtensionParameters{
		ID:        uint16(id),
		direction: direction,
		URI:       uri,
	}, nil
}

// Parses an rtpmap line, returns RTPCoddecParameters. Sample input:
// a=rtpmap:109 opus/48000/2
func sdpParseRtpMap(a sdp.Attribute) (RTPCodecParameters, error) {
	sp := strings.Index(a.Value, " ")
	if sp < 1 {
		return RTPCodecParameters{}, fmt.Errorf("rtpmap attribute to short: %s", a.Value)
	}
	payloadTypeStr := a.Value[:sp]
	payloadTypeRaw, err := strconv.ParseUint(payloadTypeStr, 10, 8)
	if err != nil {
		return RTPCodecParameters{}, fmt.Errorf("invalid payload type: %s", payloadTypeStr)
	}

	codecStr := a.Value[sp+1:]
	parts := strings.Split(codecStr, "/")
	if len(parts) < 2 {
		return RTPCodecParameters{}, fmt.Errorf("invalid codec: %s", codecStr)
	}
	name := parts[0]

	clockrateStr := parts[1]
	clockrate, err := strconv.ParseUint(clockrateStr, 10, 32)
	if err != nil {
		return RTPCodecParameters{}, fmt.Errorf("invalid clockrate: %s", clockrateStr)
	}

	channels := uint64(0)
	if len(parts) == 3 {
		channelsStr := parts[2]
		channels, err = strconv.ParseUint(channelsStr, 10, 32)
		if err != nil {
			return RTPCodecParameters{}, fmt.Errorf("invalid channels: %s", channelsStr)
		}
	}

	return RTPCodecParameters{
		Name:        name,
		PayloadType: uint8(payloadTypeRaw),
		ClockRate:   uint32(clockrate),
		Channels:    uint32(channels),
	}, nil
}

// Generic: could be moved to package SDP

// sdpSSRCMedia represents a an RFC 5576 ssrc media attribute.
type sdpSSRCMedia struct {
	SSRC      uint32
	Attribute string
	Value     string
}

// Parses an RFC 5576 ssrc media attribute. Sample input:
// a=ssrc:<ssrc-id> <attribute>
// a=ssrc:<ssrc-id> <attribute>:<value>
func sdpParseSSRCMedia(a sdp.Attribute) (sdpSSRCMedia, error) {
	sp := strings.Index(a.Value, " ")
	if sp < 1 {
		return sdpSSRCMedia{}, fmt.Errorf("ssrc media attribute to short: %s", a.Value)
	}
	ssrcStr := a.Value[:sp]
	ssrc, err := strconv.ParseUint(ssrcStr, 10, 32)
	if err != nil {
		return sdpSSRCMedia{}, fmt.Errorf("failed to parse ssrc: %s", ssrcStr)
	}
	parts := strings.Split(a.Value[sp+1:], ":")
	attribute := parts[0]
	value := ""
	if len(parts) > 1 {
		value = parts[1]
	}

	return sdpSSRCMedia{
		SSRC:      uint32(ssrc),
		Attribute: attribute,
		Value:     value,
	}, nil
}

type sdpAttributeParser func(sdp.Attribute) error

func sdpMatchAttributePrefixFunc(d *sdp.MediaDescription, key, prefix string, p sdpAttributeParser) error {
	for _, a := range d.Attributes {
		if a.Key != key || !strings.HasPrefix(a.Value, prefix) {
			continue
		}

		err := p(a)
		if err != nil {
			return err
		}
	}

	return nil
}

func sdpFindAttributePrefix(d *sdp.MediaDescription, key, prefix string) (sdp.Attribute, error) {
	for _, a := range d.Attributes {
		if a.Key != key || !strings.HasPrefix(a.Value, prefix) {
			continue
		}

		return a, nil
	}

	return sdp.Attribute{}, errors.New("attribute not found")
}
