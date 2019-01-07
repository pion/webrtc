package srtp

import (
	"crypto/cipher"
	"encoding/binary"

	"github.com/pions/webrtc/pkg/rtp"
	"github.com/pkg/errors"
)

func (c *Context) decryptRTP(dst, encrypted []byte, header *rtp.Header) ([]byte, error) {
	dst = allocateIsMismatch(dst, encrypted)

	s := c.getSSRCState(header.SSRC)
	c.updateRolloverCount(header.SequenceNumber, s)

	pktWithROC := append(append([]byte{}, dst[:len(dst)-authTagSize]...), make([]byte, 4)...)
	binary.BigEndian.PutUint32(pktWithROC[len(pktWithROC)-4:], s.rolloverCounter)

	actualAuthTag := dst[len(dst)-authTagSize:]
	verified, err := c.verifyAuthTag(pktWithROC, actualAuthTag)
	if err != nil {
		return nil, err
	} else if !verified {
		return nil, errors.Errorf("Failed to verify auth tag")
	}

	stream := cipher.NewCTR(c.srtpBlock, c.generateCounter(header.SequenceNumber, s.rolloverCounter, s.ssrc, c.srtpSessionSalt))
	stream.XORKeyStream(dst[header.PayloadOffset:], dst[header.PayloadOffset:])

	return dst[:len(dst)-authTagSize], nil
}

// DecryptRTP decrypts a RTP packet with an encrypted payload
func (c *Context) DecryptRTP(dst, encrypted []byte, header *rtp.Header) ([]byte, error) {
	if header == nil {
		header = &rtp.Header{}
	}

	if err := header.Unmarshal(encrypted); err != nil {
		return nil, err
	}

	return c.decryptRTP(dst, encrypted, header)
}

func (c *Context) encryptRTP(dst, decrypted []byte, header *rtp.Header) ([]byte, error) {
	dst = allocateIsMismatch(dst, decrypted)

	s := c.getSSRCState(header.SSRC)

	c.updateRolloverCount(header.SequenceNumber, s)

	stream := cipher.NewCTR(c.srtpBlock, c.generateCounter(header.SequenceNumber, s.rolloverCounter, s.ssrc, c.srtpSessionSalt))
	stream.XORKeyStream(dst[header.PayloadOffset:], dst[header.PayloadOffset:])

	dst = append(dst, make([]byte, 4)...)
	binary.BigEndian.PutUint32(dst[len(dst)-4:], s.rolloverCounter)

	authTag, err := c.generateAuthTag(dst, c.srtpSessionAuthTag)
	if err != nil {
		return nil, err
	}

	return append(dst[:len(dst)-4], authTag...), nil
}

// EncryptRTP Encrypts a RTP packet
func (c *Context) EncryptRTP(dst, decrypted []byte, header *rtp.Header) ([]byte, error) {
	if header == nil {
		header = &rtp.Header{}
	}

	if err := header.Unmarshal(decrypted); err != nil {
		return nil, err
	}

	return c.encryptRTP(dst, decrypted, header)
}
