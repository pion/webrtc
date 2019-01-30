package webrtc

import "fmt"

// RTPParameters contains the RTP stack settings used by both senders and receivers.
type RTPParameters struct {
	Codecs           []RTPCodecParameters
	HeaderExtensions []RTPHeaderExtensionParameters
	RTCP             RTCPParameters
}

func (p RTPReceiveParameters) getCodecParameters(payloadType uint8) (RTPCodecParameters, error) {
	for _, codec := range p.Codecs {
		if codec.PayloadType == payloadType {
			return codec, nil
		}
	}

	return RTPCodecParameters{}, fmt.Errorf("payload type %d not found", payloadType)
}
