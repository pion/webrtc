package srtp

import (
	"crypto/cipher"
	"encoding/binary"

	"github.com/pions/webrtc/pkg/rtcp"
)

func (c *Context) decryptRTCP(dst, encrypted []byte, header *rtcp.Header) ([]byte, error) {
	out := allocateIfMismatch(dst, encrypted)

	tailOffset := len(encrypted) - (authTagSize + srtcpIndexSize)
	out = out[0:tailOffset]

	isEncrypted := encrypted[tailOffset] >> 7
	if isEncrypted == 0 {
		return out, nil
	}

	srtcpIndexBuffer := out[tailOffset : tailOffset+srtcpIndexSize]
	srtcpIndexBuffer[0] &= 0x7f // unset Encryption bit

	index := binary.BigEndian.Uint32(srtcpIndexBuffer)
	ssrc := binary.BigEndian.Uint32(encrypted[4:])

	stream := cipher.NewCTR(c.srtcpBlock, c.generateCounter(uint16(index&0xffff), index>>16, ssrc, c.srtcpSessionSalt))
	stream.XORKeyStream(out[8:], out[8:])

	return out, nil
}

// DecryptRTCP decrypts a buffer that contains a RTCP packet
func (c *Context) DecryptRTCP(dst, encrypted []byte, header *rtcp.Header) ([]byte, error) {
	if header == nil {
		header = &rtcp.Header{}
	}

	if err := header.Unmarshal(encrypted); err != nil {
		return nil, err
	}

	return c.decryptRTCP(dst, encrypted, header)
}

func (c *Context) encryptRTCP(dst, decrypted []byte, header *rtcp.Header) ([]byte, error) {
	out := allocateIfMismatch(dst, decrypted)
	ssrc := binary.BigEndian.Uint32(out[4:])

	// We roll over early because MSB is used for marking as encrypted
	c.srtcpIndex++
	if c.srtcpIndex >= 2147483647 {
		c.srtcpIndex = 0
	}

	// Encrypt everything after header
	stream := cipher.NewCTR(c.srtcpBlock, c.generateCounter(uint16(c.srtcpIndex&0xffff), c.srtcpIndex>>16, ssrc, c.srtcpSessionSalt))
	stream.XORKeyStream(out[8:], out[8:])

	// Add SRTCP Index and set Encryption bit
	out = append(out, make([]byte, 4)...)
	binary.BigEndian.PutUint32(out[len(out)-4:], c.srtcpIndex)
	out[len(out)-4] |= 0x80

	authTag, err := c.generateAuthTag(out, c.srtcpSessionAuthTag)
	if err != nil {
		return nil, err
	}
	return append(out, authTag...), nil
}

// EncryptRTCP Encrypts a RTCP packet
func (c *Context) EncryptRTCP(dst, decrypted []byte, header *rtcp.Header) ([]byte, error) {
	if header == nil {
		header = &rtcp.Header{}
	}

	if err := header.Unmarshal(decrypted); err != nil {
		return nil, err
	}

	return c.encryptRTCP(dst, decrypted, header)
}
