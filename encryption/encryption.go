package encryption

import (
	"github.com/pion/webrtc/v3/pkg/media"
	"os"

)

type EncryptionFactory struct {

}

type Encryption struct {
	uiEncryptionStrategy EncryptionStrategy
	abrEncryptionStrategy EncryptionStrategy
}

func NewEncryption() *Encryption {
	enc := &Encryption{}

	uiEncryptionStrategyStr := os.Getenv("HYPERSCALE_ENCRYPTION_UI_STRATEGY")
	abrEncryptionStrategyStr := os.Getenv("HYPERSCALE_ENCRYPTION_ABR_STRATEGY")

	enc.uiEncryptionStrategy = resolveEncryptionStrategy(uiEncryptionStrategyStr)
	enc.abrEncryptionStrategy = resolveEncryptionStrategy(abrEncryptionStrategyStr)

	return enc
}

func (encryption Encryption) ShouldEncrypt(sample media.Sample, packetSequence int, payloadDataIdx int) bool {
	if sample.IsAbr {
		return encryption.abrEncryptionStrategy.ShouldEncrypt(sample, packetSequence, payloadDataIdx)
	} else {
		return encryption.uiEncryptionStrategy.ShouldEncrypt(sample, packetSequence, payloadDataIdx)
	}
}

func resolveEncryptionStrategy(strategy string) EncryptionStrategy {
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

type EncryptionStrategy interface {
	ShouldEncrypt(sample media.Sample, packetSequence int, payloadDataIdx int) bool
}

type FrameFirstPacketEncryption struct {

}

func (FrameFirstPacketEncryption) ShouldEncrypt(sample media.Sample, packetSequence int, payloadDataIdx int) bool {
	// we need to encrypt the first 'data' packet of the frame and skip any metadata packets
	if packetSequence == payloadDataIdx {
		return true
	}

	return false
}

type FullPacketEncryption struct {

}

func (FullPacketEncryption) ShouldEncrypt(sample media.Sample, packetSequence int, payloadDataIdx int) bool {
	return true
}

type NoPacketEncryption struct {

}

func (NoPacketEncryption) ShouldEncrypt(sample media.Sample, packetSequence int, payloadDataIdx int) bool {
	return false
}