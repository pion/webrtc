package srtp

import (
	"crypto/cipher"
	"encoding/binary"

	"github.com/pkg/errors"
)

// DecryptRTCP decrypts a buffer that contains a RTCP packet
// We can't pass *rtcp.Packet as the encrypt will obscure significant fields
func (c *Context) DecryptRTCP(encrypted []byte) ([]byte, error) {
	rtcpLen := int((binary.BigEndian.Uint16(encrypted[2:]) + 1) * 8)
	if rtcpLen+srtcpIndexSize+authTagSize > len(encrypted) {
		return nil, errors.Errorf("SRCTP packet invalid size: header_len %d, buffer_size %d. ", rtcpLen, len(encrypted))
	}

	out := append([]byte{}, encrypted[0:rtcpLen]...)
	isEncrypted := encrypted[rtcpLen] >> 7
	if isEncrypted == 0 {
		return out, nil
	}

	srtcpIndexBuffer := append([]byte{}, encrypted[rtcpLen:rtcpLen+srtcpIndexSize]...)
	srtcpIndexBuffer[0] &= 0x7f //unset Encryption bit

	index := binary.BigEndian.Uint32(srtcpIndexBuffer)
	ssrc := binary.BigEndian.Uint32(encrypted[4:])

	stream := cipher.NewCTR(c.srtcpBlock, c.generateCounter(uint16(index&0xffff), index>>16, ssrc, c.srtcpSessionSalt))
	stream.XORKeyStream(out[8:], out[8:])

	return out, nil
}
