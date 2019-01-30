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
	"github.com/pions/rtcp"
	"github.com/pions/rtp"
	"github.com/pions/srtp"
	"github.com/pions/webrtc/internal/mux"
	"github.com/pions/webrtc/pkg/rtcerr"
)

// DTLSTransport allows an application access to information about the DTLS
// transport over which RTP and RTCP packets are sent and received by
// RTPSender and RTPReceiver, as well other data such as SCTP packets sent
// and received by data channels.
type DTLSTransport struct {
	lock sync.RWMutex

	iceTransport     *ICETransport
	certificates     []Certificate
	remoteParameters DTLSParameters
	// State     DTLSTransportState

	// OnStateChange func()
	// OnError       func()

	conn *dtls.Conn

	srtpSession   *srtp.SessionSRTP
	srtcpSession  *srtp.SessionSRTCP
	srtpEndpoint  *mux.Endpoint
	srtcpEndpoint *mux.Endpoint
}

// NewDTLSTransport creates a new DTLSTransport.
// This constructor is part of the ORTC API. It is not
// meant to be used together with the basic WebRTC API.
func (api *API) NewDTLSTransport(transport *ICETransport, certificates []Certificate) (*DTLSTransport, error) {
	t := &DTLSTransport{iceTransport: transport}

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

// GetLocalParameters returns the DTLS parameters of the local DTLSTransport upon construction.
func (t *DTLSTransport) GetLocalParameters() DTLSParameters {
	fingerprints := []DTLSFingerprint{}

	for _, c := range t.certificates {
		prints := c.GetFingerprints() // TODO: Should be only one?
		fingerprints = append(fingerprints, prints...)
	}

	return DTLSParameters{
		Role:         DTLSRoleAuto, // always returns the default role
		Fingerprints: fingerprints,
	}
}

// Note: the caller should hold the datachannel lock.
func (t *DTLSTransport) startSRTP() error {
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
		return fmt.Errorf("failed to start srtcp: %v", err)
	}

	t.lock.Lock()
	t.srtpSession = srtpSession
	t.srtcpSession = srtcpSession
	t.lock.Unlock()

	return nil
}

func (t *DTLSTransport) getSrtpSession() (*srtp.SessionSRTP, error) {
	t.lock.RLock()
	defer t.lock.RUnlock()

	if t.srtpSession == nil {
		return nil, errors.New("the SRTP session is not started")
	}

	return t.srtpSession, nil
}

func (t *DTLSTransport) getSrtcpSession() (*srtp.SessionSRTCP, error) {
	t.lock.RLock()
	defer t.lock.RUnlock()

	if t.srtcpSession == nil {
		return nil, errors.New("the SRTCP session is not started")
	}

	return t.srtcpSession, nil
}

// drainSRTP pulls and discards RTP/RTCP packets that don't match any SRTP
// These could be sent to the user, but right now we don't provide an API
// to distribute orphaned RTCP messages. This is needed to make sure we don't block
// and provides useful debugging messages
func (t *DTLSTransport) drainSRTP() {
	go t.drainRTPSessions()
	go t.drainRTCPSessions()
}

func (t *DTLSTransport) drainRTPSessions() {
	srtpSession := t.srtpSession
	for {
		s, ssrc, err := srtpSession.AcceptStream()
		if err != nil {
			pcLog.Warnf("Failed to accept RTP %v \n", err)
			return
		}

		go t.drainRTPStream(s, ssrc)
	}
}

func (t *DTLSTransport) drainRTPStream(stream *srtp.ReadStreamSRTP, ssrc uint32) {
	rtpBuf := make([]byte, receiveMTU)
	rtpPacket := &rtp.Packet{}

	for {
		i, err := stream.Read(rtpBuf)
		if err != nil {
			pcLog.Warnf("Failed to read, drainSRTP done for: %v %d \n", err, ssrc)
			return
		}

		if err := rtpPacket.Unmarshal(rtpBuf[:i]); err != nil {
			pcLog.Warnf("Failed to unmarshal RTP packet, discarding: %v \n", err)
			continue
		}
		pcLog.Debugf("got RTP: %+v", rtpPacket)
	}
}

func (t *DTLSTransport) drainRTCPSessions() {
	srtcpSession := t.srtcpSession
	for {
		s, ssrc, err := srtcpSession.AcceptStream()
		if err != nil {
			pcLog.Warnf("Failed to accept RTCP %v \n", err)
			return
		}

		go t.drainRTCPStream(s, ssrc)
	}
}

func (t *DTLSTransport) drainRTCPStream(stream *srtp.ReadStreamSRTCP, ssrc uint32) {
	rtcpBuf := make([]byte, receiveMTU)
	for {
		i, err := stream.Read(rtcpBuf)
		if err != nil {
			pcLog.Warnf("Failed to read, drainSRTCP done for: %v %d \n", err, ssrc)
			return
		}

		rtcpPacket, _, err := rtcp.Unmarshal(rtcpBuf[:i])
		if err != nil {
			pcLog.Warnf("Failed to unmarshal RTCP packet, discarding: %v \n", err)
			continue
		}
		pcLog.Debugf("got RTCP: %+v", rtcpPacket)
	}
}

func (t *DTLSTransport) isClient() bool {
	isClient := true
	switch t.remoteParameters.Role {
	case DTLSRoleClient:
		isClient = true
	case DTLSRoleServer:
		isClient = false
	default:
		if t.iceTransport.Role() == ICERoleControlling {
			isClient = false
		}
	}

	return isClient
}

// Start DTLS transport negotiation with the parameters of the remote DTLS transport
func (t *DTLSTransport) Start(remoteParameters DTLSParameters) error {
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

	dtlsCofig := &dtls.Config{
		Certificate:            cert.x509Cert,
		PrivateKey:             cert.privateKey,
		SRTPProtectionProfiles: []dtls.SRTPProtectionProfile{dtls.SRTP_AES128_CM_HMAC_SHA1_80},
		ClientAuth:             dtls.RequireAnyClientCert,
	}
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
	if remoteCert == nil {
		return fmt.Errorf("peer didn't provide certificate via DTLS")
	}

	return t.validateFingerPrint(remoteParameters, remoteCert)
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
		if err := t.conn.Close(); err != nil {
			closeErrs = append(closeErrs, err)
		}
	}

	return flattenErrs(closeErrs)
}

func (t *DTLSTransport) validateFingerPrint(remoteParameters DTLSParameters, remoteCert *x509.Certificate) error {
	for _, fp := range remoteParameters.Fingerprints {
		hashAlgo, err := dtls.HashAlgorithmString(fp.Algorithm)
		if err != nil {
			return err
		}

		remoteValue, err := dtls.Fingerprint(remoteCert, hashAlgo)
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
	if t.iceTransport == nil ||
		t.iceTransport.conn == nil ||
		t.iceTransport.mux == nil {
		return errors.New("ICE connection not started")
	}

	return nil
}
