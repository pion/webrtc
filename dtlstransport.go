// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js
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
	"sync/atomic"
	"time"

	"github.com/pion/dtls/v3"
	"github.com/pion/dtls/v3/pkg/crypto/fingerprint"
	"github.com/pion/interceptor"
	"github.com/pion/logging"
	"github.com/pion/rtcp"
	"github.com/pion/srtp/v3"
	"github.com/pion/webrtc/v4/internal/mux"
	"github.com/pion/webrtc/v4/internal/util"
	"github.com/pion/webrtc/v4/pkg/rtcerr"
)

// DTLSTransport allows an application access to information about the DTLS
// transport over which RTP and RTCP packets are sent and received by
// RTPSender and RTPReceiver, as well other data such as SCTP packets sent
// and received by data channels.
type DTLSTransport struct {
	lock sync.RWMutex

	iceTransport          *ICETransport
	certificates          []Certificate
	remoteParameters      DTLSParameters
	remoteCertificate     []byte
	state                 DTLSTransportState
	srtpProtectionProfile srtp.ProtectionProfile

	onStateChangeHandler   func(DTLSTransportState)
	internalOnCloseHandler func()

	conn *dtls.Conn

	srtpSession, srtcpSession   atomic.Value
	srtpEndpoint, srtcpEndpoint *mux.Endpoint
	simulcastStreams            []simulcastStreamPair
	srtpReady                   chan struct{}

	dtlsMatcher mux.MatchFunc

	api *API
	log logging.LeveledLogger
}

type simulcastStreamPair struct {
	srtp  *srtp.ReadStreamSRTP
	srtcp *srtp.ReadStreamSRTCP
}

type streamsForSSRCResult struct {
	rtpReadStream   *srtp.ReadStreamSRTP
	rtpInterceptor  interceptor.RTPReader
	rtcpReadStream  *srtp.ReadStreamSRTCP
	rtcpInterceptor interceptor.RTCPReader
}

