// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js

package webrtc

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"regexp"
	"testing"
	"time"

	"github.com/pion/dtls/v4"
	dtlsCipherSuite "github.com/pion/dtls/v4/pkg/crypto/ciphersuite"
	dtlsElliptic "github.com/pion/dtls/v4/pkg/crypto/elliptic"
	"github.com/pion/dtls/v4/pkg/protocol"
	"github.com/pion/dtls/v4/pkg/protocol/handshake"
	"github.com/pion/srtp/v3"
	"github.com/pion/transport/v5/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// An invalid fingerprint MUST cause DTLSTransport to go to failed state.
func TestInvalidFingerprintCausesFailed(t *testing.T) {
	t.Run("Initial", func(t *testing.T) { testInvalidFingerprintCausesFailed(t, false) })
	t.Run("Restart", func(t *testing.T) { testInvalidFingerprintCausesFailed(t, true) })
}

func testInvalidFingerprintCausesFailed(t *testing.T, restart bool) { //nolint:cyclop
	t.Helper()
	lim := test.TimeOut(time.Second * 10)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	pcOffer, err := NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	pcAnswer, err := NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	defer closePairNow(t, pcOffer, pcAnswer)

	_, err = pcOffer.CreateDataChannel("unusedDataChannel", nil)
	require.NoError(t, err)
	if restart {
		connected := untilConnectionState(PeerConnectionStateConnected, pcOffer, pcAnswer)
		require.NoError(t, signalPairWithOptions(pcOffer, pcAnswer, withDisableInitialDataChannel(true)))
		<-connected
		pcOffer.ops.Done()
		pcAnswer.ops.Done()
	} else {
		pcAnswer.OnDataChannel(func(_ *DataChannel) {
			assert.Fail(t, "A DataChannel must not be created when Fingerprint verification fails")
		})
	}

	// Set up DTLS state tracking BEFORE starting the connection process
	// to avoid missing the state transition
	offerDTLSFailed := make(chan struct{})
	answerDTLSFailed := make(chan struct{})
	pcOffer.SCTP().Transport().OnStateChange(func(state DTLSTransportState) {
		if state == DTLSTransportStateFailed {
			select {
			case <-offerDTLSFailed:
				// Already closed
			default:
				close(offerDTLSFailed)
			}
		}
	})
	pcAnswer.SCTP().Transport().OnStateChange(func(state DTLSTransportState) {
		if state == DTLSTransportStateFailed {
			select {
			case <-answerDTLSFailed:
				// Already closed
			default:
				close(answerDTLSFailed)
			}
		}
	})

	peerConnectionsFailed := untilConnectionState(PeerConnectionStateFailed, pcOffer, pcAnswer)

	offer, err := pcOffer.CreateOffer(&OfferOptions{DTLSRestart: restart})
	require.NoError(t, err)
	offerGathered := GatheringCompletePromise(pcOffer)
	require.NoError(t, pcOffer.SetLocalDescription(offer))

	select {
	case <-offerGathered:
		offer = *pcOffer.LocalDescription()
		// Replace with invalid fingerprint
		re := regexp.MustCompile(`sha-256 (.*?)\r`)
		offer.SDP = re.ReplaceAllString(
			offer.SDP,
			"sha-256 AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA\r",
		)

		assert.NoError(t, pcAnswer.SetRemoteDescription(offer))

		answer, err := pcAnswer.CreateAnswer(nil)
		assert.NoError(t, err)
		assert.NoError(t, pcAnswer.SetLocalDescription(answer))

		answer.SDP = re.ReplaceAllString(
			answer.SDP,
			"sha-256 AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA:AA\r",
		)

		assert.NoError(t, pcOffer.SetRemoteDescription(answer))
	case <-time.After(5 * time.Second):
		assert.Fail(t, "timed out waiting to receive offer")
	}

	// Wait for DTLS to fail (should happen quickly after ICE connects, ~1-2 seconds normally,
	// but may take longer with race detector due to ICE connectivity checks)
	select {
	case <-offerDTLSFailed:
		// Expected - offer DTLS failed due to invalid fingerprint
	case <-time.After(7 * time.Second):
		assert.Fail(t, "timed out waiting for offer DTLS to fail")
	}

	select {
	case <-answerDTLSFailed:
		// Expected - answer DTLS failed due to invalid fingerprint
	case <-time.After(7 * time.Second):
		assert.Fail(t, "timed out waiting for answer DTLS to fail")
	}

	<-peerConnectionsFailed

	assert.Equal(t, DTLSTransportStateFailed, pcOffer.SCTP().Transport().State())
	assert.Nil(t, pcOffer.SCTP().Transport().conn)
	assert.False(t, pcOffer.isClosed.Load())

	assert.Equal(t, DTLSTransportStateFailed, pcAnswer.SCTP().Transport().State())
	assert.Nil(t, pcAnswer.SCTP().Transport().conn)
	assert.False(t, pcAnswer.isClosed.Load())
}

