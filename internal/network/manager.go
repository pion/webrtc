package network

import (
	"bytes"
	"context"
	"crypto"
	"crypto/x509"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/pions/datachannel"
	"github.com/pions/dtls/pkg/dtls"
	"github.com/pions/sctp"
	"github.com/pions/webrtc/internal/mux"
	"github.com/pions/webrtc/internal/srtp"
	"github.com/pions/webrtc/pkg/ice"
	"github.com/pions/webrtc/pkg/rtcp"
	"github.com/pions/webrtc/pkg/rtp"
)

const (
	srtpMasterKeyLen     = 16
	srtpMasterKeySaltLen = 14
	receiveMTU           = 8192
)

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

	mux *mux.Mux

	dtlsEndpoint  *mux.Endpoint
	srtpEndpoint  *mux.Endpoint
	srtcpEndpoint *mux.Endpoint

	bufferTransportGenerator BufferTransportGenerator
	pairsLock                sync.RWMutex
	bufferTransportPairs     map[uint32]*TransportPair

	dtlsConn *dtls.Conn

	srtpSession  *srtp.SessionSRTP
	srtcpSession *srtp.SessionSRTCP

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
func NewManager(urls []*ice.URL, btg BufferTransportGenerator, ntf ICENotifier, minport, maxport uint16) (*Manager, error) {
	config := &ice.AgentConfig{Urls: urls, Notifier: ntf, PortMin: minport, PortMax: maxport}
	iceAgent, err := ice.NewAgent(config)

	if err != nil {
		return nil, err
	}

	return &Manager{
		IceAgent:                 iceAgent,
		bufferTransportPairs:     make(map[uint32]*TransportPair),
		bufferTransportGenerator: btg,
	}, nil
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

	m.mux = mux.NewMux(m.iceConn, receiveMTU)
	m.dtlsEndpoint = m.mux.NewEndpoint(mux.MatchDTLS)
	m.srtpEndpoint = m.mux.NewEndpoint(mux.MatchSRTP)
	m.srtcpEndpoint = m.mux.NewEndpoint(mux.MatchSRTCP)

	if err := m.startDTLS(isOffer, dtlsCert, dtlsPrivKey, fingerprint, fingerprintHash); err != nil {
		return err
	}

	if err := m.startSRTP(isOffer); err != nil {
		return err
	}

	return m.startSCTP(isOffer)
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

func (m *Manager) handleSRTP() {
	buf := make([]byte, receiveMTU)
	for {
		i, err := m.srtpSession.Read(buf)
		if err != nil {
			return
		}

		packet := &rtp.Packet{}
		if err := packet.Unmarshal(append([]byte{}, buf[:i]...)); err != nil {
			fmt.Println("Failed to unmarshal RTP packet")
			return
		}

		bufferTransport := m.getOrCreateBufferTransports(packet.SSRC, packet.PayloadType)
		if bufferTransport != nil && bufferTransport.RTP != nil {
			select {
			case bufferTransport.RTP <- packet:
			default:
			}
		}
	}
}

func (m *Manager) handleSRTCP() {
	buf := make([]byte, receiveMTU)
	for {
		i, err := m.srtcpSession.Read(buf)
		if err != nil {
			return
		}
		handleRTCP(m.getBufferTransports, buf[:i])
	}
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
		m.srtpSession, err = srtp.CreateSessionSRTP(
			serverWriteKey[0:16], serverWriteKey[16:],
			clientWriteKey[0:16], clientWriteKey[16:],
			srtp.ProtectionProfileAes128CmHmacSha1_80, m.srtpEndpoint,
		)

		if err == nil {
			m.srtcpSession, err = srtp.CreateSessionSRTCP(
				serverWriteKey[0:16], serverWriteKey[16:],
				clientWriteKey[0:16], clientWriteKey[16:],
				srtp.ProtectionProfileAes128CmHmacSha1_80, m.srtcpEndpoint,
			)
		}
	} else {
		m.srtpSession, err = srtp.CreateSessionSRTP(
			clientWriteKey[0:16], clientWriteKey[16:],
			serverWriteKey[0:16], serverWriteKey[16:],
			srtp.ProtectionProfileAes128CmHmacSha1_80, m.srtpEndpoint,
		)

		if err == nil {
			m.srtcpSession, err = srtp.CreateSessionSRTCP(
				clientWriteKey[0:16], clientWriteKey[16:],
				serverWriteKey[0:16], serverWriteKey[16:],
				srtp.ProtectionProfileAes128CmHmacSha1_80, m.srtcpEndpoint,
			)
		}
	}

	if err == nil {
		go m.handleSRTP()
		go m.handleSRTCP()
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
	// Shutdown strategy:
	// 1. All Conn close by closing their underlying Conn.
	// 2. A Mux stops this chain. It won't close the underlying
	//    Conn if one of the endpoints is closed down. To
	//    continue the chain the Mux has to be closed.

	// Close SCTP. This should close the data channels, SCTP, and DTLS
	var errSCTP, errMux, errSRTP, errSRTCP error
	if m.sctpAssociation != nil {
		errSCTP = m.sctpAssociation.Close()
	}

	if m.srtpSession != nil {
		errSRTP = m.srtpSession.Close()
	}

	if m.srtcpSession != nil {
		errSRTCP = m.srtcpSession.Close()
	}

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

// SendRTP finds a connected port and sends the passed RTP packet
func (m *Manager) SendRTP(packet *rtp.Packet) {
	if m.srtpSession == nil {
		// TODO log-level
		// fmt.Printf("Tried to send RTP packet but no SRTP Context to handle it \n")
		return
	}

	raw, err := packet.Marshal()
	if err != nil {
		fmt.Printf("SendRTP failed to marshal packet: %s \n", err.Error())
		return
	}

	if _, err = m.srtpSession.Write(raw); err != nil {
		fmt.Println("SendRTP failed to write:", err)
	}
}

// SendRTCP finds a connected port and sends the passed RTCP packet
func (m *Manager) SendRTCP(pkt []byte) {
	if m.srtpSession == nil {
		// TODO log-level
		// fmt.Printf("Tried to send RTCP packet but no SRTCP Context to handle it \n")
		return
	}

	if _, err := m.srtcpSession.Write(pkt); err != nil {
		fmt.Println("SendRTCP failed to write:", err)
	}
}

func handleRTCP(getBufferTransports func(uint32) *TransportPair, buffer []byte) {
	//decrypted packets can also be compound packets, so we have to nest our reader loop here.
	compoundPacket := rtcp.NewReader(bytes.NewReader(buffer))
	for {
		_, rawrtcp, err := compoundPacket.ReadPacket()

		if err != nil {
			if err == io.EOF {
				break
			}
			fmt.Println(err)
			return
		}

		var report rtcp.Packet
		report, _, err = rtcp.Unmarshal(rawrtcp)
		if err != nil {
			fmt.Println(err)
			return
		}

		f := func(ssrc uint32) {
			bufferTransport := getBufferTransports(ssrc)
			if bufferTransport != nil && bufferTransport.RTCP != nil {
				select {
				case bufferTransport.RTCP <- report:
				default:
				}
			}
		}

		for _, ssrc := range report.DestinationSSRC() {
			f(ssrc)
		}
	}
}
