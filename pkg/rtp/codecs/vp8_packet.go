package codecs

import "github.com/pions/webrtc/pkg/rtp"

// VP8Packet represents the VP8 header that is stored in the payload of an RTP Packet
type VP8Packet struct {
	// Required Header
	X   uint8 /* extended controlbits present */
	N   uint8 /* (non-reference frame)  when set to 1 this frame can be discarded */
	S   uint8 /* start of VP8 partition */
	PID uint8 /* partition index */

	// Optional Header
	I         uint8  /* 1 if PictureID is present */
	L         uint8  /* 1 if TL0PICIDX is present */
	T         uint8  /* 1 if TID is present */
	K         uint8  /* 1 if KEYIDX is present */
	PictureID uint16 /* 8 or 16 bits, picture ID */
	TL0PICIDX uint8  /* 8 bits temporal level zero index */

	Payload []byte
}

// Unmarshal parses the passed byte slice and stores the result in the VP8Packet this method is called upon
func (p *VP8Packet) Unmarshal(packet *rtp.Packet) error {
	payload := packet.Payload

	payloadIndex := 0

	p.X = (payload[payloadIndex] & 0x80) >> 7
	p.N = (payload[payloadIndex] & 0x20) >> 5
	p.S = (payload[payloadIndex] & 0x10) >> 4
	p.PID = payload[payloadIndex] & 0x07

	payloadIndex++

	if p.X == 1 {
		p.I = (payload[payloadIndex] & 0x80) >> 7
		p.L = (payload[payloadIndex] & 0x40) >> 6
		p.T = (payload[payloadIndex] & 0x20) >> 5
		p.K = (payload[payloadIndex] & 0x10) >> 4
		payloadIndex++
	}

	if p.I == 1 { // PID present?
		if payload[payloadIndex]&0x80 > 0 { // M == 1, PID is 16bit
			payloadIndex += 2
		} else {
			payloadIndex++
		}
	}

	if p.L == 1 {
		payloadIndex++
	}

	if p.T == 1 || p.K == 1 {
		payloadIndex++
	}

	p.Payload = payload[payloadIndex:]

	return nil
}
