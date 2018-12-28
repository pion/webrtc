package webrtc

// RTCQuicParameters holds information relating to QUIC configuration.
type RTCQuicParameters struct {
	Role         RTCQuicRole          `json:"role"`
	Fingerprints []RTCDtlsFingerprint `json:"fingerprints"`
}
