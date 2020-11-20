package webrtc

import (
	"github.com/pion/webrtc/v3/pkg/interceptor/movetopionrtp"
)

func convertRTPParameters(in RTPParameters) movetopionrtp.RTPParameters {
	return movetopionrtp.RTPParameters{
		HeaderExtensions: convertHeaderExtensions(in.HeaderExtensions),
		Codecs:           convertRTPCodecParameters(in.Codecs),
	}
}

func convertHeaderExtensions(in []RTPHeaderExtensionParameter) []movetopionrtp.RTPHeaderExtensionParameter {
	result := make([]movetopionrtp.RTPHeaderExtensionParameter, 0, len(in))
	for _, v := range in {
		result = append(result, convertHeaderExtension(v))
	}

	return result
}

func convertHeaderExtension(in RTPHeaderExtensionParameter) movetopionrtp.RTPHeaderExtensionParameter {
	return movetopionrtp.RTPHeaderExtensionParameter{
		URI: in.URI,
		ID:  in.ID,
	}
}

func convertRTPCodecParameters(in []RTPCodecParameters) []movetopionrtp.RTPCodecParameters {
	result := make([]movetopionrtp.RTPCodecParameters, 0, len(in))
	for _, v := range in {
		result = append(result, convertRTPCodecParameter(v))
	}

	return result
}

func convertRTPCodecParameter(in RTPCodecParameters) movetopionrtp.RTPCodecParameters {
	return movetopionrtp.RTPCodecParameters{
		RTPCodecCapability: movetopionrtp.RTPCodecCapability{
			MimeType:     in.MimeType,
			ClockRate:    in.ClockRate,
			Channels:     in.Channels,
			SDPFmtpLine:  in.SDPFmtpLine,
			RTCPFeedback: convertRTCPFeedbacks(in.RTCPFeedback),
		},
		PayloadType: convertPayloadType(in.PayloadType),
	}
}

func convertRTCPFeedbacks(in []RTCPFeedback) []movetopionrtp.RTCPFeedback {
	result := make([]movetopionrtp.RTCPFeedback, 0, len(in))
	for _, v := range in {
		result = append(result, convertRTCPFeedback(v))
	}

	return result
}

func convertRTCPFeedback(in RTCPFeedback) movetopionrtp.RTCPFeedback {
	return movetopionrtp.RTCPFeedback{
		Type:      in.Type,
		Parameter: in.Parameter,
	}
}

func convertPayloadType(in PayloadType) movetopionrtp.PayloadType {
	return movetopionrtp.PayloadType(in)
}

func convertSSRC(in SSRC) movetopionrtp.SSRC {
	return movetopionrtp.SSRC(in)
}
