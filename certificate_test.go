//go:build !js
// +build !js

package webrtc

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGenerateCertificateRSA(t *testing.T) {
	sk, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.Nil(t, err)

	skPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(sk),
	})

	cert, err := GenerateCertificate(sk)
	assert.Nil(t, err)

	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert.x509Cert.Raw,
	})

	_, err = tls.X509KeyPair(certPEM, skPEM)
	assert.Nil(t, err)
}

func TestGenerateCertificateECDSA(t *testing.T) {
	sk, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	assert.Nil(t, err)

	skDER, err := x509.MarshalECPrivateKey(sk)
	assert.Nil(t, err)

	skPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: skDER,
	})

	cert, err := GenerateCertificate(sk)
	assert.Nil(t, err)

	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert.x509Cert.Raw,
	})

	_, err = tls.X509KeyPair(certPEM, skPEM)
	assert.Nil(t, err)
}

func TestGenerateCertificateEqual(t *testing.T) {
	sk1, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	assert.Nil(t, err)

	sk3, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err)

	cert1, err := GenerateCertificate(sk1)
	assert.Nil(t, err)

	sk2, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	assert.Nil(t, err)

	cert2, err := GenerateCertificate(sk2)
	assert.Nil(t, err)

	cert3, err := GenerateCertificate(sk3)
	assert.NoError(t, err)

	assert.True(t, cert1.Equals(*cert1))
	assert.False(t, cert1.Equals(*cert2))
	assert.True(t, cert3.Equals(*cert3))
}

func TestGenerateCertificateExpires(t *testing.T) {
	sk, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	assert.Nil(t, err)

	cert, err := GenerateCertificate(sk)
	assert.Nil(t, err)

	now := time.Now()
	assert.False(t, cert.Expires().IsZero() || now.After(cert.Expires()))

	x509Cert := CertificateFromX509(sk, &x509.Certificate{})
	assert.NotNil(t, x509Cert)
	assert.Contains(t, x509Cert.statsID, "certificate")
}

func TestPEM(t *testing.T) {
	sk, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	assert.Nil(t, err)
	cert, err := GenerateCertificate(sk)
	assert.Nil(t, err)

	pem, err := cert.PEM()
	assert.Nil(t, err)
	cert2, err := CertificateFromPEM(pem)
	assert.Nil(t, err)
	pem2, err := cert2.PEM()
	assert.Nil(t, err)
	assert.Equal(t, pem, pem2)
}
