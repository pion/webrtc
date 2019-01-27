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

const (
	srtpMasterKeyLen     = 16
	srtpMasterKeySaltLen = 14
)

// RTCDtlsTransport allows an application access to information about the DTLS
// transport over which RTP and RTCP packets are sent and received by
// RTCRtpSender and RTCRtpReceiver, as well other data such as SCTP packets sent
// and received by data channels.
type RTCDtlsTransport struct {
	lock sync.RWMutex

	iceTransport *RTCIceTransport
	certificates []RTCCertificate
	// State     RTCDtlsTransportState

	// OnStateChange func()
	// OnError       func()

	conn *dtls.Conn

	srtpSession  *srtp.SessionSRTP
	srtpEndpoint *mux.Endpoint

	srtcpSession  *srtp.SessionSRTCP
	srtcpEndpoint *mux.Endpoint
}

// NewRTCDtlsTransport creates a new RTCDtlsTransport.
// This constructor is part of the ORTC API. It is not
// meant to be used together with the basic WebRTC API.
func (api *API) NewRTCDtlsTransport(transport *RTCIceTransport, certificates []RTCCertificate) (*RTCDtlsTransport, error) {
	t := &RTCDtlsTransport{
		iceTransport: transport,
		srtpSession:  srtp.CreateSessionSRTP(),
		srtcpSession: srtp.CreateSessionSRTCP(),
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

// Start DTLS transport negotiation with the parameters of the remote DTLS transport
func (t *RTCDtlsTransport) Start(remoteParameters RTCDtlsParameters) error {
	t.lock.Lock()
	defer t.lock.Unlock()

	if err := t.ensureICEConn(); err != nil {
		return err
	}

	mx := t.iceTransport.mux
	dtlsEndpoint := mx.NewEndpoint(mux.MatchDTLS)

	// TODO: handle multiple certs
	cert := t.certificates[0]

	isClient := true
	switch remoteParameters.Role {
	case RTCDtlsRoleClient:
		isClient = true
	case RTCDtlsRoleServer:
		isClient = false
	default:
		if t.iceTransport.Role() == RTCIceRoleControlling {
			isClient = false
		}
	}

	dtlsCofig := &dtls.Config{Certificate: cert.x509Cert, PrivateKey: cert.privateKey}
	if isClient {
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

	return t.startSRTP(isClient)
}

// Start SRTP transport
// RTCDtlsTransport needs to own SRTP state to satisfy ORTC interfaces
func (t *RTCDtlsTransport) startSRTP(isClient bool) error {
	keyingMaterial, err := t.conn.ExportKeyingMaterial([]byte("EXTRACTOR-dtls_srtp"), nil, (srtpMasterKeyLen*2)+(srtpMasterKeySaltLen*2))
	if err != nil {
		return err
	}

	t.srtpEndpoint = t.iceTransport.mux.NewEndpoint(mux.MatchSRTP)
	t.srtcpEndpoint = t.iceTransport.mux.NewEndpoint(mux.MatchSRTCP)

	offset := 0
	clientWriteKey := append([]byte{}, keyingMaterial[offset:offset+srtpMasterKeyLen]...)
	offset += srtpMasterKeyLen

	serverWriteKey := append([]byte{}, keyingMaterial[offset:offset+srtpMasterKeyLen]...)
	offset += srtpMasterKeyLen

	clientWriteKey = append(clientWriteKey, keyingMaterial[offset:offset+srtpMasterKeySaltLen]...)
	offset += srtpMasterKeySaltLen

	serverWriteKey = append(serverWriteKey, keyingMaterial[offset:offset+srtpMasterKeySaltLen]...)

	if !isClient {
		err = t.srtpSession.Start(
			serverWriteKey[0:16], serverWriteKey[16:],
			clientWriteKey[0:16], clientWriteKey[16:],
			srtp.ProtectionProfileAes128CmHmacSha1_80, t.srtpEndpoint,
		)

		if err == nil {
			err = t.srtcpSession.Start(
				serverWriteKey[0:16], serverWriteKey[16:],
				clientWriteKey[0:16], clientWriteKey[16:],
				srtp.ProtectionProfileAes128CmHmacSha1_80, t.srtcpEndpoint,
			)
		}
	} else {
		err = t.srtpSession.Start(
			clientWriteKey[0:16], clientWriteKey[16:],
			serverWriteKey[0:16], serverWriteKey[16:],
			srtp.ProtectionProfileAes128CmHmacSha1_80, t.srtpEndpoint,
		)

		if err == nil {
			err = t.srtcpSession.Start(
				clientWriteKey[0:16], clientWriteKey[16:],
				serverWriteKey[0:16], serverWriteKey[16:],
				srtp.ProtectionProfileAes128CmHmacSha1_80, t.srtcpEndpoint,
			)
		}
	}

	return err

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
