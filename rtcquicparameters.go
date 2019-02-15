package webrtc

// QUICParameters holds information relating to QUIC configuration.
type QUICParameters struct {
	Role         QUICRole          `json:"role"`
	Fingerprints []DTLSFingerprint `json:"fingerprints"`
}
