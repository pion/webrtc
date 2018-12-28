package webrtc

// RTCDtlsParameters holds information relating to DTLS configuration.
type RTCDtlsParameters struct {
	Role         RTCDtlsRole          `json:"role"`
	Fingerprints []RTCDtlsFingerprint `json:"fingerprints"`
}
