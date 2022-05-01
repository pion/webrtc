package encryption

import (
	"github.com/pion/webrtc/v3/pkg/media"
	"os"
)

type Encryption struct {
	uiEncryptionStrategy  Strategy
	abrEncryptionStrategy Strategy
}

func NewEncryption() *Encryption {
	enc := &Encryption{}

	uiEncryptionStrategyStr := os.Getenv("HYPERSCALE_ENCRYPTION_UI_STRATEGY")
	abrEncryptionStrategyStr := os.Getenv("HYPERSCALE_ENCRYPTION_ABR_STRATEGY")

	enc.uiEncryptionStrategy = resolveEncryptionStrategy(uiEncryptionStrategyStr)
	enc.abrEncryptionStrategy = resolveEncryptionStrategy(abrEncryptionStrategyStr)

	return enc
}

func (encryption Encryption) ShouldEncrypt(sample media.Sample, packetSequence int, payloadDataIdx int) (bool, bool) {
	if sample.IsAbr {
		return encryption.abrEncryptionStrategy.ShouldEncrypt(sample, packetSequence, payloadDataIdx)
	} else {
		return encryption.uiEncryptionStrategy.ShouldEncrypt(sample, packetSequence, payloadDataIdx)
	}
}

func resolveEncryptionStrategy(strategy string) Strategy {
	switch strategy {
	case "FULL":
		return FullPacketEncryption{}
	case "NONE":
		return NoPacketEncryption{}
	case "FIRST_PACKET":
		return FrameFirstPacketEncryption{}
	default: // by default encrypt all
		return FullPacketEncryption{}
	}
}

type Strategy interface {
	ShouldEncrypt(sample media.Sample, packetSequence int, payloadDataIdx int) (bool, bool)
}

// FrameFirstPacketEncryption skip preceding metadata packets and encrypt the first 'data' packet of the frame
type FrameFirstPacketEncryption struct {
}

func (FrameFirstPacketEncryption) ShouldEncrypt(sample media.Sample, packetSequence int, payloadDataIdx int) (bool, bool) {
	if packetSequence == payloadDataIdx { // encrypt this packet but come back to ask about the next one
		return true, false
	} else if packetSequence > payloadDataIdx { // don't encrypt and subsequent calls will return the same result
		return false, true
	} else { // packets before data packet, don't encrypt, keep checking further packets
		return false, false
	}
}

// FullPacketEncryption encrypt every packet in the frame
type FullPacketEncryption struct {
}

func (FullPacketEncryption) ShouldEncrypt(sample media.Sample, packetSequence int, payloadDataIdx int) (bool, bool) {
	// encrypt regardless of packet sequence, subsequent calls will return the same result
	return true, true
}

// NoPacketEncryption don't encrypt any packet in the frame
type NoPacketEncryption struct {
}

func (NoPacketEncryption) ShouldEncrypt(sample media.Sample, packetSequence int, payloadDataIdx int) (bool, bool) {
	// don't encrypt regardless of packet sequence, subsequent calls will return the same result
	return false, true
}
