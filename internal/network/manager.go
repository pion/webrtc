package network

import (
	"context"
	"fmt"
	"sync"

	"github.com/pions/webrtc/internal/dtls"
	"github.com/pions/webrtc/internal/sctp"
	"github.com/pions/webrtc/internal/srtp"
	"github.com/pions/webrtc/pkg/datachannel"
	"github.com/pions/webrtc/pkg/ice"
	"github.com/pions/webrtc/pkg/rtp"
	"github.com/pkg/errors"
)

// Manager contains all network state (DTLS, SRTP) that is shared between ports
// It is also used to perform operations that involve multiple ports
type Manager struct {
	IceAgent    *ice.Agent
	iceConn     *ice.Conn
	iceNotifier ICENotifier
	isOffer     bool

	dtlsState *dtls.State

	certPairLock sync.RWMutex
	certPair     *dtls.CertPair

	dataChannelEventHandler DataChannelEventHandler

	bufferTransportGenerator BufferTransportGenerator
	bufferTransports         map[uint32]chan<- *rtp.Packet

	srtpInboundContextLock sync.RWMutex
	srtpInboundContext     *srtp.Context

	srtpOutboundContextLock sync.RWMutex
	srtpOutboundContext     *srtp.Context

	sctpAssociation *sctp.Association
}

// NewManager creates a new network.Manager
func NewManager(urls []*ice.URL, btg BufferTransportGenerator, dcet DataChannelEventHandler, ntf ICENotifier) (m *Manager, err error) {
	m = &Manager{
		iceNotifier:              ntf,
		bufferTransports:         make(map[uint32]chan<- *rtp.Packet),
		bufferTransportGenerator: btg,
		dataChannelEventHandler:  dcet,
	}
	m.dtlsState, err = dtls.NewState(m.handleDTLSState)
	if err != nil {
		return nil, err
	}

	m.sctpAssociation = sctp.NewAssocation(m.dataChannelOutboundHandler, m.dataChannelInboundHandler, m.handleSCTPState)

	m.IceAgent = ice.NewAgent(urls, m.iceNotifier)

	return m, err
}

func (m *Manager) handleDTLSState(state dtls.ConnectionState) {
	if state == dtls.Established {
		m.sctpAssociation.Connect()
	}
}

func (m *Manager) handleSCTPState(state sctp.AssociationState) {
	if state == sctp.Established {
		// Temporary way to signal sending OpenChannel messages
		m.dataChannelEventHandler(&DataChannelOpen{})
	}
}

// Start allocates DTLS/ICE state that is dependent on if we are offering or answering
func (m *Manager) Start(isOffer bool, remoteUfrag, remotePwd string) error {
	m.isOffer = isOffer

	// Start the sctpAssociation
	m.sctpAssociation.Start(isOffer)

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

	// Start DTLS
	m.dtlsState.Start(isOffer, iceConn)

	m.certPairLock.RLock()
	if !m.isOffer && m.certPair == nil {
		m.dtlsState.DoHandshake("0.0.0.0", "0.0.0.0")
	}
	m.certPairLock.RUnlock()

	// Temporary networking glue
	go m.networkLoop()

	return nil
}

// Close cleans up all the allocated state
func (m *Manager) Close() error {
	errSCTP := m.sctpAssociation.Close()
	m.dtlsState.Close()
	errICE := m.IceAgent.Close() // TODO: combine errors?

	if errSCTP != nil ||
		errICE != nil {
		return fmt.Errorf("Failed to close: %v, %v", errSCTP, errICE)
	}

	return nil
}

// DTLSFingerprint generates the fingerprint included in an SessionDescription
func (m *Manager) DTLSFingerprint() string {
	return m.dtlsState.Fingerprint()
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
	_, err := m.dtlsState.Send(raw, "0.0.0.0", "0.0.0.0")
	if err != nil {
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
