package network

import (
	"fmt"
	"sync"

	"github.com/pions/pkg/stun"
	"github.com/pions/webrtc/internal/dtls"
	"github.com/pions/webrtc/internal/sctp"
	"github.com/pions/webrtc/internal/srtp"
	webrtcStun "github.com/pions/webrtc/internal/stun"
	"github.com/pions/webrtc/pkg/datachannel"
	"github.com/pions/webrtc/pkg/ice"
	"github.com/pions/webrtc/pkg/rtcp"
	"github.com/pions/webrtc/pkg/rtp"
	"github.com/pkg/errors"
)

// Transportpair allows the application to be notified about both Rtp
// and Rtcp messages incoming from the remote host
type Transportpair struct {
	Rtp  chan<- *rtp.Packet
	Rtcp chan<- *rtcp.PacketWithHeader
}

// Manager contains all network state (DTLS, SRTP) that is shared between ports
// It is also used to perform operations that involve multiple ports
type Manager struct {
	IceAgent    *ice.Agent
	iceNotifier ICENotifier
	isOffer     bool

	dtlsState *dtls.State

	certPairLock sync.RWMutex
	certPair     *dtls.CertPair

	dataChannelEventHandler DataChannelEventHandler

	bufferTransportGenerator BufferTransportGenerator
	bufferTransportPairs     map[uint32]*Transportpair

	srtpInboundContextLock sync.RWMutex
	srtpInboundContext     *srtp.Context

	srtpOutboundContextLock sync.RWMutex
	srtpOutboundContext     *srtp.Context

	sctpAssociation *sctp.Association

	portsLock sync.RWMutex
	ports     []*port
}

//AddTransportPair notifies the network manager that an RTCTrack has
//been created externally, and packets may be incoming with this ssrc
func (m *Manager) AddTransportPair(ssrc uint32, Rtp chan<- *rtp.Packet, Rtcp chan<- *rtcp.PacketWithHeader) {
	bufferTransport := m.bufferTransportPairs[ssrc]
	if bufferTransport == nil {
		bufferTransport = &Transportpair{Rtp, Rtcp}
		m.bufferTransportPairs[ssrc] = bufferTransport
	}
}

// NewManager creates a new network.Manager
func NewManager(btg BufferTransportGenerator, dcet DataChannelEventHandler, ntf ICENotifier) (m *Manager, err error) {
	m = &Manager{
		iceNotifier:              ntf,
		bufferTransportPairs:     make(map[uint32]*Transportpair),
		bufferTransportGenerator: btg,
		dataChannelEventHandler:  dcet,
	}
	m.dtlsState, err = dtls.NewState(m.handleDTLSState)
	if err != nil {
		return nil, err
	}

	m.sctpAssociation = sctp.NewAssocation(m.dataChannelOutboundHandler, m.dataChannelInboundHandler, m.handleSCTPState)

	m.IceAgent = ice.NewAgent(m.iceNotifier)
	for _, i := range localInterfaces() {
		p, portErr := newPort(i+":0", m)
		if portErr != nil {
			return nil, portErr
		}

		m.ports = append(m.ports, p)
		m.IceAgent.AddLocalCandidate(&ice.CandidateHost{
			CandidateBase: ice.CandidateBase{
				Protocol: ice.ProtoTypeUDP,
				Address:  p.listeningAddr.IP.String(),
				Port:     p.listeningAddr.Port,
				Conn:     p.conn,
			},
		})
	}

	return m, err
}

func (m *Manager) getBufferTransports(ssrc uint32) *Transportpair {
	fmt.Println(m.bufferTransportPairs)
	return m.bufferTransportPairs[ssrc]
}