func TestDTLSFailureAfterConnected(t *testing.T) {
	defer test.TimeOut(10 * time.Second).Stop()
	defer test.CheckRoutines(t)()
	offer, answer, err := newPair()
	require.NoError(t, err)
	defer closePairNow(t, offer, answer)
	_, err = offer.AddTransceiverFromKind(RTPCodecTypeVideo)
	require.NoError(t, err)
	connected := untilConnectionState(PeerConnectionStateConnected, offer, answer)
	require.NoError(t, signalPairWithOptions(offer, answer, withDisableInitialDataChannel(true)))
	<-connected

	failed := untilConnectionState(PeerConnectionStateFailed, offer)
	dtlsFailed := make(chan struct{})
	transport := offer.dtlsTransport
	transport.OnStateChange(func(state DTLSTransportState) {
		if state == DTLSTransportStateFailed {
			close(dtlsFailed)
		}
	})
	iceTransport := transport.ICETransport()
	iceTransport.lock.Lock()
	conn := iceTransport.conn
	iceTransport.conn = &errConn{writeErr: errTestWriteFailed}
	iceTransport.lock.Unlock()
	defer func() {
		iceTransport.lock.Lock()
		iceTransport.conn = conn
		iceTransport.lock.Unlock()
	}()
	_, err = transport.conn.Write([]byte{1})
	require.NoError(t, err)
	<-failed
	<-dtlsFailed
	assert.Equal(t, DTLSTransportStateFailed, transport.State())
	assert.Equal(t, ICEConnectionStateConnected, offer.ICEConnectionState())
	assert.False(t, offer.isClosed.Load())
}

// DTLS closure updates the transport without closing the PeerConnection.
func TestDTLSClose(t *testing.T) {
	for _, closePeerConnection := range []bool{false, true} {
		name := "DTLSTransport"
		if closePeerConnection {
			name = "PeerConnection"
		}
		t.Run(name, func(t *testing.T) {
			defer test.TimeOut(10 * time.Second).Stop()
			defer test.CheckRoutines(t)()

			offer, answer, err := newPair()
			assert.NoError(t, err)
			defer closePairNow(t, offer, answer)

			_, err = offer.AddTransceiverFromKind(RTPCodecTypeVideo)
			assert.NoError(t, err)
			connected := untilConnectionState(PeerConnectionStateConnected, offer, answer)
			assert.NoError(t, signalPair(offer, answer))
			<-connected

			transport := answer.SCTP().Transport()
			closed := make(chan struct{})
			transport.OnStateChange(func(state DTLSTransportState) {
				if state == DTLSTransportStateClosed {
					close(closed)
				}
			})
			defer transport.OnStateChange(nil)

			if closePeerConnection {
				assert.NoError(t, offer.Close())
			} else {
				assert.NoError(t, offer.SCTP().Transport().Stop())
				assert.Equal(t, PeerConnectionStateConnected, offer.ConnectionState())
			}
			<-closed

			assert.Equal(t, DTLSTransportStateClosed, transport.State())
			assert.Equal(t, ICEConnectionStateConnected, answer.ICEConnectionState())
			assert.Equal(t, PeerConnectionStateConnected, answer.ConnectionState())
			assert.Equal(t, SignalingStateStable, answer.SignalingState())
			assert.False(t, answer.isClosed.Load())
			_, err = answer.CreateOffer(nil)
			assert.NoError(t, err)
		})
	}
}

