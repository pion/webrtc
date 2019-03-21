// +build !js

package webrtc

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"fmt"
	"math/big"
	"time"

	"github.com/pions/dtls"
	"github.com/pions/webrtc/pkg/rtcerr"
)

// Certificate represents a x509Cert used to authenticate WebRTC communications.
type Certificate struct {
	privateKey crypto.PrivateKey
	x509Cert   *x509.Certificate
}

// NewCertificate generates a new x509 compliant Certificate to be used
// by DTLS for encrypting data sent over the wire. This method differs from
// GenerateCertificate by allowing to specify a template x509.Certificate to
// be used in order to define certificate parameters.
func NewCertificate(key crypto.PrivateKey, tpl x509.Certificate) (*Certificate, error) {
	var err error
	var certDER []byte
	switch sk := key.(type) {
	case *rsa.PrivateKey:
		pk := sk.Public()
		tpl.SignatureAlgorithm = x509.SHA256WithRSA
		certDER, err = x509.CreateCertificate(rand.Reader, &tpl, &tpl, pk, sk)
		if err != nil {
			return nil, &rtcerr.UnknownError{Err: err}
		}
	case *ecdsa.PrivateKey:
		pk := sk.Public()
		tpl.SignatureAlgorithm = x509.ECDSAWithSHA256
		certDER, err = x509.CreateCertificate(rand.Reader, &tpl, &tpl, pk, sk)
		if err != nil {
			return nil, &rtcerr.UnknownError{Err: err}
		}
	default:
		return nil, &rtcerr.NotSupportedError{Err: ErrPrivateKeyType}
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return nil, &rtcerr.UnknownError{Err: err}
	}

	return &Certificate{privateKey: key, x509Cert: cert}, nil
}

// Equals determines if two certificates are identical by comparing both the
// secretKeys and x509Certificates.
func (c Certificate) Equals(o Certificate) bool {
	switch cSK := c.privateKey.(type) {
	case *rsa.PrivateKey:
		if oSK, ok := o.privateKey.(*rsa.PrivateKey); ok {
			if cSK.N.Cmp(oSK.N) != 0 {
				return false
			}
			return c.x509Cert.Equal(o.x509Cert)
		}
		return false
	case *ecdsa.PrivateKey:
		if oSK, ok := o.privateKey.(*ecdsa.PrivateKey); ok {
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

// Expires returns the timestamp after which this certificate is no longer valid.
func (c Certificate) Expires() time.Time {
	if c.x509Cert == nil {
		return time.Time{}
	}
	return c.x509Cert.NotAfter
}

// GetFingerprints returns the list of certificate fingerprints, one of which
// is computed with the digest algorithm used in the certificate signature.
func (c Certificate) GetFingerprints() []DTLSFingerprint {
	fingerprintAlgorithms := []dtls.HashAlgorithm{dtls.HashAlgorithmSHA256}
	res := make([]DTLSFingerprint, len(fingerprintAlgorithms))

	i := 0
	for _, algo := range fingerprintAlgorithms {
		value, err := dtls.Fingerprint(c.x509Cert, algo)
		if err != nil {
			fmt.Printf("Failed to create fingerprint: %v\n", err)
			continue
		}
		res[i] = DTLSFingerprint{
			Algorithm: algo.String(),
			Value:     value,
		}
	}

	return res[:i+1]
}

// GenerateCertificate causes the creation of an X.509 certificate and
// corresponding private key.
func GenerateCertificate(secretKey crypto.PrivateKey) (*Certificate, error) {
	origin := make([]byte, 16)
	/* #nosec */
	if _, err := rand.Read(origin); err != nil {
		return nil, &rtcerr.UnknownError{Err: err}
	}

	// Max random value, a 130-bits integer, i.e 2^130 - 1
	maxBigInt := new(big.Int)
	/* #nosec */
	maxBigInt.Exp(big.NewInt(2), big.NewInt(130), nil).Sub(maxBigInt, big.NewInt(1))
	/* #nosec */
	serialNumber, err := rand.Int(rand.Reader, maxBigInt)
	if err != nil {
		return nil, &rtcerr.UnknownError{Err: err}
	}

	return NewCertificate(secretKey, x509.Certificate{
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageClientAuth,
			x509.ExtKeyUsageServerAuth,
		},
		BasicConstraintsValid: true,
		NotBefore:             time.Now(),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		NotAfter:              time.Now().AddDate(0, 1, 0),
		SerialNumber:          serialNumber,
		Version:               2,
		Subject:               pkix.Name{CommonName: hex.EncodeToString(origin)},
		IsCA:                  true,
	})
}