func (m *Manager) getOrCreateBufferTransports(ssrc uint32, payloadtype uint8) *Transportpair {
	bufferTransport := m.bufferTransportPairs[ssrc]
	if bufferTransport == nil {
		bufferTransport = m.bufferTransportGenerator(ssrc, payloadtype)
		fmt.Printf("CREATE FOR %x %v\n", ssrc, bufferTransport)
		m.bufferTransportPairs[ssrc] = bufferTransport
	}

	return bufferTransport
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

// AddURL takes an ICE Url, allocates any state and adds the candidate
func (m *Manager) AddURL(url *ice.URL) error {
	switch url.Scheme {
	case ice.SchemeTypeSTUN:
		laddr, xoraddr, err := webrtcStun.AllocateUDP(url)
		if err != nil {
			return err
		}

		p, err := newPort(laddr.String(), m)
		if err != nil {
			return err
		}

		c := &ice.CandidateSrflx{
			CandidateBase: ice.CandidateBase{
				Protocol: ice.ProtoTypeUDP,
				Address:  xoraddr.IP.String(),
				Port:     xoraddr.Port,
				Conn:     p.conn,
			},
			RemoteAddress: laddr.IP.String(),
			RemotePort:    laddr.Port,
		}

		m.portsLock.Lock()
		defer m.portsLock.Unlock()
		m.ports = append(m.ports, p)
		m.IceAgent.AddLocalCandidate(c)
	default:
		return errors.Errorf("%s is not implemented", url.Scheme.String())
	}

	return nil
}

// Start allocates DTLS/ICE state that is dependent on if we are offering or answering
func (m *Manager) Start(isOffer bool, remoteUfrag, remotePwd string) error {
	m.isOffer = isOffer

	// Start the sctpAssociation
	m.sctpAssociation.Start(isOffer)

	if err := m.IceAgent.Start(isOffer, remoteUfrag, remotePwd); err != nil {
		return err
	}
	// Start DTLS
	m.dtlsState.Start(isOffer)

	return nil
}

// Close cleans up all the allocated state
func (m *Manager) Close() {
	m.portsLock.Lock()
	defer m.portsLock.Unlock()

	err := m.sctpAssociation.Close()
	m.dtlsState.Close()
	m.IceAgent.Close()

	for i := len(m.ports) - 1; i >= 0; i-- {
		if portError := m.ports[i].close(); portError != nil {
			if err != nil {
				err = errors.Wrapf(portError, " also: %s", err.Error())
			} else {
				err = portError
			}
		} else {
			m.ports = append(m.ports[:i], m.ports[i+1:]...)
		}
	}
}

// DTLSFingerprint generates the fingerprint included in an SessionDescription
func (m *Manager) DTLSFingerprint() string {
	return m.dtlsState.Fingerprint()
}

// SendRTP finds a connected port and sends the passed RTP packet
func (m *Manager) SendRTP(packet *rtp.Packet) {

	local, remote := m.IceAgent.SelectedPair()
	if local == nil || remote == nil {
		return
	}

	m.portsLock.RLock()
	defer m.portsLock.RUnlock()
	for _, p := range m.ports {
		if p.listeningAddr.Equal(local) {
			p.sendRTP(packet, remote)
		}
	}
}

// SendRTCP finds a connected port and sends the passed RTCP packet
func (m *Manager) SendRTCP(pkt []byte) {
	local, remote := m.IceAgent.SelectedPair()
	if local == nil || remote == nil {
		return
	}

	m.portsLock.RLock()
	defer m.portsLock.RUnlock()
	for _, p := range m.ports {
		if p.listeningAddr.Equal(local) {
			p.sendRTCP(pkt, remote)
		}
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
	local, remote := m.IceAgent.SelectedPair()
	if remote == nil || local == nil {
		// Send data on any valid pair
		fmt.Println("dataChannelOutboundHandler: no valid candidates, dropping packet")
		return
	}

	m.portsLock.RLock()
	defer m.portsLock.RUnlock()
	p, err := m.port(local)
	if err != nil {
		fmt.Println("dataChannelOutboundHandler: no valid port for candidate, dropping packet")
		return

	}
	p.sendSCTP(raw, remote)
}

func (m *Manager) port(local *stun.TransportAddr) (*port, error) {
	for _, p := range m.ports {
		if p.listeningAddr.Equal(local) {
			return p, nil
		}
	}
	return nil, errors.New("port not found")
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