func TestPeerConnection_DTLSRoleSettingEngine(t *testing.T) {
	runTest := func(r DTLSRole) {
		s := SettingEngine{}
		assert.NoError(t, s.SetAnsweringDTLSRole(r))

		offerPC, err := NewAPI(WithSettingEngine(s)).NewPeerConnection(Configuration{})
		assert.NoError(t, err)

		answerPC, err := NewAPI(WithSettingEngine(s)).NewPeerConnection(Configuration{})
		assert.NoError(t, err)
		assert.NoError(t, signalPair(offerPC, answerPC))

		connectionComplete := untilConnectionState(PeerConnectionStateConnected, answerPC)
		<-connectionComplete
		closePairNow(t, offerPC, answerPC)
	}

	report := test.CheckRoutines(t)
	defer report()

	t.Run("Server", func(*testing.T) {
		runTest(DTLSRoleServer)
	})

	t.Run("Client", func(*testing.T) {
		runTest(DTLSRoleClient)
	})
}

func TestPeerConnection_DTLSVersion(t *testing.T) {
	for _, tc := range []struct {
		name                                     string
		offerMin, offerMax, answerMin, answerMax protocol.Version
		want                                     protocol.Version
	}{
		{name: "default", want: protocol.Version1_3},
		{name: "DTLS1.2", offerMax: protocol.Version1_2, answerMax: protocol.Version1_2, want: protocol.Version1_2},
		{name: "DTLS1.3", offerMin: protocol.Version1_3, answerMin: protocol.Version1_3, want: protocol.Version1_3},
		{name: "offer limited to DTLS1.2", offerMax: protocol.Version1_2, want: protocol.Version1_2},
		{name: "answer limited to DTLS1.2", answerMax: protocol.Version1_2, want: protocol.Version1_2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			defer test.TimeOut(10 * time.Second).Stop()
			defer test.CheckRoutines(t)()

			var offerSettings, answerSettings SettingEngine
			offerSettings.SetDTLSMinVersion(tc.offerMin)
			offerSettings.SetDTLSMaxVersion(tc.offerMax)
			answerSettings.SetDTLSMinVersion(tc.answerMin)
			answerSettings.SetDTLSMaxVersion(tc.answerMax)
			offer, err := NewAPI(WithSettingEngine(offerSettings)).NewPeerConnection(Configuration{})
			require.NoError(t, err)
			defer func() { require.NoError(t, offer.Close()) }()
			answer, err := NewAPI(WithSettingEngine(answerSettings)).NewPeerConnection(Configuration{})
			require.NoError(t, err)
			defer func() { require.NoError(t, answer.Close()) }()

			for _, peer := range []*PeerConnection{offer, answer} {
				stats := getTransportStats(t, peer.GetStats(), "iceTransport")
				encoded, marshalErr := json.Marshal(stats)
				require.NoError(t, marshalErr)
				var fields map[string]json.RawMessage
				require.NoError(t, json.Unmarshal(encoded, &fields))
				for _, name := range []string{"tlsVersion", "dtlsCipher", "srtpCipher"} {
					assert.NotContains(t, fields, name)
				}
			}

			connected := untilConnectionState(PeerConnectionStateConnected, offer, answer)
			require.NoError(t, signalPair(offer, answer))
			<-connected
			for _, peer := range []*PeerConnection{offer, answer} {
				state, ok := peer.dtlsTransport.dtlsConn.ConnectionState()
				require.True(t, ok)
				require.Equal(t, tc.want, state.NegotiatedVersion())
				stats := getTransportStats(t, peer.GetStats(), "iceTransport")
				assert.Equal(t, fmt.Sprintf("%04X", tc.want), stats.TLSVersion)
				assert.Equal(t, state.CipherSuiteID.String(), stats.DTLSCipher)
				assert.NotEmpty(t, stats.DTLSCipher)
				assert.Equal(t, "SRTP_AEAD_AES_256_GCM", stats.SRTPCipher)
				require.NoError(t, peer.Close())
				closedStats := getTransportStats(t, peer.GetStats(), "iceTransport")
				assert.Equal(t, stats.TLSVersion, closedStats.TLSVersion)
				assert.Equal(t, stats.DTLSCipher, closedStats.DTLSCipher)
				assert.Equal(t, stats.SRTPCipher, closedStats.SRTPCipher)
			}
		})
	}
}

