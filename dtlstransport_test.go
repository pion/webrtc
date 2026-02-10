// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js
// +build !js

package webrtc

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"net"
	"reflect"
	"regexp"
	"testing"
	"time"

	"github.com/pion/dtls/v3"
	dtlsElliptic "github.com/pion/dtls/v3/pkg/crypto/elliptic"
	"github.com/pion/dtls/v3/pkg/protocol/handshake"
	"github.com/pion/srtp/v3"
	"github.com/pion/transport/v4/test"
	"github.com/pion/webrtc/v4/internal/mux"
	"github.com/stretchr/testify/assert"
)

// An invalid fingerprint MUST cause DTLSTransport to go to failed state.
func TestInvalidFingerprintCausesFailed(t *testing.T) { //nolint:cyclop
	lim := test.TimeOut(time.Second * 10)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	pcOffer, err := NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	pcAnswer, err := NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	pcAnswer.OnDataChannel(func(_ *DataChannel) {
		assert.Fail(t, "A DataChannel must not be created when Fingerprint verification fails")
	})

	defer closePairNow(t, pcOffer, pcAnswer)

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

	offerChan := make(chan SessionDescription)
	pcOffer.OnICECandidate(func(candidate *ICECandidate) {
		if candidate == nil {
			offerChan <- *pcOffer.PendingLocalDescription()
		}
	})

	// Also wait for PeerConnection to close (may take longer due to cleanup)
	offerConnectionHasClosed := untilConnectionState(PeerConnectionStateClosed, pcOffer)
	answerConnectionHasClosed := untilConnectionState(PeerConnectionStateClosed, pcAnswer)

	_, err = pcOffer.CreateDataChannel("unusedDataChannel", nil)
	assert.NoError(t, err)

	offer, err := pcOffer.CreateOffer(nil)
	assert.NoError(t, err)
	assert.NoError(t, pcOffer.SetLocalDescription(offer))

	select {
	case offer := <-offerChan:
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

	// Wait for PeerConnection to close (may take longer due to cleanup)
	offerConnectionHasClosed.Wait()
	answerConnectionHasClosed.Wait()

	assert.Contains(
		t, []DTLSTransportState{DTLSTransportStateClosed, DTLSTransportStateFailed}, pcOffer.SCTP().Transport().State(),
		"DTLS Transport should be closed or failed",
	)
	assert.Nil(t, pcOffer.SCTP().Transport().conn)

	assert.Contains(
		t, []DTLSTransportState{DTLSTransportStateClosed, DTLSTransportStateFailed}, pcAnswer.SCTP().Transport().State(),
		"DTLS Transport should be closed or failed",
	)
	assert.Nil(t, pcAnswer.SCTP().Transport().conn)
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
		connectionComplete.Wait()
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

type failingPacketConn struct {
	localAddr net.Addr
	readErr   error
	writeErr  error
}

var errTestWriteFailed = errors.New("write failed")

func (c *failingPacketConn) ReadFrom([]byte) (int, net.Addr, error) {
	return 0, c.localAddr, c.readErr
}

func (c *failingPacketConn) WriteTo([]byte, net.Addr) (int, error) {
	return 0, c.writeErr
}

func (c *failingPacketConn) Close() error                     { return nil }
func (c *failingPacketConn) LocalAddr() net.Addr              { return c.localAddr }
func (c *failingPacketConn) SetDeadline(time.Time) error      { return nil }
func (c *failingPacketConn) SetReadDeadline(time.Time) error  { return nil }
func (c *failingPacketConn) SetWriteDeadline(time.Time) error { return nil }

func TestDTLSTransport_Start_ErrICEConnectionNotStarted(t *testing.T) {
	transport := &DTLSTransport{state: DTLSTransportStateNew}

	err := transport.Start(DTLSParameters{Role: DTLSRoleServer})
	assert.ErrorIs(t, err, errICEConnectionNotStarted)
	assert.Equal(t, DTLSTransportStateNew, transport.State())
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
	iceTransport.mux = mux.NewMux(mux.Config{
		Conn:          localConn,
		BufferSize:    1500,
		LoggerFactory: loggerFactory,
	})
	defer func() { _ = iceTransport.mux.Close() }()

	transport, err := api.NewDTLSTransport(iceTransport, nil)
	assert.NoError(t, err)
	assert.Equal(t, DTLSTransportStateNew, transport.State())

	transport.api.settingEngine.dtls.cipherSuites = []dtls.CipherSuiteID{}

	err = transport.Start(DTLSParameters{Role: DTLSRoleServer})
	assert.Error(t, err)
	assert.Equal(t, DTLSTransportStateFailed, transport.State())
	assert.Nil(t, transport.conn)

	assert.Equal(t, 2, reflect.ValueOf(iceTransport.mux).Elem().FieldByName("endpoints").Len())
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
	iceTransport.mux = mux.NewMux(mux.Config{
		Conn:          conn,
		BufferSize:    1500,
		LoggerFactory: loggerFactory,
	})
	defer func() { _ = iceTransport.mux.Close() }()

	transport, err := api.NewDTLSTransport(iceTransport, nil)
	assert.NoError(t, err)
	assert.Equal(t, DTLSTransportStateNew, transport.State())

	err = transport.Start(DTLSParameters{Role: DTLSRoleServer})
	assert.Error(t, err)
	assert.Equal(t, DTLSTransportStateFailed, transport.State())
	assert.Nil(t, transport.conn)

	assert.Equal(t, 2, reflect.ValueOf(iceTransport.mux).Elem().FieldByName("endpoints").Len())
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
				se.dtls.customCipherSuites = func() []dtls.CipherSuite {
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
				se.dtls.cipherSuites = []dtls.CipherSuiteID{
					dtls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
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
				se.dtls.customCipherSuites = func() []dtls.CipherSuite {
					return nil
				}
				se.dtls.retransmissionInterval = time.Second

				window := uint(1)
				se.replayProtection.DTLS = &window

				se.dtls.cipherSuites = []dtls.CipherSuiteID{
					dtls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
				}
				se.dtls.ellipticCurves = []dtlsElliptic.Curve{dtlsElliptic.P256}
				se.dtls.rootCAs = x509.NewCertPool()
				se.dtls.keyLogWriter = &bytes.Buffer{}
			},
			wantExtra: 7,
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

func TestDTLSTransport_handshakeDTLS_DeferredCancel(t *testing.T) {
	lim := test.TimeOut(time.Second)
	defer lim.Stop()

	api := NewAPI()
	transport := &DTLSTransport{api: api}

	connectContextMakerCalled := false
	cancelCalled := false
	api.settingEngine.dtls.connectContextMaker = func() (context.Context, func()) {
		connectContextMakerCalled = true

		ctx, cancel := context.WithCancel(context.Background())

		return ctx, func() {
			cancelCalled = true
			cancel()
		}
	}

	packetConn := &failingPacketConn{
		localAddr: &net.UDPAddr{IP: net.IPv4zero, Port: 1},
		readErr:   io.EOF,
		writeErr:  errTestWriteFailed,
	}

	dtlsConn, err := dtls.ClientWithOptions(packetConn, &net.UDPAddr{IP: net.IPv4zero, Port: 2})
	assert.NoError(t, err)
	defer func() { _ = dtlsConn.Close() }()

	err = transport.handshakeDTLS(dtlsConn)
	assert.Error(t, err)
	assert.True(t, connectContextMakerCalled)
	assert.True(t, cancelCalled)
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
