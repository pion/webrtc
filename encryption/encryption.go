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
	// we want to encrypt the first packet of the IDR data, not the metadata (sps/pps)
	// following h264 payloader scheme, when an iframe includes sps/ssp they get packetized
	// using STAP-A in a single packet and are the first packet of the frame.
	// in this case, the next packet, seq == 1 will be the first packet conataining the IDR data.
	// when the frame doesn't include sps/pps the first IDR data packet is expected to be
	// the first packet of the frame, seq == 0
	if sample.IsSpsPps && sample.IsIFrame && packetSequence == 1 {
		return true
	} else if !sample.IsSpsPps && sample.IsIFrame && packetSequence == 0 {
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