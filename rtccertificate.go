package webrtc

import (
	"crypto"
	"time"
)

// RTCCertificate represents a certificate used to authenticate WebRTC communications.
type RTCCertificate struct {
	privateKey crypto.PrivateKey
	Expires    time.Time
	Hello      string
}

func NewRTCCertificate(privateKey crypto.PrivateKey, expires time.Time) RTCCertificate {
	return RTCCertificate{
		privateKey: privateKey,
		Expires:    expires,
	}
}

// Equals determines if two certificates are identical
func (c RTCCertificate) Equals(other RTCCertificate) bool {
	return c.Expires == other.Expires
}

func (c RTCCertificate) GetFingerprints() {

}
