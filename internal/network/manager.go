package network

import (
	"context"
	"crypto"
	"crypto/x509"
	"fmt"
	"strings"
	"sync"

	"github.com/pions/dtls/pkg/dtls"
	"github.com/pions/webrtc/internal/datachannel"
	"github.com/pions/webrtc/internal/mux"
	"github.com/pions/webrtc/internal/sctp"
	"github.com/pions/webrtc/internal/srtp"
	"github.com/pions/webrtc/pkg/ice"
	"github.com/pions/webrtc/pkg/rtcp"
	"github.com/pions/webrtc/pkg/rtp"
)

const receiveMTU = 8192

// TransportPair allows the application to be notified about both Rtp
// and Rtcp messages incoming from the remote host
type TransportPair struct {
	RTP  chan<- *rtp.Packet
	RTCP chan<- rtcp.Packet
}

// Manager contains all network state (DTLS, SRTP) that is shared between ports
// It is also used to perform operations that involve multiple ports
type Manager struct {
	IceAgent *ice.Agent
	iceConn  *ice.Conn
	isOffer  bool

	dtlsEndpoint *mux.Endpoint
	srtpEndpoint *mux.Endpoint

	srtpInboundContextLock  sync.RWMutex
	srtpInboundContext      *srtp.Context
	srtpOutboundContextLock sync.RWMutex
	srtpOutboundContext     *srtp.Context

	bufferTransportGenerator BufferTransportGenerator
	pairsLock                sync.RWMutex
	bufferTransportPairs     map[uint32]*TransportPair

	dtlsConn *dtls.Conn

	sctpAssociation *sctp.Association
}

//AddTransportPair notifies the network manager that an RTCTrack has
//been created externally, and packets may be incoming with this ssrc
func (m *Manager) AddTransportPair(ssrc uint32, Rtp chan<- *rtp.Packet, Rtcp chan<- rtcp.Packet) {
	m.pairsLock.Lock()
	defer m.pairsLock.Unlock()
	bufferTransport := m.bufferTransportPairs[ssrc]
	if bufferTransport == nil {
		bufferTransport = &TransportPair{Rtp, Rtcp}
		m.bufferTransportPairs[ssrc] = bufferTransport
	}
}

// NewManager creates a new network.Manager
func NewManager(urls []*ice.URL, btg BufferTransportGenerator, ntf ICENotifier) *Manager {
	iceAgent := ice.NewAgent(urls, ntf)

	return &Manager{
		IceAgent:                 iceAgent,
		bufferTransportPairs:     make(map[uint32]*TransportPair),
		bufferTransportGenerator: btg,
	}
}

func (m *Manager) getBufferTransports(ssrc uint32) *TransportPair {
	m.pairsLock.RLock()
	defer m.pairsLock.RUnlock()
	return m.bufferTransportPairs[ssrc]
}

func (m *Manager) getOrCreateBufferTransports(ssrc uint32, payloadtype uint8) *TransportPair {
	m.pairsLock.Lock()
	defer m.pairsLock.Unlock()
	bufferTransport := m.bufferTransportPairs[ssrc]
	if bufferTransport == nil {
		bufferTransport = m.bufferTransportGenerator(ssrc, payloadtype)
		m.bufferTransportPairs[ssrc] = bufferTransport
	}

	return bufferTransport
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

	mx := mux.NewMux(m.iceConn, receiveMTU)
	m.dtlsEndpoint = mx.NewEndpoint(mux.MatchDTLS)
	m.srtpEndpoint = mx.NewEndpoint(mux.MatchSRTP)

	m.startSRTP()

	if err := m.startDTLS(isOffer, dtlsCert, dtlsPrivKey, fingerprint, fingerprintHash); err != nil {
		return err
	}

	if err := m.createContextSRTP(isOffer); err != nil {
		return err
	}

	if err := m.startSCTP(isOffer); err != nil {
		return err
	}

	return nil
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

func (m *Manager) startSRTP() {
	// Glue code until SRTP is a Conn.
	go func() {
		buf := make([]byte, receiveMTU)
		for {
			n, err := m.srtpEndpoint.Read(buf)
			if err != nil {
				return
			}
			m.handleSRTP(buf[:n])
		}
	}()
}

func (m *Manager) createContextSRTP(isOffer bool) error {
	keyingMaterial, err := m.dtlsConn.ExportKeyingMaterial([]byte("EXTRACTOR-dtls_srtp"), nil, (srtpMasterKeyLen*2)+(srtpMasterKeySaltLen*2))
	if err != nil {
		return err
	}
	if err = m.CreateContextSRTP(keyingMaterial, isOffer); err != nil {
		return err
	}
	return nil
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

func (m *Manager) startSCTP(isOffer bool) error {
	if isOffer {
		sctpAssociation, err := sctp.Client(m.dtlsConn)
		if err != nil {
			return err
		}
		m.sctpAssociation = sctpAssociation
	} else {
		sctpAssociation, err := sctp.Server(m.dtlsConn)
		if err != nil {
			return err
		}
		m.sctpAssociation = sctpAssociation
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
	var errSCTP, errDTLS, errICE error
	if m.sctpAssociation != nil {
		errSCTP = m.sctpAssociation.Close()
	}
	if m.dtlsConn != nil {
		errDTLS = m.dtlsConn.Close()
	}
	if m.IceAgent != nil {
		errICE = m.IceAgent.Close()
	}

	// TODO: better way to combine/handle errors?
	if errSCTP != nil ||
		errDTLS != nil ||
		errICE != nil {
		return fmt.Errorf("Failed to close: %v, %v, %v", errSCTP, errDTLS, errICE)
	}

	return nil
}

// SendRTP finds a connected port and sends the passed RTP packet
func (m *Manager) SendRTP(packet *rtp.Packet) {
	m.srtpOutboundContextLock.Lock()
	defer m.srtpOutboundContextLock.Unlock()
	if m.srtpOutboundContext == nil {
		// TODO log-level
		// fmt.Printf("Tried to send RTP packet but no SRTP Context to handle it \n")
		return
	}

	if ok := m.srtpOutboundContext.EncryptRTP(packet); !ok {
		fmt.Println("SendRTP failed to encrypt packet")
		return
	}

	raw, err := packet.Marshal()
	if err != nil {
		fmt.Printf("SendRTP failed to marshal packet: %s \n", err.Error())
	}

	_, err = m.iceConn.Write(raw)
	if err != nil {
		fmt.Println("SendRTP failed to write:", err)
	}
}

// SendRTCP finds a connected port and sends the passed RTCP packet
func (m *Manager) SendRTCP(pkt []byte) {
	m.srtpOutboundContextLock.Lock()
	defer m.srtpOutboundContextLock.Unlock()
	if m.srtpOutboundContext == nil {
		fmt.Printf("Tried to send RTCP packet but no SRTP Context to handle it \n")
		return
	}

	encrypted, err := m.srtpOutboundContext.EncryptRTCP(pkt)
	if err != nil {
		fmt.Println("SendRTCP failed to encrypt packet:", err)
		return
	}

	_, err = m.iceConn.Write(encrypted)
	if err != nil {
		fmt.Println("SendRTCP failed to write:", err)
	}
}
