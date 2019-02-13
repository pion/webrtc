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

	"github.com/pions/dtls"
	"github.com/pions/srtp"
	"github.com/pions/webrtc/internal/mux"
	"github.com/pions/webrtc/pkg/rtcerr"
)

// RTCDtlsTransport allows an application access to information about the DTLS
// transport over which RTP and RTCP packets are sent and received by
// RTCRtpSender and RTCRtpReceiver, as well other data such as SCTP packets sent
// and received by data channels.
type RTCDtlsTransport struct {
	lock sync.RWMutex

	iceTransport     *RTCIceTransport
	certificates     []RTCCertificate
	remoteParameters RTCDtlsParameters
	// State     RTCDtlsTransportState

	// OnStateChange func()
	// OnError       func()

	conn *dtls.Conn

	srtpSession   *srtp.SessionSRTP
	srtcpSession  *srtp.SessionSRTCP
	srtpEndpoint  *mux.Endpoint
	srtcpEndpoint *mux.Endpoint
}

// NewRTCDtlsTransport creates a new RTCDtlsTransport.
// This constructor is part of the ORTC API. It is not
// meant to be used together with the basic WebRTC API.
func (api *API) NewRTCDtlsTransport(transport *RTCIceTransport, certificates []RTCCertificate) (*RTCDtlsTransport, error) {
	t := &RTCDtlsTransport{iceTransport: transport}

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

// GetLocalParameters returns the DTLS parameters of the local RTCDtlsTransport upon construction.
func (t *RTCDtlsTransport) GetLocalParameters() RTCDtlsParameters {
	fingerprints := []RTCDtlsFingerprint{}

	for _, c := range t.certificates {
		prints := c.GetFingerprints() // TODO: Should be only one?
		fingerprints = append(fingerprints, prints...)
	}

	return RTCDtlsParameters{
		Role:         RTCDtlsRoleAuto, // always returns the default role
		Fingerprints: fingerprints,
	}
}

func (t *RTCDtlsTransport) startSRTP() error {
	t.lock.Lock()
	defer t.lock.Unlock()

	if t.srtpSession != nil && t.srtcpSession != nil {
		return nil
	} else if t.conn == nil {
		return fmt.Errorf("the DTLS transport has not started yet")
	}

	srtpConfig := &srtp.Config{
		Profile: srtp.ProtectionProfileAes128CmHmacSha1_80,
	}

	err := srtpConfig.ExtractSessionKeysFromDTLS(t.conn, t.isClient())
	if err != nil {
		return fmt.Errorf("failed to extract sctp session keys: %v", err)
	}

	srtpSession, err := srtp.NewSessionSRTP(t.srtpEndpoint, srtpConfig)
	if err != nil {
		return fmt.Errorf("failed to start srtp: %v", err)
	}

	srtcpSession, err := srtp.NewSessionSRTCP(t.srtcpEndpoint, srtpConfig)
	if err != nil {
		return fmt.Errorf("failed to start srtp: %v", err)
	}

	t.srtpSession = srtpSession
	t.srtcpSession = srtcpSession
	return nil
}

func (t *RTCDtlsTransport) getSRTPSession() (*srtp.SessionSRTP, error) {
	t.lock.RLock()
	if t.srtpSession != nil {
		t.lock.RUnlock()
		return t.srtpSession, nil
	}
	t.lock.RUnlock()

	if err := t.startSRTP(); err != nil {
		return nil, err
	}

	return t.srtpSession, nil
}

func (t *RTCDtlsTransport) getSRTCPSession() (*srtp.SessionSRTCP, error) {
	t.lock.RLock()
	if t.srtcpSession != nil {
		t.lock.RUnlock()
		return t.srtcpSession, nil
	}
	t.lock.RUnlock()

	if err := t.startSRTP(); err != nil {
		return nil, err
	}

	return t.srtcpSession, nil
}

func (t *RTCDtlsTransport) isClient() bool {
	isClient := true
	switch t.remoteParameters.Role {
	case RTCDtlsRoleClient:
		isClient = true
	case RTCDtlsRoleServer:
		isClient = false
	default:
		if t.iceTransport.Role() == RTCIceRoleControlling {
			isClient = false
		}
	}

	return isClient
}

// Start DTLS transport negotiation with the parameters of the remote DTLS transport
func (t *RTCDtlsTransport) Start(remoteParameters RTCDtlsParameters) error {
	t.lock.Lock()
	defer t.lock.Unlock()

	if err := t.ensureICEConn(); err != nil {
		return err
	}

	mx := t.iceTransport.mux
	dtlsEndpoint := mx.NewEndpoint(mux.MatchDTLS)
	t.srtpEndpoint = mx.NewEndpoint(mux.MatchSRTP)
	t.srtcpEndpoint = mx.NewEndpoint(mux.MatchSRTCP)

	// TODO: handle multiple certs
	cert := t.certificates[0]

	dtlsCofig := &dtls.Config{Certificate: cert.x509Cert, PrivateKey: cert.privateKey, SRTPProtectionProfiles: []dtls.SRTPProtectionProfile{dtls.SRTP_AES128_CM_HMAC_SHA1_80}}
	if t.isClient() {
		// Assumes the peer offered to be passive and we accepted.
		dtlsConn, err := dtls.Client(dtlsEndpoint, dtlsCofig)
		if err != nil {
			return err
		}
		t.conn = dtlsConn
	} else {
		// Assumes we offer to be passive and this is accepted.
		dtlsConn, err := dtls.Server(dtlsEndpoint, dtlsCofig)
		if err != nil {
			return err
		}
		t.conn = dtlsConn
	}

	// Check the fingerprint if a certificate was exchanged
	remoteCert := t.conn.RemoteCertificate()
	if remoteCert != nil {
		err := t.validateFingerPrint(remoteParameters, remoteCert)
		if err != nil {
			return err
		}
	} else {
		fmt.Println("Warning: Certificate not checked")
	}

	return nil
}

// Stop stops and closes the RTCDtlsTransport object.
func (t *RTCDtlsTransport) Stop() error {
	t.lock.Lock()
	defer t.lock.Unlock()

	// Try closing everything and collect the errors
	var closeErrs []error

	if t.srtpSession != nil {
		if err := t.srtpSession.Close(); err != nil {
			closeErrs = append(closeErrs, err)
		}
	}

	if t.srtcpSession != nil {
		if err := t.srtcpSession.Close(); err != nil {
			closeErrs = append(closeErrs, err)
		}
	}

	if t.conn != nil {
		if err := t.conn.Close(); err != nil {
			closeErrs = append(closeErrs, err)
		}
	}
	return flattenErrs(closeErrs)
}

func (t *RTCDtlsTransport) validateFingerPrint(remoteParameters RTCDtlsParameters, remoteCert *x509.Certificate) error {
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

func (t *RTCDtlsTransport) ensureICEConn() error {
	if t.iceTransport == nil ||
		t.iceTransport.conn == nil ||
		t.iceTransport.mux == nil {
		return errors.New("ICE connection not started")
	}

	return nil
}
