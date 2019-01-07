package network

import (
	"fmt"

	"github.com/pions/dtls/pkg/dtls"
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

// Close cleans up all the allocated state
func (m *Manager) Close() error {
	var errSRTP, errSRTCP error

	errSRTP = m.SrtpSession.Close()
	errSRTCP = m.SrtcpSession.Close()

	// TODO: better way to combine/handle errors?
	if errSRTP != nil ||
		errSRTCP != nil {
		return fmt.Errorf("Failed to close: %v, %v", errSRTP, errSRTCP)
	}

	return nil
}
