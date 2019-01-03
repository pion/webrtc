package network

import (
	"context"
	"crypto"
	"crypto/x509"
	"fmt"
	"strings"
	"sync"

	"github.com/pions/datachannel"
	"github.com/pions/dtls/pkg/dtls"
	"github.com/pions/sctp"
	"github.com/pions/webrtc/internal/mux"
	"github.com/pions/webrtc/internal/srtp"
	"github.com/pions/webrtc/pkg/ice"
)

const (
	srtpMasterKeyLen     = 16
	srtpMasterKeySaltLen = 14
	receiveMTU           = 8192
)

// Manager contains all network state (DTLS, SRTP) that is shared between ports
// It is also used to perform operations that involve multiple ports
type Manager struct {
	IceAgent *ice.Agent
	iceConn  *ice.Conn
	isOffer  bool

	SrtpSession  *srtp.SessionSRTP
	SrtcpSession *srtp.SessionSRTCP

	mux *mux.Mux

	dtlsEndpoint  *mux.Endpoint
	srtpEndpoint  *mux.Endpoint
	srtcpEndpoint *mux.Endpoint

	dtlsConn *dtls.Conn

	sctpAssociationMutex sync.RWMutex
	sctpAssociation      *sctp.Association
}

// NewManager creates a new network.Manager
func NewManager(urls []*ice.URL, ntf ICENotifier, minport, maxport uint16) (*Manager, error) {
	config := &ice.AgentConfig{Urls: urls, Notifier: ntf, PortMin: minport, PortMax: maxport}
	iceAgent, err := ice.NewAgent(config)

	if err != nil {
		return nil, err
	}

	return &Manager{
		IceAgent:     iceAgent,
		SrtpSession:  srtp.CreateSessionSRTP(),
		SrtcpSession: srtp.CreateSessionSRTCP(),
	}, nil
}

// Start allocates the network stack
// TODO: Turn into the ORTC constructors
func (m *Manager) Start(isOffer bool,
	remoteUfrag, remotePwd string,
	dtlsCert *x509.Certificate, dtlsPrivKey crypto.PrivateKey, fingerprint, fingerprintHash string) error {

	m.isOffer = isOffer

	if err := m.startICE(isOffer, remoteUfrag, remotePwd); err != nil {
		return err
	}

	m.mux = mux.NewMux(m.iceConn, receiveMTU)
	m.dtlsEndpoint = m.mux.NewEndpoint(mux.MatchDTLS)
	m.srtpEndpoint = m.mux.NewEndpoint(mux.MatchSRTP)
	m.srtcpEndpoint = m.mux.NewEndpoint(mux.MatchSRTCP)

	if err := m.startDTLS(isOffer, dtlsCert, dtlsPrivKey, fingerprint, fingerprintHash); err != nil {
		return err
	}

	return m.startSRTP(isOffer)
}

func (m *Manager) startICE(isOffer bool, remoteUfrag, remotePwd string) error {
	if isOffer {
		iceConn, err := m.IceAgent.Dial(context.TODO(), remoteUfrag, remotePwd)
		if err != nil {
			return err
		}
		m.iceConn = iceConn
	} else {
		iceConn, err := m.IceAgent.Accept(context.TODO(), remoteUfrag, remotePwd)
		if err != nil {
			return err
		}
		m.iceConn = iceConn
	}
	return nil
}

func (m *Manager) startSRTP(isOffer bool) error {
	keyingMaterial, err := m.dtlsConn.ExportKeyingMaterial([]byte("EXTRACTOR-dtls_srtp"), nil, (srtpMasterKeyLen*2)+(srtpMasterKeySaltLen*2))
	if err != nil {
		return err
	}

	offset := 0
	clientWriteKey := append([]byte{}, keyingMaterial[offset:offset+srtpMasterKeyLen]...)
	offset += srtpMasterKeyLen

	serverWriteKey := append([]byte{}, keyingMaterial[offset:offset+srtpMasterKeyLen]...)
	offset += srtpMasterKeyLen

	clientWriteKey = append(clientWriteKey, keyingMaterial[offset:offset+srtpMasterKeySaltLen]...)
	offset += srtpMasterKeySaltLen

	serverWriteKey = append(serverWriteKey, keyingMaterial[offset:offset+srtpMasterKeySaltLen]...)

	if isOffer {
		err = m.SrtpSession.Start(
			serverWriteKey[0:16], serverWriteKey[16:],
			clientWriteKey[0:16], clientWriteKey[16:],
			srtp.ProtectionProfileAes128CmHmacSha1_80, m.srtpEndpoint,
		)

		if err == nil {
			err = m.SrtcpSession.Start(
				serverWriteKey[0:16], serverWriteKey[16:],
				clientWriteKey[0:16], clientWriteKey[16:],
				srtp.ProtectionProfileAes128CmHmacSha1_80, m.srtcpEndpoint,
			)
		}
	} else {
		err = m.SrtpSession.Start(
			clientWriteKey[0:16], clientWriteKey[16:],
			serverWriteKey[0:16], serverWriteKey[16:],
			srtp.ProtectionProfileAes128CmHmacSha1_80, m.srtpEndpoint,
		)

		if err == nil {
			err = m.SrtcpSession.Start(
				clientWriteKey[0:16], clientWriteKey[16:],
				serverWriteKey[0:16], serverWriteKey[16:],
				srtp.ProtectionProfileAes128CmHmacSha1_80, m.srtcpEndpoint,
			)
		}
	}

	return err
}