func TestDTLSTransport_InvalidVersionRange(t *testing.T) {
	for _, tc := range []struct {
		name     string
		min, max protocol.Version
	}{
		{name: "unsupported minimum", min: protocol.Version1_0},
		{name: "unsupported maximum", max: protocol.Version1_0},
		{name: "minimum exceeds maximum", min: protocol.Version1_3, max: protocol.Version1_2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var settings SettingEngine
			settings.SetDTLSMinVersion(tc.min)
			settings.SetDTLSMaxVersion(tc.max)
			api := NewAPI(WithSettingEngine(settings))
			transport, err := api.NewDTLSTransport(nil, nil)
			require.NoError(t, err)
			cert := transport.certificates[0]
			opts := transport.dtlsSharedOptions(tls.Certificate{
				Certificate: [][]byte{cert.x509Cert.Raw}, PrivateKey: cert.privateKey,
			})
			addr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 4444}
			_, err = dtls.DetachedClient(addr, transport.toDTLSClientOptions(opts)...)
			require.Error(t, err)
			_, err = dtls.DetachedServer(addr, transport.toDTLSServerOptions(opts)...)
			require.Error(t, err)
		})
	}
}

type errConn struct {
	localAddr  net.Addr
	remoteAddr net.Addr
	readErr    error
	writeErr   error
}

func (c *errConn) Read([]byte) (int, error)         { return 0, c.readErr }
func (c *errConn) Write([]byte) (int, error)        { return 0, c.writeErr }
func (c *errConn) Close() error                     { return nil }
func (c *errConn) LocalAddr() net.Addr              { return c.localAddr }
func (c *errConn) RemoteAddr() net.Addr             { return c.remoteAddr }
func (c *errConn) SetDeadline(time.Time) error      { return nil }
func (c *errConn) SetReadDeadline(time.Time) error  { return nil }
func (c *errConn) SetWriteDeadline(time.Time) error { return nil }

var errTestWriteFailed = errors.New("write failed")

func TestDTLSTransport_Start_ErrICEConnectionNotStarted(t *testing.T) {
	api := NewAPI()
	connectContextMakerCalled := false
	api.settingEngine.dtls.connectContextMaker = func() (context.Context, func()) {
		connectContextMakerCalled = true

		return context.Background(), nil
	}

	transport := &DTLSTransport{api: api, state: DTLSTransportStateNew}

	err := transport.Start(DTLSParameters{Role: DTLSRoleServer})
	assert.ErrorIs(t, err, errICEConnectionNotStarted)
	assert.Equal(t, DTLSTransportStateNew, transport.State())
	assert.False(t, connectContextMakerCalled)
}

func TestDTLSTransport_Start_UsesConnectContextMaker(t *testing.T) {
	lim := test.TimeOut(time.Second)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	api := NewAPI()
	loggerFactory := api.settingEngine.LoggerFactory

	connectContextMakerCalled := false
	cancelCalled := false
	api.settingEngine.dtls.connectContextMaker = func() (context.Context, func()) {
		connectContextMakerCalled = true

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		return ctx, func() {
			cancelCalled = true
		}
	}

	conn := &errConn{
		localAddr:  &net.UDPAddr{IP: net.IPv4zero, Port: 1},
		remoteAddr: &net.UDPAddr{IP: net.IPv4zero, Port: 2},
		readErr:    io.EOF,
		writeErr:   errTestWriteFailed,
	}

	iceTransport := NewICETransport(nil, loggerFactory)
	iceTransport.conn = conn
	defer func() { _ = conn.Close() }()

	transport, err := api.NewDTLSTransport(iceTransport, nil)
	assert.NoError(t, err)
	transport.OnStateChange(func(state DTLSTransportState) {
		if state == DTLSTransportStateConnecting {
			assert.False(t, connectContextMakerCalled)
		}
	})

	err = transport.Start(DTLSParameters{Role: DTLSRoleServer})
	assert.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, DTLSTransportStateFailed, transport.State())
	assert.True(t, connectContextMakerCalled)
	assert.True(t, cancelCalled)
}

