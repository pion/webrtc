package webrtc

// RTPCapabilities represents the capabilities of a transceiver
type RTPCapabilities struct {
	Codecs           []RTPCodecCapability
	HeaderExtensions []RTPHeaderExtensionCapability
}
