package codecs

import "github.com/pions/webrtc/pkg/rtp"

// OpusPayloader payloads Opus packets
type OpusPayloader struct{}

// Payload fragments an Opus packet across one or more byte arrays
func (p *OpusPayloader) Payload(mtu int, payload []byte) [][]byte {
	return [][]byte{payload}
}

// OpusPacket represents the VP8 header that is stored in the payload of an RTP Packet
type OpusPacket struct {
	Payload []byte
}

// Unmarshal parses the passed byte slice and stores the result in the OpusPacket this method is called upon
func (p *OpusPacket) Unmarshal(packet *rtp.Packet) error {
	p.Payload = packet.Payload
	return nil
}