func TestDTLSTransport_Start_ConnectErrorFailsTransport(t *testing.T) {
	lim := test.TimeOut(time.Second)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	api := NewAPI()
	loggerFactory := api.settingEngine.LoggerFactory

	localConn, remoteConn := net.Pipe()
	defer func() { _ = remoteConn.Close() }()

	iceTransport := NewICETransport(nil, loggerFactory)
	iceTransport.conn = localConn
	defer func() { _ = localConn.Close() }()

	transport, err := api.NewDTLSTransport(iceTransport, nil)
	assert.NoError(t, err)
	assert.Equal(t, DTLSTransportStateNew, transport.State())

	transport.api.settingEngine.dtls.cipherSuites = []dtlsCipherSuite.ID{}

	err = transport.Start(DTLSParameters{Role: DTLSRoleServer})
	assert.Error(t, err)
	assert.Equal(t, DTLSTransportStateFailed, transport.State())
	assert.Nil(t, transport.conn)

	assert.NotNil(t, iceTransport.endpoints[iceEndpointSRTP])
	assert.NotNil(t, iceTransport.endpoints[iceEndpointSRTCP])
}

func TestDTLSTransport_Start_HandshakeErrorFailsTransport(t *testing.T) {
	lim := test.TimeOut(time.Second)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	api := NewAPI()
	loggerFactory := api.settingEngine.LoggerFactory

	conn := &errConn{
		localAddr:  &net.UDPAddr{IP: net.IPv4zero, Port: 1},
		remoteAddr: &net.UDPAddr{IP: net.IPv4zero, Port: 2},
		readErr:    io.EOF,
		writeErr:   errTestWriteFailed,
	}

	iceTransport := NewICETransport(nil, loggerFactory)
	iceTransport.conn = conn
	defer func() { _ = conn.Close() }()

	transport, err := api.NewDTLSTransport(iceTransport, nil)
	assert.NoError(t, err)
	assert.Equal(t, DTLSTransportStateNew, transport.State())

	err = transport.Start(DTLSParameters{Role: DTLSRoleServer})
	assert.Error(t, err)
	assert.Equal(t, DTLSTransportStateFailed, transport.State())
	assert.Nil(t, transport.conn)

	assert.NotNil(t, iceTransport.endpoints[iceEndpointSRTP])
	assert.NotNil(t, iceTransport.endpoints[iceEndpointSRTCP])
}

