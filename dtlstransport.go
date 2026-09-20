// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js

package webrtc

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
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
	"github.com/pion/transport/v4/packetio"
	packetioV5 "github.com/pion/transport/v5/packetio"
	"github.com/pion/webrtc/v4/internal/detacheddtls"
	"github.com/pion/webrtc/v4/internal/netconn"
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

	conn     *detacheddtls.Conn
	dtlsConn *dtls.DetachedConn

	srtpSession, srtcpSession   atomic.Value
	srtpEndpoint, srtcpEndpoint *netconn.Conn
	simulcastStreams            []simulcastStreamPair
	srtpReady                   chan struct{}

	api *API
	log logging.LeveledLogger
}

type simulcastStreamPair struct {
	srtp  *srtp.ReadStreamSRTP
	srtcp *srtp.ReadStreamSRTCP
}

type streamsForSSRCResult struct {
	rtpReadStream             *srtp.ReadStreamSRTP
	rtpInterceptor            interceptor.RTPReader
	startRTPReaderImmediately bool
	rtcpReadStream            *srtp.ReadStreamSRTCP
	rtcpInterceptor           interceptor.RTCPReader
}

type srtpRTPReader struct {
	readStream *srtp.ReadStreamSRTP
}

func (r *srtpRTPReader) Read(in []byte, a interceptor.Attributes) (int, interceptor.Attributes, error) {
	n, err := r.readStream.Read(in)

	return n, a, err
}

