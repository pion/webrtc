package srtp

import (
	"crypto/cipher"
	"encoding/binary"

	"github.com/pions/webrtc/pkg/rtp"
	"github.com/pkg/errors"
)

// DecryptRTP decrypts a RTP packet with an encrypted payload
func (c *Context) DecryptRTP(encrypted []byte) ([]byte, error) {
	p := &rtp.Packet{}
	if err := p.Unmarshal(append([]byte{}, encrypted...)); err != nil {
		return nil, err
	}

	s := c.getSSRCState(p.SSRC)

	c.updateRolloverCount(p.SequenceNumber, s)

	pktWithROC := append([]byte{}, p.Raw[:len(p.Raw)-authTagSize]...)
	pktWithROC = append(pktWithROC, make([]byte, 4)...)
	binary.BigEndian.PutUint32(pktWithROC[len(pktWithROC)-4:], s.rolloverCounter)

	actualAuthTag := p.Payload[len(p.Payload)-authTagSize:]
	verified, err := c.verifyAuthTag(pktWithROC, actualAuthTag)
	if err != nil {
		return nil, err
	} else if !verified {
		return nil, errors.Errorf("Failed to verify auth tag")
	}

	p.Payload = p.Payload[:len(p.Payload)-authTagSize]

	stream := cipher.NewCTR(c.srtpBlock, c.generateCounter(p.SequenceNumber, s.rolloverCounter, s.ssrc, c.srtpSessionSalt))
	stream.XORKeyStream(p.Payload, p.Payload)

	return append(p.Raw[0:p.PayloadOffset], p.Payload...), nil
}

// EncryptRTP Encrypts a SRTP packet in place
func (c *Context) EncryptRTP(encrypted []byte) ([]byte, error) {
	p := &rtp.Packet{}
	if err := p.Unmarshal(append([]byte{}, encrypted...)); err != nil {
		return nil, err
	}

	s := c.getSSRCState(p.SSRC)

	c.updateRolloverCount(p.SequenceNumber, s)

	stream := cipher.NewCTR(c.srtpBlock, c.generateCounter(p.SequenceNumber, s.rolloverCounter, s.ssrc, c.srtpSessionSalt))
	stream.XORKeyStream(p.Payload, p.Payload)

	fullPkt, err := p.Marshal()
	if err != nil {
		return nil, err
	}

	fullPkt = append(fullPkt, make([]byte, 4)...)
	binary.BigEndian.PutUint32(fullPkt[len(fullPkt)-4:], s.rolloverCounter)

	authTag, err := c.generateAuthTag(fullPkt, c.srtpSessionAuthTag)
	if err != nil {
		return nil, err
	}

	return append(p.Raw[0:p.PayloadOffset], append(p.Payload, authTag...)...), nil
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