func (m *Manager) startDTLS(isOffer bool, dtlsCert *x509.Certificate, dtlsPrivKey crypto.PrivateKey, fingerprint, fingerprintHash string) error {
	dtlsCofig := &dtls.Config{Certificate: dtlsCert, PrivateKey: dtlsPrivKey}
	if isOffer {
		// Assumes we offer to be passive and this is accepted.
		dtlsConn, err := dtls.Server(m.dtlsEndpoint, dtlsCofig)
		if err != nil {
			return err
		}
		m.dtlsConn = dtlsConn
	} else {
		// Assumes the peer offered to be passive and we accepted.
		dtlsConn, err := dtls.Client(m.dtlsEndpoint, dtlsCofig)
		if err != nil {
			return err
		}
		m.dtlsConn = dtlsConn
	}

	// Check the fingerprint if a certificate was exchanged
	cert := m.dtlsConn.RemoteCertificate()
	if cert != nil {
		hashAlgo, err := dtls.HashAlgorithmString(fingerprintHash)
		if err != nil {
			return err
		}

		fp := ""
		fp, err = dtls.Fingerprint(cert, hashAlgo)
		if err != nil {
			return err
		}

		if strings.ToUpper(fp) != fingerprint {
			return fmt.Errorf("invalid fingerprint: %s <> %s", fp, fingerprint)
		}
	} else {
		fmt.Println("Warning: Certificate not checked")
	}
	return nil
}

// StartSCTP starts the SCTP association
func (m *Manager) StartSCTP(isOffer bool) error {
	if isOffer {
		sctpAssociation, err := sctp.Client(m.dtlsConn)
		if err != nil {
			return err
		}

		m.sctpAssociationMutex.Lock()
		m.sctpAssociation = sctpAssociation
		m.sctpAssociationMutex.Unlock()
	} else {
		sctpAssociation, err := sctp.Server(m.dtlsConn)
		if err != nil {
			return err
		}

		m.sctpAssociationMutex.Lock()
		m.sctpAssociation = sctpAssociation
		m.sctpAssociationMutex.Unlock()
	}
	return nil
}

// OpenDataChannel is used to open a data channel
// TODO: Move to RTCSctpTransport
func (m *Manager) OpenDataChannel(id uint16, config *datachannel.Config) (*datachannel.DataChannel, error) {
	return datachannel.Dial(m.sctpAssociation, id, config)
}

// AcceptDataChannel is used to accept incoming data channels
// TODO: Move to RTCSctpTransport
func (m *Manager) AcceptDataChannel() (*datachannel.DataChannel, error) {
	return datachannel.Accept(m.sctpAssociation)
}

// Close cleans up all the allocated state
func (m *Manager) Close() error {
	// Shutdown strategy:
	// 1. All Conn close by closing their underlying Conn.
	// 2. A Mux stops this chain. It won't close the underlying
	//    Conn if one of the endpoints is closed down. To
	//    continue the chain the Mux has to be closed.

	// Close SCTP. This should close the data channels, SCTP, and DTLS
	var errSCTP, errMux, errSRTP, errSRTCP error

	m.sctpAssociationMutex.RLock()
	if m.sctpAssociation != nil {
		errSCTP = m.sctpAssociation.Close()
	}
	m.sctpAssociationMutex.RUnlock()

	errSRTP = m.SrtpSession.Close()
	errSRTCP = m.SrtcpSession.Close()

	// Close the Mux. This should close the Mux and ICE.
	if m.mux != nil {
		errMux = m.mux.Close()
	}

	// TODO: better way to combine/handle errors?
	if errSCTP != nil ||
		errMux != nil ||
		errSRTP != nil ||
		errSRTCP != nil {
		return fmt.Errorf("Failed to close: %v, %v, %v, %v", errSCTP, errMux, errSRTP, errSRTCP)
	}

	return nil
}