func TestDTLSTransport_dtlsSharedOptions_IncludesOptionalOptions(t *testing.T) {
	baseAPI := NewAPI()
	baseTransport := &DTLSTransport{api: baseAPI}
	baseCount := len(baseTransport.dtlsSharedOptions(tls.Certificate{}))

	tests := []struct {
		name      string
		configure func(*SettingEngine)
		wantExtra int
	}{
		{
			name: "CustomCipherSuites",
			configure: func(se *SettingEngine) {
				se.dtls.customCipherSuites = func() []dtlsCipherSuite.Suite {
					return nil
				}
			},
			wantExtra: 1,
		},
		{
			name: "FlightInterval",
			configure: func(se *SettingEngine) {
				se.dtls.retransmissionInterval = time.Second
			},
			wantExtra: 1,
		},
		{
			name: "ReplayProtectionWindow",
			configure: func(se *SettingEngine) {
				window := uint(1)
				se.replayProtection.DTLS = &window
			},
			wantExtra: 1,
		},
		{
			name: "CipherSuites",
			configure: func(se *SettingEngine) {
				se.dtls.cipherSuites = []dtlsCipherSuite.ID{
					dtlsCipherSuite.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
				}
			},
			wantExtra: 1,
		},
		{
			name: "EllipticCurves",
			configure: func(se *SettingEngine) {
				se.dtls.ellipticCurves = []dtlsElliptic.Curve{dtlsElliptic.P256}
			},
			wantExtra: 1,
		},
		{
			name: "RootCAs",
			configure: func(se *SettingEngine) {
				se.dtls.rootCAs = x509.NewCertPool()
			},
			wantExtra: 1,
		},
		{
			name: "KeyLogWriter",
			configure: func(se *SettingEngine) {
				se.dtls.keyLogWriter = &bytes.Buffer{}
			},
			wantExtra: 1,
		},
		{
			name: "AllOptional",
			configure: func(se *SettingEngine) {
				se.dtls.customCipherSuites = func() []dtlsCipherSuite.Suite {
					return nil
				}
				se.dtls.retransmissionInterval = time.Second

				window := uint(1)
				se.replayProtection.DTLS = &window

				se.dtls.cipherSuites = []dtlsCipherSuite.ID{
					dtlsCipherSuite.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
				}
				se.dtls.ellipticCurves = []dtlsElliptic.Curve{dtlsElliptic.P256}
				se.dtls.rootCAs = x509.NewCertPool()
				se.dtls.keyLogWriter = &bytes.Buffer{}
			},
			wantExtra: 7,
		},
		{
			name: "SupportedProtocols",
			configure: func(se *SettingEngine) {
				se.dtls.supportedProtocols = []string{"webrtc", "c-webrtc"}
			},
			wantExtra: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			api := NewAPI()
			tc.configure(api.settingEngine)

			transport := &DTLSTransport{api: api}
			opts := transport.dtlsSharedOptions(tls.Certificate{})
			assert.Len(t, opts, baseCount+tc.wantExtra)
		})
	}
}

func TestDTLSTransport_toDTLSClientOptions_IncludesOptionalOptions(t *testing.T) {
	baseAPI := NewAPI()
	baseTransport := &DTLSTransport{api: baseAPI}
	baseSharedOpts := baseTransport.dtlsSharedOptions(tls.Certificate{})
	baseCount := len(baseTransport.toDTLSClientOptions(baseSharedOpts))

	api := NewAPI()
	api.settingEngine.dtls.clientHelloMessageHook = func(m handshake.MessageClientHello) handshake.Message {
		return &m
	}
	transport := &DTLSTransport{api: api}
	sharedOpts := transport.dtlsSharedOptions(tls.Certificate{})
	opts := transport.toDTLSClientOptions(sharedOpts)

	assert.Len(t, opts, baseCount+1)
}

func TestDTLSTransport_verifyPeerCertificateFunc_NoRemoteCertificate(t *testing.T) {
	api := NewAPI()
	transport := &DTLSTransport{api: api}

	err := transport.verifyPeerCertificateFunc()(nil, nil)
	assert.ErrorIs(t, err, errNoRemoteCertificate)
	assert.Nil(t, transport.GetRemoteCertificate())
}

func TestDTLSTransport_verifyPeerCertificateFunc_ParseError(t *testing.T) {
	api := NewAPI()
	transport := &DTLSTransport{api: api}

	rawCert := []byte("not a certificate")
	err := transport.verifyPeerCertificateFunc()([][]byte{rawCert}, nil)
	assert.Error(t, err)
	assert.Equal(t, rawCert, transport.GetRemoteCertificate())
}

