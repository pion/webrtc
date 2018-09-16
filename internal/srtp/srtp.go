package srtp

import (
	"crypto/cipher"

	"github.com/pions/webrtc/pkg/rtp"
)

// DecryptPacket decrypts a RTP packet with an encrypted payload
func (c *Context) DecryptPacket(packet *rtp.Packet) bool {
	s := c.getSSRCState(packet.SSRC)

	c.updateRolloverCount(packet.SequenceNumber, s)

	stream := cipher.NewCTR(c.block, c.generateCounter(packet.SequenceNumber, s))
	stream.XORKeyStream(packet.Payload, packet.Payload)

	// TODO remove tags, need to assert value
	packet.Payload = packet.Payload[:len(packet.Payload)-10]

	// Replace payload with decrypted
	packet.Raw = packet.Raw[0:packet.PayloadOffset]
	packet.Raw = append(packet.Raw, packet.Payload...)

	return true
}

// EncryptPacket Encrypts a SRTP packet in place
func (c *Context) EncryptPacket(packet *rtp.Packet) bool {
	s := c.getSSRCState(packet.SSRC)

	c.updateRolloverCount(packet.SequenceNumber, s)

	stream := cipher.NewCTR(c.block, c.generateCounter(packet.SequenceNumber, s))
	stream.XORKeyStream(packet.Payload, packet.Payload)

	if err := c.addAuthTag(packet, s); err != nil {
		return false
	}

	return true
}
