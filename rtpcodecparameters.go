package webrtc

import "fmt"

// RTPCodecParameters provides information on codec settings.
type RTPCodecParameters struct {
	Name         string
	MimeType     string
	PayloadType  uint8
	ClockRate    uint32
	Maxptime     uint32
	Ptime        uint32
	Channels     uint32
	RTCPFeedback []RTCPFeedback
	Parameters   map[string]string
}

func (p RTPCodecParameters) String() string {
	return fmt.Sprintf("%d %s/%d/%d", p.PayloadType, p.Name, p.ClockRate, p.Channels)
}

func (p RTPCodecParameters) equalFMTP(other string) (bool, error) {
	b, err := sdpParseFmtpString(other)
	if err != nil {
		return false, fmt.Errorf("failed to parse FMTP: %s: %v", other, err)
	}

	return cmpMapStringString(p.Parameters, b), nil
}

func cmpMapStringString(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}

	for k, v := range a {
		vb, ok := b[k]
		if !ok || vb != v {
			return false
		}
	}

	return true
}
