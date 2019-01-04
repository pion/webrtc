package network

import (
	"fmt"
	"sync"

	"github.com/pions/datachannel"
	"github.com/pions/dtls/pkg/dtls"
	"github.com/pions/sctp"
	"github.com/pions/webrtc/internal/mux"
	"github.com/pions/webrtc/internal/srtp"
)

const (
	srtpMasterKeyLen     = 16
	srtpMasterKeySaltLen = 14
)

// Manager contains all network state (DTLS, SRTP) that is shared between ports
// It is also used to perform operations that involve multiple ports
type Manager struct {
	isOffer bool

	SrtpSession  *srtp.SessionSRTP
	SrtcpSession *srtp.SessionSRTCP

	mux *mux.Mux

	srtpEndpoint  *mux.Endpoint
	srtcpEndpoint *mux.Endpoint

	dtlsConn *dtls.Conn

	sctpAssociationMutex sync.RWMutex
	sctpAssociation      *sctp.Association
}

// NewManager creates a new network.Manager
func NewManager() *Manager {
	return &Manager{
		SrtpSession:  srtp.CreateSessionSRTP(),
		SrtcpSession: srtp.CreateSessionSRTCP(),
	}
}

// Start starts the network manager
func (m *Manager) Start(mx *mux.Mux, dtlsConn *dtls.Conn, isOffer bool) error {
	m.isOffer = isOffer

	m.mux = mx
	m.srtpEndpoint = m.mux.NewEndpoint(mux.MatchSRTP)
	m.srtcpEndpoint = m.mux.NewEndpoint(mux.MatchSRTCP)

	m.dtlsConn = dtlsConn

	return m.startSRTP(isOffer)
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
	var errSCTP, errSRTP, errSRTCP error

	m.sctpAssociationMutex.RLock()
	if m.sctpAssociation != nil {
		errSCTP = m.sctpAssociation.Close()
	}
	m.sctpAssociationMutex.RUnlock()

	errSRTP = m.SrtpSession.Close()
	errSRTCP = m.SrtcpSession.Close()

	// TODO: better way to combine/handle errors?
	if errSCTP != nil ||
		errSRTP != nil ||
		errSRTCP != nil {
		return fmt.Errorf("Failed to close: %v, %v, %v", errSCTP, errSRTP, errSRTCP)
	}

	return nil
}