// NewDTLSTransport creates a new DTLSTransport.
// This constructor is part of the ORTC API. It is not
// meant to be used together with the basic WebRTC API.
func (api *API) NewDTLSTransport(transport *ICETransport, certificates []Certificate) (*DTLSTransport, error) {
	trans := &DTLSTransport{
		iceTransport: transport,
		api:          api,
		state:        DTLSTransportStateNew,
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

func (t *DTLSTransport) startSRTP() error { //nolint:cyclop
	srtpConfig := &srtp.Config{
		Profile:       t.srtpProtectionProfile,
		LoggerFactory: t.api.settingEngine.LoggerFactory,
	}
	if factory := t.api.settingEngine.BufferFactory; factory != nil {
		srtpConfig.BufferFactory = func(kind packetioV5.BufferPacketType, ssrc uint32) io.ReadWriteCloser {
			return factory(packetio.BufferPacketType(kind), ssrc)
		}
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

	connState, ok := t.dtlsConn.ConnectionState()
	if !ok {
		// nolint
		return fmt.Errorf("%w: Failed to get DTLS ConnectionState", errDtlsKeyExtractionFailed)
	}

	err := srtpConfig.ExtractSessionKeysFromDTLS(&connState, t.role() == DTLSRoleClient)
	if err != nil {
		// nolint
		return fmt.Errorf("%w: %v", errDtlsKeyExtractionFailed, err)
	}

	srtpSession, _ := t.getSRTPSession()
	if srtpSession == nil {
		srtpSession, err = srtp.NewSessionSRTP(t.srtpEndpoint, srtpConfig)
	} else {
		err = srtpSession.UpdateKey(srtpConfig.Keys, srtpConfig.Profile)
	}
	if err != nil {
		return fmt.Errorf("%w: %w", errFailedToStartSRTP, err)
	}
	srtcpSession, _ := t.getSRTCPSession()
	if srtcpSession == nil {
		srtcpSession, err = srtp.NewSessionSRTCP(t.srtcpEndpoint, srtpConfig)
	} else {
		err = srtcpSession.UpdateKey(srtpConfig.Keys, srtpConfig.Profile)
	}
	if err != nil {
		// nolint
		return fmt.Errorf("%w: %w", errFailedToStartSRTCP, err)
	}

	t.srtpSession.Store(srtpSession)
	t.srtcpSession.Store(srtcpSession)
	select {
	case <-t.srtpReady:
	default:
		close(t.srtpReady)
	}

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
	if role := oppositeDTLSRole(t.remoteParameters.Role); role != DTLSRoleAuto {
		return role
	}
	if role := t.api.settingEngine.answeringDTLSRole; role == DTLSRoleClient || role == DTLSRoleServer {
		return role
	}

	// Remote was auto and no explicit role was configured via SettingEngine
	if t.iceTransport.Role() == ICERoleControlling {
		return DTLSRoleServer
	}

	return defaultDtlsRoleAnswer
}

// Start DTLS transport negotiation with the parameters of the remote DTLS transport.
func (t *DTLSTransport) Start(remoteParameters DTLSParameters) error {
	return t.start(remoteParameters, DTLSTransportStateNew, t.api.settingEngine.dtls.connectContextMaker)
}

// StartContext starts DTLS negotiation. Canceling ctx interrupts the handshake.
//
//nolint:contextcheck // start obtains the caller's context through the factory.
func (t *DTLSTransport) StartContext(ctx context.Context, remoteParameters DTLSParameters) error {
	return t.start(remoteParameters, DTLSTransportStateNew, func() (context.Context, func()) { return ctx, nil })
}

//nolint:cyclop
func (t *DTLSTransport) start(
	remoteParameters DTLSParameters, previousState DTLSTransportState, newContext func() (context.Context, func()),
) error {
	t.lock.Lock()
	if err := t.ensureICEConn(); err != nil {
		t.lock.Unlock()

		return err
	}
	if t.state != previousState {
		state := t.state
		t.lock.Unlock()

		return &rtcerr.InvalidStateError{Err: fmt.Errorf("%w: %s", errInvalidDTLSStart, state)}
	}
	if t.srtpEndpoint == nil {
		t.srtpEndpoint = t.iceTransport.newEndpoint(iceEndpointSRTP)
		t.srtcpEndpoint = t.iceTransport.newEndpoint(iceEndpointSRTCP)
	}
	t.remoteParameters, t.remoteCertificate = remoteParameters, nil
	conn, cert, role := t.conn, t.certificates[0], t.role()
	t.onStateChange(DTLSTransportStateConnecting)
	t.lock.Unlock()

	ctx := context.Background()
	if newContext != nil {
		var cancel func()
		ctx, cancel = newContext()
		if cancel != nil {
			defer cancel()
		}
	}
	if conn != nil {
		if err := conn.Pause(); err != nil {
			return t.failStart(err)
		}
	}
	dtlsConn, err := t.connectDTLS(ctx, role, t.dtlsSharedOptions(tls.Certificate{
		Certificate: [][]byte{cert.x509Cert.Raw}, PrivateKey: cert.privateKey,
	}))
	if err != nil {
		return t.failStart(err)
	}

	t.lock.Lock()
	if t.state != DTLSTransportStateConnecting {
		state := t.state
		t.lock.Unlock()
		_ = dtlsConn.Close()

		return &rtcerr.InvalidStateError{Err: fmt.Errorf("%w: %s", errInvalidDTLSStart, state)}
	}
	if conn == nil {
		conn = detacheddtls.New(detacheddtls.Config{
			WriteDatagram:      t.iceTransport.write,
			SetDatagramHandler: t.iceTransport.setDTLSHandler,
			NetConn: netconn.Config{
				LocalAddr:        t.iceTransport.localAddr,
				RemoteAddr:       t.iceTransport.remoteAddr,
				SetWriteDeadline: t.iceTransport.setWriteDeadline,
			},
			OnClose: t.internalOnCloseHandler,
		})
		t.conn = conn
	}
	t.dtlsConn = dtlsConn
	t.lock.Unlock()

	err = conn.Start(ctx, dtlsConn)
	if err == nil {
		err = t.completeStart(dtlsConn)
	}
	if err != nil {
		return t.failStart(err)
	}

	return nil
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

	if len(t.api.settingEngine.dtls.supportedProtocols) > 0 {
		sharedOpts = append(
			sharedOpts,
			dtls.WithSupportedProtocols(t.api.settingEngine.dtls.supportedProtocols...),
		)
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
	ctx context.Context, role DTLSRole, opts []dtls.Option,
) (*dtls.DetachedConn, error) {
	for {
		if t.State() == DTLSTransportStateClosed || t.iceTransport.State() == ICETransportStateClosed ||
			t.iceTransport.State() == ICETransportStateFailed {
			return nil, io.ErrClosedPipe
		}
		if addr := t.iceTransport.remoteAddr(); addr != nil {
			if role == DTLSRoleClient {
				return dtls.DetachedClient(addr, t.toDTLSClientOptions(opts)...)
			}

			return dtls.DetachedServer(addr, t.toDTLSServerOptions(opts)...)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-t.iceTransport.connectionChanged:
		}
	}
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

func (t *DTLSTransport) completeStart(dtlsConn *dtls.DetachedConn) error {
	srtpProtectionProfile, err := srtpProtectionProfileFromDTLSConn(dtlsConn)

	t.lock.Lock()
	defer t.lock.Unlock()

	if t.state == DTLSTransportStateClosed {
		return io.ErrClosedPipe
	}
	if err != nil {
		return err
	}
	t.srtpProtectionProfile = srtpProtectionProfile
	if err = t.startSRTP(); err != nil {
		return err
	}
	t.onStateChange(DTLSTransportStateConnected)

	return nil
}

func (t *DTLSTransport) failStart(err error) error {
	t.lock.Lock()
	if t.state != DTLSTransportStateClosed {
		t.onStateChange(DTLSTransportStateFailed)
	}
	conn := t.conn
	t.conn, t.dtlsConn = nil, nil
	t.lock.Unlock()
	if conn != nil {
		_ = conn.Close()
	}

	return err
}

func srtpProtectionProfileFromDTLSConn(dtlsConn *dtls.DetachedConn) (srtp.ProtectionProfile, error) {
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
	conn := t.conn
	simulcastStreams := t.simulcastStreams
	t.simulcastStreams = nil
	t.onStateChange(DTLSTransportStateClosed)
	t.lock.Unlock()
	if t.iceTransport != nil {
		t.iceTransport.notifyConnectionChanged()
	}

	// Try closing everything and collect the errors
	var closeErrs []error

	if srtpSession, err := t.getSRTPSession(); err == nil && srtpSession != nil {
		closeErrs = append(closeErrs, srtpSession.Close())
	}

	if srtcpSession, err := t.getSRTCPSession(); err == nil && srtcpSession != nil {
		closeErrs = append(closeErrs, srtcpSession.Close())
	}

	for i := range simulcastStreams {
		closeErrs = append(closeErrs, simulcastStreams[i].srtp.Close())
		closeErrs = append(closeErrs, simulcastStreams[i].srtcp.Close())
	}

	if conn != nil {
		// dtls connection may be closed on sctp close.
		if err := conn.Close(); err != nil && !errors.Is(err, dtls.ErrConnClosed) {
			closeErrs = append(closeErrs, err)
		}
	}

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

	rtpReader := &srtpRTPReader{readStream: rtpReadStream}
	rtpInterceptor := t.api.interceptor.BindRemoteStream(&streamInfo, rtpReader)

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
		rtpReadStream:             rtpReadStream,
		rtpInterceptor:            rtpInterceptor,
		startRTPReaderImmediately: rtpInterceptor != rtpReader && t.api.settingEngine.BufferFactory == nil,
		rtcpReadStream:            rtcpReadStream,
		rtcpInterceptor:           rtcpInterceptor,
	}, nil
}
