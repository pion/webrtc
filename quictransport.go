// +build !js
// +build quic

package webrtc

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/pion/dtls/v2/pkg/crypto/fingerprint"
	"github.com/pion/logging"
	"github.com/pion/quic"
	"github.com/pion/webrtc/v2/internal/mux"
	"github.com/pion/webrtc/v2/pkg/rtcerr"
)

// QUICTransport is a specialization of QuicTransportBase focused on
// peer-to-peer use cases and includes information relating to use of a
// QUIC transport with an ICE transport.
type QUICTransport struct {
	lock sync.RWMutex
	quic.TransportBase

	iceTransport *ICETransport
	certificates []Certificate

	api *API
	log logging.LeveledLogger
}

// NewQUICTransport creates a new QUICTransport.
// This constructor is part of the ORTC API. It is not
// meant to be used together with the basic WebRTC API.
// Note that the Quic transport is a draft and therefore
// highly experimental. It is currently not supported by
// any browsers yet.
func (api *API) NewQUICTransport(transport *ICETransport, certificates []Certificate) (*QUICTransport, error) {
	t := &QUICTransport{
		iceTransport: transport,
		api:          api,
		log:          api.settingEngine.LoggerFactory.NewLogger("quic"),
	}

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
		t.certificates = []Certificate{*certificate}
	}

	return t, nil
}

// GetLocalParameters returns the Quic parameters of the local QUICParameters upon construction.
func (t *QUICTransport) GetLocalParameters() (QUICParameters, error) {
	fingerprints := []DTLSFingerprint{}

	for _, c := range t.certificates {
		prints, err := c.GetFingerprints()
		if err != nil {
			return QUICParameters{}, err
		}
		fingerprints = append(fingerprints, prints...)
	}

	return QUICParameters{
		Role:         QUICRoleAuto, // always returns the default role
		Fingerprints: fingerprints,
	}, nil
}

// Start Quic transport with the parameters of the remote
func (t *QUICTransport) Start(remoteParameters QUICParameters) error {
	t.lock.Lock()
	defer t.lock.Unlock()

	if err := t.ensureICEConn(); err != nil {
		return err
	}

	// pion/webrtc#753
	cert := t.certificates[0]

	isClient := true
	switch remoteParameters.Role {
	case QUICRoleClient:
		isClient = true
	case QUICRoleServer:
		isClient = false
	default:
		if t.iceTransport.Role() == ICERoleControlling {
			isClient = false
		}
	}

	cfg := &quic.Config{
		Client:      isClient,
		Certificate: cert.x509Cert,
		PrivateKey:  cert.privateKey,
	}
	endpoint := t.iceTransport.NewEndpoint(mux.MatchAll)
	err := t.TransportBase.StartBase(endpoint, cfg)
	if err != nil {
		return err
	}

	// Check the fingerprint if a certificate was exchanged
	remoteCerts := t.TransportBase.GetRemoteCertificates()
	if len(remoteCerts) > 0 {
		err := t.validateFingerPrint(remoteParameters, remoteCerts[0])
		if err != nil {
			return err
		}
	} else {
		t.log.Errorf("Warning: Certificate not checked")
	}

	return nil
}

func (t *QUICTransport) validateFingerPrint(remoteParameters QUICParameters, remoteCert *x509.Certificate) error {
	for _, fp := range remoteParameters.Fingerprints {
		hashAlgo, err := fingerprint.HashFromString(fp.Algorithm)
		if err != nil {
			return err
		}

		remoteValue, err := fingerprint.Fingerprint(remoteCert, hashAlgo)
		if err != nil {
			return err
		}

		if strings.EqualFold(remoteValue, fp.Value) {
			return nil
		}
	}

	return errors.New("no matching fingerprint")
}

func (t *QUICTransport) ensureICEConn() error {
	if t.iceTransport == nil ||
		t.iceTransport.State() == ICETransportStateNew {
		return errors.New("ICE connection not started")
	}

	return nil
}
