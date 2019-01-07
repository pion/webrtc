package quic

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"math/big"
	"testing"
	"time"

	"github.com/pions/transport/test"
)

func TestTransport_E2E(t *testing.T) {
	// Limit runtime in case of deadlocks
	lim := test.TimeOut(time.Second * 20)
	defer lim.Stop()

	// TODO: Check how we can make sure quic-go closes without leaking
	// report := test.CheckRoutines(t)
	// defer report()

	url := "localhost:50000"

	cert, key, err := GenerateSelfSigned()
	if err != nil {
		t.Fatal(err)
	}

	cfgA := &Config{Certificate: cert, PrivateKey: key}

	cert, key, err = GenerateSelfSigned()
	if err != nil {
		t.Fatal(err)
	}

	cfgB := &Config{Certificate: cert, PrivateKey: key}

	srvErr := make(chan error)
	awaitSetup := make(chan struct{})

	var tb *Transport
	go func() {
		var sErr error
		tb, sErr = newServer(url, cfgB)
		if sErr != nil {
			srvErr <- sErr
		}

		tb.OnBidirectionalStream(func(stream *BidirectionalStream) {
			go readLoop(stream) // Read to pull incoming messages

			close(awaitSetup)
		})
		close(srvErr)
	}()

	ta, err := NewTransport(url, cfgA)
	if err != nil {
		t.Fatal(err)
	}

	stream, err := ta.CreateBidirectionalStream()
	if err != nil {
		t.Fatal(err)
	}

	go readLoop(stream) // Read to pull incoming messages

	// Write to open stream
	data := StreamWriteParameters{
		Data: []byte("Hello"),
	}
	err = stream.Write(data)
	if err != nil {
		t.Fatal(err)
	}

	err = <-srvErr
	if err != nil {
		t.Fatal(err)
	}
	<-awaitSetup

	err = ta.Stop(TransportStopInfo{})
	if err != nil {
		t.Fatal(err)
	}

	err = tb.Stop(TransportStopInfo{})
	if err != nil {
		t.Fatal(err)
	}
}

func readLoop(s *BidirectionalStream) {
	for {
		buffer := make([]byte, 15)
		_, err := s.ReadInto(buffer)
		if err != nil {
			return
		}
	}
}

// GenerateSelfSigned creates a self-signed certificate
func GenerateSelfSigned() (*x509.Certificate, crypto.PrivateKey, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}

	origin := make([]byte, 16)

	// Max random value, a 130-bits integer, i.e 2^130 - 1
	maxBigInt := new(big.Int)
	/* #nosec */
	maxBigInt.Exp(big.NewInt(2), big.NewInt(130), nil).Sub(maxBigInt, big.NewInt(1))
	serialNumber, err := rand.Int(rand.Reader, maxBigInt)
	if err != nil {
		return nil, nil, err
	}

	template := x509.Certificate{
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
	}

	raw, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return nil, nil, err
	}

	cert, err := x509.ParseCertificate(raw)
	if err != nil {
		return nil, nil, err
	}

	return cert, priv, nil
}
