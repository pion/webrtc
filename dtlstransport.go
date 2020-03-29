// +build !js

package webrtc

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/pion/dtls/v2"
	"github.com/pion/dtls/v2/pkg/crypto/fingerprint"
	"github.com/pion/srtp"
	"github.com/pion/webrtc/v2/internal/mux"
	"github.com/pion/webrtc/v2/internal/util"
	"github.com/pion/webrtc/v2/pkg/rtcerr"
)

// DTLSTransport allows an application access to information about the DTLS
// transport over which RTP and RTCP packets are sent and received by
// RTPSender and RTPReceiver, as well other data such as SCTP packets sent
// and received by data channels.
type DTLSTransport struct {
	lock sync.RWMutex

	iceTransport      *ICETransport
	certificates      []Certificate
	remoteParameters  DTLSParameters
	remoteCertificate []byte
	state             DTLSTransportState

	onStateChangeHdlr func(DTLSTransportState)

	conn *dtls.Conn

	srtpSession   *srtp.SessionSRTP
	srtcpSession  *srtp.SessionSRTCP
	srtpEndpoint  *mux.Endpoint
	srtcpEndpoint *mux.Endpoint

	dtlsMatcher mux.MatchFunc

	api *API
}

// NewDTLSTransport creates a new DTLSTransport.
// This constructor is part of the ORTC API. It is not
// meant to be used together with the basic WebRTC API.
func (api *API) NewDTLSTransport(transport *ICETransport, certificates []Certificate) (*DTLSTransport, error) {
	t := &DTLSTransport{
		iceTransport: transport,
		api:          api,
		state:        DTLSTransportStateNew,
		dtlsMatcher:  mux.MatchDTLS,
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

// ICETransport returns the currently-configured *ICETransport or nil
// if one has not been configured
func (t *DTLSTransport) ICETransport() *ICETransport {
	t.lock.RLock()
	defer t.lock.RUnlock()
	return t.iceTransport
}

// onStateChange requires the caller holds the lock
func (t *DTLSTransport) onStateChange(state DTLSTransportState) {
	t.state = state
	hdlr := t.onStateChangeHdlr
	if hdlr != nil {
		hdlr(state)
	}
}

// OnStateChange sets a handler that is fired when the DTLS
// connection state changes.
func (t *DTLSTransport) OnStateChange(f func(DTLSTransportState)) {
	t.lock.Lock()
	defer t.lock.Unlock()
	t.onStateChangeHdlr = f
}

// State returns the current dtls transport state.
func (t *DTLSTransport) State() DTLSTransportState {
	t.lock.RLock()
	defer t.lock.RUnlock()
	return t.state
}

// GetLocalParameters returns the DTLS parameters of the local DTLSTransport upon construction.
func (t *DTLSTransport) GetLocalParameters() (DTLSParameters, error) {
	fingerprints := []DTLSFingerprint{}

	for _, c := range t.certificates {
		prints, err := c.GetFingerprints()
		if err != nil {
			return DTLSParameters{}, err
		}

		fingerprints = append(fingerprints, prints...)
	}

	return DTLSParameters{
		Role:         DTLSRoleAuto, // always returns the default role
		Fingerprints: fingerprints,
	}, nil
}

// GetRemoteCertificate returns the certificate chain in use by the remote side
// returns an empty list prior to selection of the remote certificate
func (t *DTLSTransport) GetRemoteCertificate() []byte {
	t.lock.RLock()
	defer t.lock.RUnlock()
	return t.remoteCertificate
}

func (t *DTLSTransport) startSRTP() error {
	t.lock.Lock()
	defer t.lock.Unlock()

	if t.srtpSession != nil && t.srtcpSession != nil {
		return nil
	} else if t.conn == nil {
		return fmt.Errorf("the DTLS transport has not started yet")
	}

	srtpConfig := &srtp.Config{
		Profile:       srtp.ProtectionProfileAes128CmHmacSha1_80,
		LoggerFactory: t.api.settingEngine.LoggerFactory,
	}
	if t.api.settingEngine.replayProtection.SRTP != nil {
		srtpConfig.RemoteOptions = append(
			srtpConfig.RemoteOptions,
			srtp.SRTPReplayProtection(*t.api.settingEngine.replayProtection.SRTP),
		)
	}
	if t.api.settingEngine.replayProtection.SRTCP != nil {
		srtpConfig.RemoteOptions = append(
			srtpConfig.RemoteOptions,
			srtp.SRTCPReplayProtection(*t.api.settingEngine.replayProtection.SRTCP),
		)
	}

	err := srtpConfig.ExtractSessionKeysFromDTLS(t.conn, t.role() == DTLSRoleClient)
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

func (t *DTLSTransport) getSRTPSession() (*srtp.SessionSRTP, error) {
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

func (t *DTLSTransport) getSRTCPSession() (*srtp.SessionSRTCP, error) {
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

func (t *DTLSTransport) role() DTLSRole {
	// If remote has an explicit role use the inverse
	switch t.remoteParameters.Role {
	case DTLSRoleClient:
		return DTLSRoleServer
	case DTLSRoleServer:
		return DTLSRoleClient
	}

	// If SettingEngine has an explicit role
	switch t.api.settingEngine.answeringDTLSRole {
	case DTLSRoleServer:
		return DTLSRoleServer
	case DTLSRoleClient:
		return DTLSRoleClient
	}

	// Remote was auto and no explicit role was configured via SettingEngine
	if t.iceTransport.Role() == ICERoleControlling {
		return DTLSRoleClient
	}
	return defaultDtlsRoleAnswer
}

// Start DTLS transport negotiation with the parameters of the remote DTLS transport
func (t *DTLSTransport) Start(remoteParameters DTLSParameters) error {
	// Take lock and prepare connection, we must not hold the lock
	// when connecting
	prepareTransport := func() (DTLSRole, *dtls.Config, error) {
		t.lock.Lock()
		defer t.lock.Unlock()

		if err := t.ensureICEConn(); err != nil {
			return DTLSRole(0), nil, err
		}

		if t.state != DTLSTransportStateNew {
			return DTLSRole(0), nil, &rtcerr.InvalidStateError{Err: fmt.Errorf("attempted to start DTLSTransport that is not in new state: %s", t.state)}
		}

		t.srtpEndpoint = t.iceTransport.NewEndpoint(mux.MatchSRTP)
		t.srtcpEndpoint = t.iceTransport.NewEndpoint(mux.MatchSRTCP)
		t.remoteParameters = remoteParameters

		// pion/webrtc#753
		cert := t.certificates[0]
		t.onStateChange(DTLSTransportStateConnecting)

		return t.role(), &dtls.Config{
			Certificates: []tls.Certificate{
				{
					Certificate: [][]byte{cert.x509Cert.Raw},
					PrivateKey:  cert.privateKey,
				}},
			SRTPProtectionProfiles: []dtls.SRTPProtectionProfile{dtls.SRTP_AES128_CM_HMAC_SHA1_80},
			ClientAuth:             dtls.RequireAnyClientCert,
			LoggerFactory:          t.api.settingEngine.LoggerFactory,
			InsecureSkipVerify:     true,
		}, nil
	}

	var dtlsConn *dtls.Conn
	dtlsEndpoint := t.iceTransport.NewEndpoint(mux.MatchDTLS)
	role, dtlsConfig, err := prepareTransport()
	if err != nil {
		return err
	}

	if t.api.settingEngine.replayProtection.DTLS != nil {
		dtlsConfig.ReplayProtectionWindow = int(*t.api.settingEngine.replayProtection.DTLS)
	}

	// Connect as DTLS Client/Server, function is blocking and we
	// must not hold the DTLSTransport lock
	if role == DTLSRoleClient {
		dtlsConn, err = dtls.Client(dtlsEndpoint, dtlsConfig)
	} else {
		dtlsConn, err = dtls.Server(dtlsEndpoint, dtlsConfig)
	}

	// Re-take the lock, nothing beyond here is blocking
	t.lock.Lock()
	defer t.lock.Unlock()

	if err != nil {
		t.onStateChange(DTLSTransportStateFailed)
		return err
	}

	t.conn = dtlsConn
	t.onStateChange(DTLSTransportStateConnected)

	if t.api.settingEngine.disableCertificateFingerprintVerification {
		return nil
	}

	// Check the fingerprint if a certificate was exchanged
	remoteCerts := t.conn.RemoteCertificate()
	if len(remoteCerts) == 0 {
		t.onStateChange(DTLSTransportStateFailed)
		return fmt.Errorf("peer didn't provide certificate via DTLS")
	}
	t.remoteCertificate = remoteCerts[0]

	parsedRemoteCert, err := x509.ParseCertificate(t.remoteCertificate)
	if err != nil {
		t.onStateChange(DTLSTransportStateFailed)
		return err
	}

	err = t.validateFingerPrint(parsedRemoteCert)
	if err != nil {
		t.onStateChange(DTLSTransportStateFailed)
	}
	return err
}

// Stop stops and closes the DTLSTransport object.
func (t *DTLSTransport) Stop() error {
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
		// dtls connection may be closed on sctp close.
		if err := t.conn.Close(); err != nil && err != dtls.ErrConnClosed {
			closeErrs = append(closeErrs, err)
		}
	}
	t.onStateChange(DTLSTransportStateClosed)
	return util.FlattenErrs(closeErrs)
}

func (t *DTLSTransport) validateFingerPrint(remoteCert *x509.Certificate) error {
	for _, fp := range t.remoteParameters.Fingerprints {
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

func (t *DTLSTransport) ensureICEConn() error {
	if t.iceTransport == nil || t.iceTransport.State() == ICETransportStateNew {
		return errors.New("ICE connection not started")
	}

	return nil
}
