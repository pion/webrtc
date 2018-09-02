package webrtc

type RTCDtlsFingerprint struct {
	// Algorithm specifies one of the the hash function algorithms defined in
	// the 'Hash function Textual Names' registry.
	Algorithm string

	// Value specifies the value of the certificate fingerprint in lowercase
	// hex string as expressed utilizing the syntax of 'fingerprint' in
	// https://tools.ietf.org/html/rfc4572#section-5.
	Value string
}