func TestDTLSTransport_toDTLSServerOptions_IncludesOptionalOptions(t *testing.T) {
	baseAPI := NewAPI()
	baseTransport := &DTLSTransport{api: baseAPI}
	baseCount := len(baseTransport.toDTLSServerOptions(nil))

	tests := []struct {
		name      string
		configure func(*SettingEngine)
		wantExtra int
	}{
		{
			name: "ServerHelloMessageHook",
			configure: func(se *SettingEngine) {
				se.dtls.serverHelloMessageHook = func(m handshake.MessageServerHello) handshake.Message {
					return &m
				}
			},
			wantExtra: 1,
		},
		{
			name: "CertificateRequestMessageHook",
			configure: func(se *SettingEngine) {
				se.dtls.certificateRequestMessageHook = func(m handshake.MessageCertificateRequest) handshake.Message {
					return &m
				}
			},
			wantExtra: 1,
		},
		{
			name: "AllOptional",
			configure: func(se *SettingEngine) {
				se.dtls.serverHelloMessageHook = func(m handshake.MessageServerHello) handshake.Message {
					return &m
				}
				se.dtls.certificateRequestMessageHook = func(m handshake.MessageCertificateRequest) handshake.Message {
					return &m
				}
			},
			wantExtra: 2,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			api := NewAPI()
			tc.configure(api.settingEngine)

			transport := &DTLSTransport{api: api}
			opts := transport.toDTLSServerOptions(nil)
			assert.Len(t, opts, baseCount+tc.wantExtra)
		})
	}
}

func TestSRTPProtectionProfileFromDTLS(t *testing.T) {
	tests := []struct {
		name    string
		profile dtls.SRTPProtectionProfile
		want    srtp.ProtectionProfile
		wantErr error
	}{
		{
			name:    "SRTP_AEAD_AES_128_GCM",
			profile: dtls.SRTP_AEAD_AES_128_GCM,
			want:    srtp.ProtectionProfileAeadAes128Gcm,
		},
		{
			name:    "SRTP_AEAD_AES_256_GCM",
			profile: dtls.SRTP_AEAD_AES_256_GCM,
			want:    srtp.ProtectionProfileAeadAes256Gcm,
		},
		{
			name:    "SRTP_AES128_CM_HMAC_SHA1_80",
			profile: dtls.SRTP_AES128_CM_HMAC_SHA1_80,
			want:    srtp.ProtectionProfileAes128CmHmacSha1_80,
		},
		{
			name:    "SRTP_NULL_HMAC_SHA1_80",
			profile: dtls.SRTP_NULL_HMAC_SHA1_80,
			want:    srtp.ProtectionProfileNullHmacSha1_80,
		},
		{
			name:    "Unknown",
			profile: dtls.SRTPProtectionProfile(255),
			wantErr: ErrNoSRTPProtectionProfile,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := srtpProtectionProfileFromDTLS(tc.profile)
			if tc.wantErr != nil {
				assert.ErrorIs(t, err, tc.wantErr)

				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestDTLSTransport_StartContextInterruptedWhileWaitingForICE(t *testing.T) {
	for _, action := range []string{"cancel", "stop DTLS", "stop ICE"} {
		t.Run(action, func(t *testing.T) {
			defer test.TimeOut(5 * time.Second).Stop()
			defer test.CheckRoutines(t)()

			api := NewAPI()
			iceTransport := api.NewICETransport(nil)
			transport, err := api.NewDTLSTransport(iceTransport, nil)
			require.NoError(t, err)
			defer func() { require.NoError(t, transport.Stop()) }()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			connecting := make(chan struct{})
			transport.OnStateChange(func(state DTLSTransportState) {
				if state == DTLSTransportStateConnecting {
					close(connecting)
				}
			})
			result := make(chan error, 1)
			go func() {
				result <- transport.StartContext(ctx, DTLSParameters{Role: DTLSRoleServer})
			}()
			<-connecting

			switch action {
			case "cancel":
				cancel()
				require.ErrorIs(t, <-result, context.Canceled)
				require.Equal(t, DTLSTransportStateFailed, transport.State())
			case "stop DTLS":
				require.NoError(t, transport.Stop())
				require.ErrorIs(t, <-result, io.ErrClosedPipe)
				require.Equal(t, DTLSTransportStateClosed, transport.State())
			case "stop ICE":
				require.NoError(t, iceTransport.Stop())
				require.ErrorIs(t, <-result, io.ErrClosedPipe)
				require.Equal(t, DTLSTransportStateFailed, transport.State())
			}
		})
	}
}
