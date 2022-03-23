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

func (encryption Encryption) ShouldEncrypt(sample media.Sample, packetSequence int) bool {
	if sample.IsAbr {
		return encryption.abrEncryptionStrategy.ShouldEncrypt(sample, packetSequence)
	} else {
		return encryption.uiEncryptionStrategy.ShouldEncrypt(sample, packetSequence)
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
	ShouldEncrypt(sample media.Sample, packetSequence int) bool
}

type FrameFirstPacketEncryption struct {

}

func (FrameFirstPacketEncryption) ShouldEncrypt(sample media.Sample, packetSequence int) bool {
	if packetSequence == 0 {
		return true
	}
	return false
}

type FullPacketEncryption struct {

}

func (FullPacketEncryption) ShouldEncrypt(sample media.Sample, packetSequence int) bool {
	return true
}

type NoPacketEncryption struct {

}

func (NoPacketEncryption) ShouldEncrypt(sample media.Sample, packetSequence int) bool {
	return false
}