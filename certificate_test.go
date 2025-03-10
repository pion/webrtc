// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

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

func TestBadCertificate(t *testing.T) {
	var nokey interface{}
	badcert, err := NewCertificate(nokey, x509.Certificate{})
	assert.Nil(t, badcert)
	assert.Error(t, err)

	sk, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	assert.Nil(t, err)

	badcert, err = NewCertificate(sk, x509.Certificate{})
	assert.Nil(t, badcert)
	assert.Error(t, err)

	c0 := Certificate{}
	c1 := Certificate{}
	assert.False(t, c0.Equals(c1))
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

var certHeader = `!! This is a test certificate: Don't use it in production !!
You can create your own using openssl
` + "```sh" + `
openssl req -new -sha256 -newkey ec -pkeyopt ec_paramgen_curve:prime256v1 ` +
`-x509 -nodes -days 365 -out cert.pem -keyout cert.pem -subj "/CN=WebRTC"
openssl x509 -in cert.pem -noout -fingerprint -sha256
` + "```\n"

var certPriv = `-----BEGIN PRIVATE KEY-----
MIGHAgEAMBMGByqGSM49AgEGCCqGSM49AwEHBG0wawIBAQQg2XFaTNqFpTUqNtG9
A21MEe04JtsWVpUTDD8nI0KvchKhRANCAAS1nqME3jS5GFicwYfGDYaz7oSINwWm
X4BkfsSCxMrhr7mPtfxOi4Lxy/P3w6EvSSEU8t5E9ouKIWh5xPS9dYwu
-----END PRIVATE KEY-----
`
var certCert = `-----BEGIN CERTIFICATE-----
MIIBljCCATugAwIBAgIUQa1sD+5HG43K+hCEVZLYxB68/hQwCgYIKoZIzj0EAwIw
IDEeMBwGA1UEAwwVc3dpdGNoLmV2YW4tYnJhc3MubmV0MB4XDTI0MDQyNDIwMjEy
MFoXDTI1MDQyNDIwMjEyMFowIDEeMBwGA1UEAwwVc3dpdGNoLmV2YW4tYnJhc3Mu
bmV0MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEtZ6jBN40uRhYnMGHxg2Gs+6E
iDcFpl+AZH7EgsTK4a+5j7X8TouC8cvz98OhL0khFPLeRPaLiiFoecT0vXWMLqNT
MFEwHQYDVR0OBBYEFGecfGnYqZFVgUApHGgX2kSIhUusMB8GA1UdIwQYMBaAFGec
fGnYqZFVgUApHGgX2kSIhUusMA8GA1UdEwEB/wQFMAMBAf8wCgYIKoZIzj0EAwID
SQAwRgIhAJ3VWO8JZ7FEOJhxpUCeyOgl+G4vXSHtj9J9NRD3uGGZAiEAsTKGLOGE
9c6CtLDU9Ohf1c+Xj2Yi9H+srLZj1mrsnd4=
-----END CERTIFICATE-----
`

func TestOpensslCert(t *testing.T) {
	// Check that CertificateFromPEM can parse certificates with the PRIVATE KEY before the CERTIFICATE block
	cert, err := CertificateFromPEM(certHeader + certPriv + certCert)
	assert.Nil(t, err)
	_ = cert
}

func TestEmpty(t *testing.T) {
	cert, err := CertificateFromPEM("")
	assert.Nil(t, cert)
	assert.Equal(t, errCertificatePEMMissing, err)
}

func TestMultiCert(t *testing.T) {
	cert, err := CertificateFromPEM(certHeader + certCert + certPriv + certCert)
	assert.Nil(t, cert)
	assert.Equal(t, errCertificatePEMMultipleCert, err)
}

func TestMultiPriv(t *testing.T) {
	cert, err := CertificateFromPEM(certPriv + certHeader + certCert + certPriv)
	assert.Nil(t, cert)
	assert.Equal(t, errCertificatePEMMultiplePriv, err)
}
