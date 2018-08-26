package webrtc

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"math/big"
	"time"
)

// RTCCertificate represents a x509Cert used to authenticate WebRTC communications.
type RTCCertificate struct {
	secretKey crypto.PrivateKey
	x509Cert  *x509.Certificate
}

func GenerateCertificate(secretKey crypto.PrivateKey) (*RTCCertificate, error) {
	origin := make([]byte, 16)
	if _, err := rand.Read(origin); err != nil {
		return nil, &UnknownError{err}
	}

	// Max random value, a 130-bits integer, i.e 2^130 - 1
	maxBigInt := new(big.Int)
	maxBigInt.Exp(big.NewInt(2), big.NewInt(130), nil).Sub(maxBigInt, big.NewInt(1))
	serialNumber, err := rand.Int(rand.Reader, maxBigInt)
	if err != nil {
		return nil, &UnknownError{err}
	}

	temp := &x509.Certificate{
		Version:      2,
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName: hex.EncodeToString(origin),
		},
		IsCA: true,
		BasicConstraintsValid: true,
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(0, 1, 0),
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageClientAuth,
			x509.ExtKeyUsageServerAuth,
		},
		KeyUsage: x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
	}

	var certDER []byte
	switch sk := secretKey.(type) {
	case *rsa.PrivateKey:
		pk := sk.Public()
		temp.SignatureAlgorithm = x509.SHA256WithRSA
		certDER, err = x509.CreateCertificate(rand.Reader, temp, temp, pk, sk)
		if err != nil {
			return nil, &UnknownError{err}
		}
	case *ecdsa.PrivateKey:
		pk := sk.Public()
		temp.SignatureAlgorithm = x509.ECDSAWithSHA256
		certDER, err = x509.CreateCertificate(rand.Reader, temp, temp, pk, sk)
		if err != nil {
			return nil, &UnknownError{err}
		}
	default:
		return nil, &NotSupportedError{ErrPrivateKeyType}
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return nil, &UnknownError{err}
	}

	return &RTCCertificate{secretKey: secretKey, x509Cert: cert}, nil
}

// Equals determines if two certificates are identical
func (c RTCCertificate) Equals(o RTCCertificate) bool {
	switch cSK := c.secretKey.(type) {
	case *rsa.PrivateKey:
		if oSK, ok := o.secretKey.(*rsa.PrivateKey); ok {
			if cSK.N.Cmp(oSK.N) != 0 {
				return false
			}
			return c.x509Cert.Equal(o.x509Cert)
		}
		return false
	case *ecdsa.PrivateKey:
		if oSK, ok := o.secretKey.(*ecdsa.PrivateKey); ok {
			if cSK.X.Cmp(oSK.X) != 0 || cSK.Y.Cmp(oSK.Y) != 0 {
				return false
			}
			return c.x509Cert.Equal(o.x509Cert)
		}
		return false
	default:
		return false
	}
}

func (c RTCCertificate) Expires() time.Time {
	if c.x509Cert == nil {
		return time.Time{}
	}
	return c.x509Cert.NotAfter
}

func (c RTCCertificate) GetFingerprints() {

}
