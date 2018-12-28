package webrtc

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/pions/dtls/pkg/dtls"
	"github.com/pions/webrtc/pkg/quic"
	"github.com/pions/webrtc/pkg/rtcerr"
)

// RTCQuicTransport is a specialization of QuicTransportBase focused on
// peer-to-peer use cases and includes information relating to use of a
// QUIC transport with an ICE transport.
type RTCQuicTransport struct {
	lock sync.RWMutex
	quic.TransportBase

	iceTransport *RTCIceTransport
	certificates []RTCCertificate
}

// NewRTCQuicTransport creates a new RTCQuicTransport.
// This constructor is part of the ORTC API. It is not
// meant to be used together with the basic WebRTC API.
// Note that the Quic transport is a draft and therefore
// highly experimental. It is currently not supported by
// any browsers yet.
func NewRTCQuicTransport(transport *RTCIceTransport, certificates []RTCCertificate) (*RTCQuicTransport, error) {
	t := &RTCQuicTransport{iceTransport: transport}

	if len(certificates) > 0 {
		now := time.Now()
		for _, x509Cert := range certificates {
			if !x509Cert.Expires().IsZero() && now.After(x509Cert.Expires()) {
				return nil, &rtcerr.InvalidAccessError{Err: ErrCertificateExpired}
			}
			t.certificates = append(t.certificates, x509Cert)
		}
	} else {
		sk, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			return nil, &rtcerr.UnknownError{Err: err}
		}
		certificate, err := GenerateCertificate(sk)
		if err != nil {
			return nil, err
		}
		t.certificates = []RTCCertificate{*certificate}
	}

	return t, nil
}

// GetLocalParameters returns the Quic parameters of the local RTCQuicParameters upon construction.
func (t *RTCQuicTransport) GetLocalParameters() RTCQuicParameters {
	fingerprints := []RTCDtlsFingerprint{}

	for _, c := range t.certificates {
		prints := c.GetFingerprints() // TODO: Should be only one?
		fingerprints = append(fingerprints, prints...)
	}

	return RTCQuicParameters{
		Role:         RTCQuicRoleAuto, // always returns the default role
		Fingerprints: fingerprints,
	}
}

// Start Quic transport with the parameters of the remote
func (t *RTCQuicTransport) Start(remoteParameters RTCQuicParameters) error {
	t.lock.Lock()
	defer t.lock.Unlock()

	if err := t.ensureICEConn(); err != nil {
		return err
	}

	// TODO: handle multiple certs
	cert := t.certificates[0]

	isClient := true
	switch remoteParameters.Role {
	case RTCQuicRoleClient:
		isClient = true
	case RTCQuicRoleServer:
		isClient = false
	default:
		if t.iceTransport.Role() == RTCIceRoleControlling {
			isClient = false
		}
	}

	cfg := &quic.Config{
		Client:      isClient,
		Certificate: cert.x509Cert,
		PrivateKey:  cert.privateKey,
	}
	err := t.TransportBase.StartBase(t.iceTransport.conn, cfg)
	if err != nil {
		return err
	}

	// Check the fingerprint if a certificate was exchanged
	// TODO: Check why never received.
	remoteCerts := t.TransportBase.GetRemoteCertificates()
	if len(remoteCerts) > 0 {
		err := t.validateFingerPrint(remoteParameters, remoteCerts[0])
		if err != nil {
			return err
		}
	} else {
		fmt.Println("Warning: Certificate not checked")
	}

	return nil
}

func (t *RTCQuicTransport) validateFingerPrint(remoteParameters RTCQuicParameters, remoteCert *x509.Certificate) error {
	for _, fp := range remoteParameters.Fingerprints {
		hashAlgo, err := dtls.HashAlgorithmString(fp.Algorithm)
		if err != nil {
			return err
		}

		remoteValue, err := dtls.Fingerprint(remoteCert, hashAlgo)
		if err != nil {
			return err
		}

		if strings.ToLower(remoteValue) == strings.ToLower(fp.Value) {
			return nil
		}
	}

	return errors.New("No matching fingerprint")
}

func (t *RTCQuicTransport) ensureICEConn() error {
	if t.iceTransport == nil ||
		t.iceTransport.conn == nil {
		return errors.New("ICE connection not started")
	}

	return nil
}
