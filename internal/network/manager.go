package network

import (
	"context"
	"crypto"
	"crypto/x509"
	"fmt"
	"strings"
	"sync"

	"github.com/pions/dtls/pkg/dtls"
	"github.com/pions/webrtc/internal/sctp"
	"github.com/pions/webrtc/internal/srtp"
	"github.com/pions/webrtc/pkg/datachannel"
	"github.com/pions/webrtc/pkg/ice"
	"github.com/pions/webrtc/pkg/rtcp"
	"github.com/pions/webrtc/pkg/rtp"
	"github.com/pkg/errors"
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
	IceAgent    *ice.Agent
	iceConn     *ice.Conn
	iceNotifier ICENotifier
	isOffer     bool

	srtpInboundContextLock  sync.RWMutex
	srtpInboundContext      *srtp.Context
	srtpOutboundContextLock sync.RWMutex
	srtpOutboundContext     *srtp.Context

	bufferTransportGenerator BufferTransportGenerator
	pairsLock                sync.RWMutex
	bufferTransportPairs     map[uint32]*TransportPair

	dtlsConn *dtls.Conn

	sctpAssociation *sctp.Association

	dataChannelEventHandler DataChannelEventHandler
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
func NewManager(urls []*ice.URL, btg BufferTransportGenerator, dcet DataChannelEventHandler, ntf ICENotifier) (m *Manager, err error) {
	m = &Manager{
		iceNotifier:              ntf,
		bufferTransportPairs:     make(map[uint32]*TransportPair),
		bufferTransportGenerator: btg,
		dataChannelEventHandler:  dcet,
	}

	m.sctpAssociation = sctp.NewAssocation(m.dataChannelOutboundHandler, m.dataChannelInboundHandler, m.handleSCTPState)

	m.IceAgent = ice.NewAgent(urls, m.iceNotifier)

	return m, err
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

func (m *Manager) handleSCTPState(state sctp.AssociationState) {
	if state == sctp.Established {
		// Temporary way to signal sending OpenChannel messages
		m.dataChannelEventHandler(&DataChannelOpen{})
	}
}

// Start allocates DTLS/ICE state that is dependent on if we are offering or answering
func (m *Manager) Start(isOffer bool,
	remoteUfrag, remotePwd string,
	dtlsCert *x509.Certificate, dtlsPrivKey crypto.PrivateKey, fingerprint, fingerprintHash string) error {

	m.isOffer = isOffer

	// Start the sctpAssociation
	m.sctpAssociation.Start(isOffer)

	// Spin up ICE
	var iceConn *ice.Conn
	var err error
	if isOffer {
		iceConn, err = m.IceAgent.Dial(context.TODO(), remoteUfrag, remotePwd)
	} else {
		iceConn, err = m.IceAgent.Accept(context.TODO(), remoteUfrag, remotePwd)
	}

	if err != nil {
		return err
	}

	m.iceConn = iceConn

	// Spin up SRTP
	srtpConn := srtp.Wrap(iceConn, m.handleSRTP)

	// Spin up DTLS
	var dtlsConn *dtls.Conn
	dtlsCofig := &dtls.Config{Certificate: dtlsCert, PrivateKey: dtlsPrivKey}
	if isOffer {
		// Assumes we offer to be passive and this is accepted.
		dtlsConn, err = dtls.Server(srtpConn, dtlsCofig)
	} else {
		// Assumes the peer offered to be passive and we accepted.
		dtlsConn, err = dtls.Client(srtpConn, dtlsCofig)
	}

	if err != nil {
		return err
	}

	m.dtlsConn = dtlsConn

	keyingMaterial, err := dtlsConn.ExportKeyingMaterial([]byte("EXTRACTOR-dtls_srtp"), nil, (srtpMasterKeyLen*2)+(srtpMasterKeySaltLen*2))
	if err != nil {
		return err
	}
	if err = m.CreateContextSRTP(keyingMaterial); err != nil {
		return err
	}

	// Check the fingerprint if a certificate was exchanged
	cert := dtlsConn.RemoteCertificate()
	if cert != nil {
		hashAlgo, err := dtls.HashAlgorithmString(fingerprintHash)
		if err != nil {
			return err
		}

		fp, err := dtls.Fingerprint(cert, hashAlgo)
		if err != nil {
			return err
		}

		if strings.ToUpper(fp) != fingerprint {
			return fmt.Errorf("invalid fingerprint: %s <> %s", fp, fingerprint)
		}
	} else {
		fmt.Println("Warning: Certificate not checked")
	}

	// Temporary networking glue for SCTP
	go m.networkLoop()

	// Spin up SCTP
	m.sctpAssociation.Connect()

	return nil
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

// SendDataChannelMessage sends a DataChannel message to a connected peer
func (m *Manager) SendDataChannelMessage(payload datachannel.Payload, streamIdentifier uint16) error {
	var data []byte
	var ppi sctp.PayloadProtocolIdentifier

	/*
		https://tools.ietf.org/html/draft-ietf-rtcweb-data-channel-12#section-6.6
		SCTP does not support the sending of empty user messages.  Therefore,
		if an empty message has to be sent, the appropriate PPID (WebRTC
		String Empty or WebRTC Binary Empty) is used and the SCTP user
		message of one zero byte is sent.  When receiving an SCTP user
		message with one of these PPIDs, the receiver MUST ignore the SCTP
		user message and process it as an empty message.
	*/
	switch p := payload.(type) {
	case datachannel.PayloadString:
		data = p.Data
		if len(data) == 0 {
			data = []byte{0}
			ppi = sctp.PayloadTypeWebRTCStringEmpty
		} else {
			ppi = sctp.PayloadTypeWebRTCString
		}
	case datachannel.PayloadBinary:
		data = p.Data
		if len(data) == 0 {
			data = []byte{0}
			ppi = sctp.PayloadTypeWebRTCBinaryEmpty
		} else {
			ppi = sctp.PayloadTypeWebRTCBinary
		}
	default:
		return errors.Errorf("Unknown DataChannel Payload (%s)", payload.PayloadType().String())
	}

	m.sctpAssociation.Lock()
	err := m.sctpAssociation.HandleOutbound(data, streamIdentifier, ppi)
	m.sctpAssociation.Unlock()

	if err != nil {
		return errors.Wrap(err, "SCTP Association failed handling outbound packet")
	}

	return nil
}

func (m *Manager) dataChannelInboundHandler(data []byte, streamIdentifier uint16, payloadType sctp.PayloadProtocolIdentifier) {
	switch payloadType {
	case sctp.PayloadTypeWebRTCDCEP:
		msg, err := datachannel.Parse(data)
		if err != nil {
			fmt.Println(errors.Wrap(err, "Failed to parse DataChannel packet"))
			return
		}
		switch msg := msg.(type) {
		case *datachannel.ChannelOpen:
			// Cannot return err
			ack := datachannel.ChannelAck{}
			ackMsg, err := ack.Marshal()
			if err != nil {
				fmt.Println("Error Marshaling ChannelOpen ACK", err)
				return
			}
			if err = m.sctpAssociation.HandleOutbound(ackMsg, streamIdentifier, sctp.PayloadTypeWebRTCDCEP); err != nil {
				fmt.Println("Error sending ChannelOpen ACK", err)
				return
			}
			m.dataChannelEventHandler(&DataChannelCreated{streamIdentifier: streamIdentifier, Label: string(msg.Label)})
		case *datachannel.ChannelAck:
			// TODO: handle ChannelAck (https://tools.ietf.org/html/draft-ietf-rtcweb-data-protocol-09#section-5.2)
		default:
			fmt.Println("Unhandled DataChannel message", msg)
		}
	case sctp.PayloadTypeWebRTCString:
		fallthrough
	case sctp.PayloadTypeWebRTCStringEmpty:
		payload := &datachannel.PayloadString{Data: data}
		m.dataChannelEventHandler(&DataChannelMessage{streamIdentifier: streamIdentifier, Payload: payload})
	case sctp.PayloadTypeWebRTCBinary:
		fallthrough
	case sctp.PayloadTypeWebRTCBinaryEmpty:
		payload := &datachannel.PayloadBinary{Data: data}
		m.dataChannelEventHandler(&DataChannelMessage{streamIdentifier: streamIdentifier, Payload: payload})
	default:
		fmt.Printf("Unhandled Payload Protocol Identifier %v \n", payloadType)
	}
}

func (m *Manager) dataChannelOutboundHandler(raw []byte) {
	if _, err := m.dtlsConn.Write(raw); err != nil {
		fmt.Println(err)
	}
}

// SendOpenChannelMessage sends the message to open a datachannel to the connected peer
func (m *Manager) SendOpenChannelMessage(streamIdentifier uint16, label string) error {
	msg := &datachannel.ChannelOpen{
		ChannelType:          datachannel.ChannelTypeReliable,
		Priority:             datachannel.ChannelPriorityNormal,
		ReliabilityParameter: 0,

		Label:    []byte(label),
		Protocol: []byte(""),
	}

	rawMsg, err := msg.Marshal()
	if err != nil {
		return fmt.Errorf("Error Marshaling ChannelOpen %v", err)
	}
	m.sctpAssociation.Lock()
	defer m.sctpAssociation.Unlock()
	if err = m.sctpAssociation.HandleOutbound(rawMsg, streamIdentifier, sctp.PayloadTypeWebRTCDCEP); err != nil {
		return fmt.Errorf("Error sending ChannelOpen %v", err)
	}
	return nil
}
