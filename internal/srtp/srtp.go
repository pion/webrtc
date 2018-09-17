package srtp

import (
	"crypto/cipher"
	"encoding/binary"

	"github.com/pions/webrtc/pkg/rtp"
)

// DecryptRTP decrypts a RTP packet with an encrypted payload
func (c *Context) DecryptRTP(packet *rtp.Packet) bool {
	s := c.getSSRCState(packet.SSRC)

	c.updateRolloverCount(packet.SequenceNumber, s)

	stream := cipher.NewCTR(c.srtpBlock, c.generateCounter(packet.SequenceNumber, s.rolloverCounter, s.ssrc, c.srtpSessionSalt))
	stream.XORKeyStream(packet.Payload, packet.Payload)

	// TODO remove tags, need to assert value
	packet.Payload = packet.Payload[:len(packet.Payload)-10]

	// Replace payload with decrypted
	packet.Raw = packet.Raw[0:packet.PayloadOffset]
	packet.Raw = append(packet.Raw, packet.Payload...)

	return true
}

// EncryptRTP Encrypts a SRTP packet in place
func (c *Context) EncryptRTP(packet *rtp.Packet) bool {
	s := c.getSSRCState(packet.SSRC)

	c.updateRolloverCount(packet.SequenceNumber, s)

	stream := cipher.NewCTR(c.srtpBlock, c.generateCounter(packet.SequenceNumber, s.rolloverCounter, s.ssrc, c.srtpSessionSalt))
	stream.XORKeyStream(packet.Payload, packet.Payload)

	fullPkt, err := packet.Marshal()
	if err != nil {
		return false
	}

	fullPkt = append(fullPkt, make([]byte, 4)...)
	binary.BigEndian.PutUint32(fullPkt[len(fullPkt)-4:], s.rolloverCounter)

	authTag, err := c.generateAuthTag(fullPkt, c.srtpSessionAuthTag)
	if err != nil {
		return false
	}

	packet.Payload = append(packet.Payload, authTag...)
	return true
}

// https://tools.ietf.org/html/rfc3550#appendix-A.1
func (c *Context) updateRolloverCount(sequenceNumber uint16, s *ssrcState) {
	if !s.rolloverHasProcessed {
		s.rolloverHasProcessed = true
	} else if sequenceNumber == 0 { // We exactly hit the rollover count

		// Only update rolloverCounter if lastSequenceNumber is greater then maxROCDisorder
		// otherwise we already incremented for disorder
		if s.lastSequenceNumber > maxROCDisorder {
			s.rolloverCounter++
		}
	} else if s.lastSequenceNumber < maxROCDisorder && sequenceNumber > (maxSequenceNumber-maxROCDisorder) {
		// Our last sequence number incremented because we crossed 0, but then our current number was within maxROCDisorder of the max
		// So we fell behind, drop to account for jitter
		s.rolloverCounter--
	} else if sequenceNumber < maxROCDisorder && s.lastSequenceNumber > (maxSequenceNumber-maxROCDisorder) {
		// our current is within a maxROCDisorder of 0
		// and our last sequence number was a high sequence number, increment to account for jitter
		s.rolloverCounter++
	}
	s.lastSequenceNumber = sequenceNumber
}

func (c *Context) getSSRCState(ssrc uint32) *ssrcState {
	s, ok := c.ssrcStates[ssrc]
	if ok {
		return s
	}

	s = &ssrcState{ssrc: ssrc}
	c.ssrcStates[ssrc] = s
	return s
}
