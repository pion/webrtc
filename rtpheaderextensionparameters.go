package webrtc

// RTPHeaderExtensionParameters dictionary enables a header extension
// to be configured for use within an RTPSender or RTPReceiver.
type RTPHeaderExtensionParameters struct {
	ID        uint16
	direction string
	URI       string
}