// NewDTLSTransport creates a new DTLSTransport.
// This constructor is part of the ORTC API. It is not
// meant to be used together with the basic WebRTC API.
func (api *API) NewDTLSTransport(transport *ICETransport, certificates []Certificate) (*DTLSTransport, error) {
	trans := &DTLSTransport{
		iceTransport: transport,
		api:          api,
		state:        DTLSTransportStateNew,
		dtlsMatcher:  mux.MatchDTLS,
		srtpReady:    make(chan struct{}),
		log:          api.settingEngine.LoggerFactory.NewLogger("DTLSTransport"),
	}

	if len(certificates) > 0 {
		now := time.Now()
		for _, x509Cert := range certificates {
			if !x509Cert.Expires().IsZero() && now.After(x509Cert.Expires()) {
				return nil, &rtcerr.InvalidAccessError{Err: ErrCertificateExpired}
			}
			trans.certificates = append(trans.certificates, x509Cert)
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
		trans.certificates = []Certificate{*certificate}
	}

	return trans, nil
}

// ICETransport returns the currently-configured *ICETransport or nil
// if one has not been configured.
func (t *DTLSTransport) ICETransport() *ICETransport {
	t.lock.RLock()
	defer t.lock.RUnlock()

	return t.iceTransport
}

// onStateChange requires the caller holds the lock.
func (t *DTLSTransport) onStateChange(state DTLSTransportState) {
	t.state = state
	handler := t.onStateChangeHandler
	if handler != nil {
		handler(state)
	}
}

// OnStateChange sets a handler that is fired when the DTLS
// connection state changes.
func (t *DTLSTransport) OnStateChange(f func(DTLSTransportState)) {
	t.lock.Lock()
	defer t.lock.Unlock()
	t.onStateChangeHandler = f
}

// State returns the current dtls transport state.
func (t *DTLSTransport) State() DTLSTransportState {
	t.lock.RLock()
	defer t.lock.RUnlock()

	return t.state
}

// WriteRTCP sends a user provided RTCP packet to the connected peer. If no peer is connected the
// packet is discarded.
func (t *DTLSTransport) WriteRTCP(pkts []rtcp.Packet) (int, error) {
	raw, err := rtcp.Marshal(pkts)
	if err != nil {
		return 0, err
	}

	srtcpSession, err := t.getSRTCPSession()
	if err != nil {
		return 0, err
	}

	writeStream, err := srtcpSession.OpenWriteStream()
	if err != nil {
		// nolint
		return 0, fmt.Errorf("%w: %v", errPeerConnWriteRTCPOpenWriteStream, err)
	}

	return writeStream.Write(raw)
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
// returns an empty list prior to selection of the remote certificate.
func (t *DTLSTransport) GetRemoteCertificate() []byte {
	t.lock.RLock()
	defer t.lock.RUnlock()

	return t.remoteCertificate
}

func (t *DTLSTransport) startSRTP() error {
	srtpConfig := &srtp.Config{
		Profile:       t.srtpProtectionProfile,
		BufferFactory: t.api.settingEngine.BufferFactory,
		LoggerFactory: t.api.settingEngine.LoggerFactory,
	}
	if t.api.settingEngine.replayProtection.SRTP != nil {
		srtpConfig.RemoteOptions = append(
			srtpConfig.RemoteOptions,
			srtp.SRTPReplayProtection(*t.api.settingEngine.replayProtection.SRTP),
		)
	}

	if t.api.settingEngine.disableSRTPReplayProtection {
		srtpConfig.RemoteOptions = append(
			srtpConfig.RemoteOptions,
			srtp.SRTPNoReplayProtection(),
		)
	}

	if t.api.settingEngine.replayProtection.SRTCP != nil {
		srtpConfig.RemoteOptions = append(
			srtpConfig.RemoteOptions,
			srtp.SRTCPReplayProtection(*t.api.settingEngine.replayProtection.SRTCP),
		)
	}

	if t.api.settingEngine.disableSRTCPReplayProtection {
		srtpConfig.RemoteOptions = append(
			srtpConfig.RemoteOptions,
			srtp.SRTCPNoReplayProtection(),
		)
	}

	connState, ok := t.conn.ConnectionState()
	if !ok {
		// nolint
		return fmt.Errorf("%w: Failed to get DTLS ConnectionState", errDtlsKeyExtractionFailed)
	}

	err := srtpConfig.ExtractSessionKeysFromDTLS(&connState, t.role() == DTLSRoleClient)
	if err != nil {
		// nolint
		return fmt.Errorf("%w: %v", errDtlsKeyExtractionFailed, err)
	}

	srtpSession, err := srtp.NewSessionSRTP(t.srtpEndpoint, srtpConfig)
	if err != nil {
		// nolint
		return fmt.Errorf("%w: %v", errFailedToStartSRTP, err)
	}

	srtcpSession, err := srtp.NewSessionSRTCP(t.srtcpEndpoint, srtpConfig)
	if err != nil {
		// nolint
		return fmt.Errorf("%w: %v", errFailedToStartSRTCP, err)
	}

	t.srtpSession.Store(srtpSession)
	t.srtcpSession.Store(srtcpSession)
	close(t.srtpReady)

	return nil
}

func (t *DTLSTransport) getSRTPSession() (*srtp.SessionSRTP, error) {
	if value, ok := t.srtpSession.Load().(*srtp.SessionSRTP); ok {
		return value, nil
	}

	return nil, errDtlsTransportNotStarted
}

func (t *DTLSTransport) getSRTCPSession() (*srtp.SessionSRTCP, error) {
	if value, ok := t.srtcpSession.Load().(*srtp.SessionSRTCP); ok {
		return value, nil
	}

	return nil, errDtlsTransportNotStarted
}

func (t *DTLSTransport) role() DTLSRole {
	// If remote has an explicit role use the inverse
	switch t.remoteParameters.Role {
	case DTLSRoleClient:
		return DTLSRoleServer
	case DTLSRoleServer:
		return DTLSRoleClient
	default:
	}

	// If SettingEngine has an explicit role
	switch t.api.settingEngine.answeringDTLSRole {
	case DTLSRoleServer:
		return DTLSRoleServer
	case DTLSRoleClient:
		return DTLSRoleClient
	default:
	}

	// Remote was auto and no explicit role was configured via SettingEngine
	if t.iceTransport.Role() == ICERoleControlling {
		return DTLSRoleServer
	}

	return defaultDtlsRoleAnswer
}

// Start DTLS transport negotiation with the parameters of the remote DTLS transport.
func (t *DTLSTransport) Start(remoteParameters DTLSParameters) error {
	role, certificate, err := t.prepareStart(remoteParameters)
	if err != nil {
		return err
	}

	dtlsEndpoint := t.iceTransport.newEndpoint(mux.MatchDTLS)
	dtlsEndpoint.SetOnClose(t.internalOnCloseHandler)

	sharedOpts := t.dtlsSharedOptions(certificate)

	dtlsConn, err := t.connectDTLS(dtlsEndpoint, role, sharedOpts)
	if err != nil {
		dtlsEndpoint.SetOnClose(nil)
		_ = dtlsEndpoint.Close()

		return t.failStart(err)
	}

	if err = t.handshakeDTLS(dtlsConn); err != nil {
		dtlsEndpoint.SetOnClose(nil)
		_ = dtlsConn.Close()

		return t.failStart(err)
	}

	if err = t.completeStart(dtlsConn); err != nil {
		dtlsEndpoint.SetOnClose(nil)
		_ = dtlsConn.Close()

		return err
	}

	return nil
}

func (t *DTLSTransport) prepareStart(remoteParameters DTLSParameters) (DTLSRole, tls.Certificate, error) {
	t.lock.Lock()
	defer t.lock.Unlock()

	if err := t.ensureICEConn(); err != nil {
		return DTLSRole(0), tls.Certificate{}, err
	}

	if t.state != DTLSTransportStateNew {
		return DTLSRole(0), tls.Certificate{}, &rtcerr.InvalidStateError{
			Err: fmt.Errorf("%w: %s", errInvalidDTLSStart, t.state),
		}
	}

	t.srtpEndpoint = t.iceTransport.newEndpoint(mux.MatchSRTP)
	t.srtcpEndpoint = t.iceTransport.newEndpoint(mux.MatchSRTCP)
	t.remoteParameters = remoteParameters

	cert := t.certificates[0]
	t.onStateChange(DTLSTransportStateConnecting)

	return t.role(), tls.Certificate{
		Certificate: [][]byte{cert.x509Cert.Raw},
		PrivateKey:  cert.privateKey,
	}, nil
}

func (t *DTLSTransport) dtlsSharedOptions(certificate tls.Certificate) []dtls.Option {
	sharedOpts := []dtls.Option{
		dtls.WithCertificates(certificate),
		dtls.WithSRTPProtectionProfiles(t.srtpProtectionProfiles()...),
		dtls.WithExtendedMasterSecret(t.api.settingEngine.dtls.extendedMasterSecret),
		dtls.WithInsecureSkipVerify(!t.api.settingEngine.dtls.disableInsecureSkipVerify),
		dtls.WithLoggerFactory(t.api.settingEngine.LoggerFactory),
		dtls.WithVerifyPeerCertificate(t.verifyPeerCertificateFunc()),
	}

	if t.api.settingEngine.dtls.customCipherSuites != nil {
		sharedOpts = append(
			sharedOpts,
			dtls.WithCustomCipherSuites(t.api.settingEngine.dtls.customCipherSuites),
		)
	}

	if t.api.settingEngine.dtls.retransmissionInterval > 0 {
		sharedOpts = append(
			sharedOpts,
			dtls.WithFlightInterval(t.api.settingEngine.dtls.retransmissionInterval),
		)
	}

	if t.api.settingEngine.replayProtection.DTLS != nil {
		sharedOpts = append(
			sharedOpts,
			dtls.WithReplayProtectionWindow(int(*t.api.settingEngine.replayProtection.DTLS)), //nolint:gosec // G115
		)
	}

	if t.api.settingEngine.dtls.cipherSuites != nil {
		sharedOpts = append(
			sharedOpts,
			dtls.WithCipherSuites(t.api.settingEngine.dtls.cipherSuites...),
		)
	}

	if len(t.api.settingEngine.dtls.ellipticCurves) > 0 {
		sharedOpts = append(
			sharedOpts,
			dtls.WithEllipticCurves(t.api.settingEngine.dtls.ellipticCurves...),
		)
	}

	if t.api.settingEngine.dtls.rootCAs != nil {
		sharedOpts = append(sharedOpts, dtls.WithRootCAs(t.api.settingEngine.dtls.rootCAs))
	}

	if t.api.settingEngine.dtls.keyLogWriter != nil {
		sharedOpts = append(sharedOpts, dtls.WithKeyLogWriter(t.api.settingEngine.dtls.keyLogWriter))
	}

	return sharedOpts
}

func (t *DTLSTransport) srtpProtectionProfiles() []dtls.SRTPProtectionProfile {
	if len(t.api.settingEngine.srtpProtectionProfiles) > 0 {
		return t.api.settingEngine.srtpProtectionProfiles
	}

	return defaultSrtpProtectionProfiles()
}

func (t *DTLSTransport) verifyPeerCertificateFunc() func([][]byte, [][]*x509.Certificate) error {
	return func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
		if len(rawCerts) == 0 {
			return errNoRemoteCertificate
		}

		t.lock.Lock()
		defer t.lock.Unlock()
		t.remoteCertificate = rawCerts[0]

		if t.api.settingEngine.disableCertificateFingerprintVerification {
			return nil
		}

		parsedRemoteCert, err := x509.ParseCertificate(t.remoteCertificate)
		if err != nil {
			return err
		}

		return t.validateFingerPrint(parsedRemoteCert)
	}
}

func (t *DTLSTransport) connectDTLS(
	dtlsEndpoint *mux.Endpoint,
	role DTLSRole,
	sharedOpts []dtls.Option,
) (*dtls.Conn, error) {
	if role == DTLSRoleClient {
		clientOpts := t.toDTLSClientOptions(sharedOpts)

		return dtls.ClientWithOptions(
			dtlsEndpoint,
			dtlsEndpoint.RemoteAddr(),
			clientOpts...,
		)
	}

	serverOpts := t.toDTLSServerOptions(sharedOpts)

	return dtls.ServerWithOptions(
		dtlsEndpoint,
		dtlsEndpoint.RemoteAddr(),
		serverOpts...,
	)
}

func (t *DTLSTransport) toDTLSServerOptions(sharedOpts []dtls.Option) []dtls.ServerOption {
	serverOpts := make([]dtls.ServerOption, 0, len(sharedOpts)+5)
	for _, opt := range sharedOpts {
		serverOpts = append(serverOpts, opt)
	}

	clientAuth := dtls.RequireAnyClientCert
	if t.api.settingEngine.dtls.clientAuth != nil {
		clientAuth = *t.api.settingEngine.dtls.clientAuth
	}

	serverOpts = append(serverOpts,
		dtls.WithClientAuth(clientAuth),
		dtls.WithClientCAs(t.api.settingEngine.dtls.clientCAs),
		dtls.WithInsecureSkipVerifyHello(t.api.settingEngine.dtls.insecureSkipHelloVerify),
	)

	if t.api.settingEngine.dtls.serverHelloMessageHook != nil {
		serverOpts = append(
			serverOpts,
			dtls.WithServerHelloMessageHook(t.api.settingEngine.dtls.serverHelloMessageHook),
		)
	}

	if t.api.settingEngine.dtls.certificateRequestMessageHook != nil {
		serverOpts = append(
			serverOpts,
			dtls.WithCertificateRequestMessageHook(t.api.settingEngine.dtls.certificateRequestMessageHook),
		)
	}

	return serverOpts
}

func (t *DTLSTransport) toDTLSClientOptions(sharedOpts []dtls.Option) []dtls.ClientOption {
	clientOpts := make([]dtls.ClientOption, 0, len(sharedOpts)+1)
	for _, opt := range sharedOpts {
		clientOpts = append(clientOpts, opt)
	}

	if t.api.settingEngine.dtls.clientHelloMessageHook != nil {
		clientOpts = append(
			clientOpts,
			dtls.WithClientHelloMessageHook(t.api.settingEngine.dtls.clientHelloMessageHook),
		)
	}

	return clientOpts
}

func (t *DTLSTransport) handshakeDTLS(dtlsConn *dtls.Conn) error {
	if t.api.settingEngine.dtls.connectContextMaker == nil {
		return dtlsConn.Handshake()
	}

	handshakeCtx, cancel := t.api.settingEngine.dtls.connectContextMaker()
	if cancel != nil {
		defer cancel()
	}

	return dtlsConn.HandshakeContext(handshakeCtx)
}

func (t *DTLSTransport) completeStart(dtlsConn *dtls.Conn) error {
	srtpProtectionProfile, err := srtpProtectionProfileFromDTLSConn(dtlsConn)

	t.lock.Lock()
	defer t.lock.Unlock()

	if err != nil {
		t.onStateChange(DTLSTransportStateFailed)

		return err
	}

	t.srtpProtectionProfile = srtpProtectionProfile
	t.conn = dtlsConn
	t.onStateChange(DTLSTransportStateConnected)

	return t.startSRTP()
}

func (t *DTLSTransport) failStart(err error) error {
	t.lock.Lock()
	defer t.lock.Unlock()
	t.onStateChange(DTLSTransportStateFailed)

	return err
}

func srtpProtectionProfileFromDTLSConn(dtlsConn *dtls.Conn) (srtp.ProtectionProfile, error) {
	srtpProfile, ok := dtlsConn.SelectedSRTPProtectionProfile()
	if !ok {
		return 0, ErrNoSRTPProtectionProfile
	}

	return srtpProtectionProfileFromDTLS(srtpProfile)
}

func srtpProtectionProfileFromDTLS(srtpProfile dtls.SRTPProtectionProfile) (srtp.ProtectionProfile, error) {
	switch srtpProfile {
	case dtls.SRTP_AEAD_AES_128_GCM:
		return srtp.ProtectionProfileAeadAes128Gcm, nil
	case dtls.SRTP_AEAD_AES_256_GCM:
		return srtp.ProtectionProfileAeadAes256Gcm, nil
	case dtls.SRTP_AES128_CM_HMAC_SHA1_80:
		return srtp.ProtectionProfileAes128CmHmacSha1_80, nil
	case dtls.SRTP_NULL_HMAC_SHA1_80:
		return srtp.ProtectionProfileNullHmacSha1_80, nil
	default:
		return 0, ErrNoSRTPProtectionProfile
	}
}

// Stop stops and closes the DTLSTransport object.
func (t *DTLSTransport) Stop() error {
	t.lock.Lock()
	defer t.lock.Unlock()

	// Try closing everything and collect the errors
	var closeErrs []error

	if srtpSession, err := t.getSRTPSession(); err == nil && srtpSession != nil {
		closeErrs = append(closeErrs, srtpSession.Close())
	}

	if srtcpSession, err := t.getSRTCPSession(); err == nil && srtcpSession != nil {
		closeErrs = append(closeErrs, srtcpSession.Close())
	}

	for i := range t.simulcastStreams {
		closeErrs = append(closeErrs, t.simulcastStreams[i].srtp.Close())
		closeErrs = append(closeErrs, t.simulcastStreams[i].srtcp.Close())
	}

	if t.conn != nil {
		// dtls connection may be closed on sctp close.
		if err := t.conn.Close(); err != nil && !errors.Is(err, dtls.ErrConnClosed) {
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

	return errNoMatchingCertificateFingerprint
}

func (t *DTLSTransport) ensureICEConn() error {
	if t.iceTransport == nil {
		return errICEConnectionNotStarted
	}

	return nil
}

func (t *DTLSTransport) storeSimulcastStream(
	srtpReadStream *srtp.ReadStreamSRTP,
	srtcpReadStream *srtp.ReadStreamSRTCP,
) {
	t.lock.Lock()
	defer t.lock.Unlock()

	t.simulcastStreams = append(t.simulcastStreams, simulcastStreamPair{srtpReadStream, srtcpReadStream})
}

func (t *DTLSTransport) streamsForSSRC(
	ssrc SSRC,
	streamInfo interceptor.StreamInfo,
) (*streamsForSSRCResult, error) {
	srtpSession, err := t.getSRTPSession()
	if err != nil {
		return nil, err
	}

	rtpReadStream, err := srtpSession.OpenReadStream(uint32(ssrc))
	if err != nil {
		return nil, err
	}

	rtpInterceptor := t.api.interceptor.BindRemoteStream(
		&streamInfo,
		interceptor.RTPReaderFunc(
			func(in []byte, a interceptor.Attributes) (n int, attributes interceptor.Attributes, err error) {
				n, err = rtpReadStream.Read(in)

				return n, a, err
			},
		),
	)

	srtcpSession, err := t.getSRTCPSession()
	if err != nil {
		return nil, err
	}

	rtcpReadStream, err := srtcpSession.OpenReadStream(uint32(ssrc))
	if err != nil {
		return nil, err
	}

	rtcpInterceptor := t.api.interceptor.BindRTCPReader(interceptor.RTCPReaderFunc(
		func(in []byte, a interceptor.Attributes) (n int, attributes interceptor.Attributes, err error) {
			n, err = rtcpReadStream.Read(in)

			return n, a, err
		}),
	)

	return &streamsForSSRCResult{
		rtpReadStream:   rtpReadStream,
		rtpInterceptor:  rtpInterceptor,
		rtcpReadStream:  rtcpReadStream,
		rtcpInterceptor: rtcpInterceptor,
	}, nil
}
