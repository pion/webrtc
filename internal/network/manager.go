package network

import (
	"fmt"
	"net"
	"strconv"
	"sync"

	"github.com/pions/pkg/stun"
	"github.com/pions/webrtc/internal/dtls"
	"github.com/pions/webrtc/internal/sctp"
	"github.com/pions/webrtc/internal/srtp"
	webrtcStun "github.com/pions/webrtc/internal/stun"
	"github.com/pions/webrtc/pkg/datachannel"
	"github.com/pions/webrtc/pkg/ice"
	"github.com/pions/webrtc/pkg/rtp"
	"github.com/pkg/errors"
)

// Manager contains all network state (DTLS, SRTP) that is shared between ports
// It is also used to perform operations that involve multiple ports
type Manager struct {
	IceAgent    *ice.Agent
	iceNotifier ICENotifier

	dtlsState *dtls.State

	certPairLock sync.RWMutex
	certPair     *dtls.CertPair

	dataChannelEventHandler DataChannelEventHandler

	bufferTransportGenerator BufferTransportGenerator
	bufferTransports         map[uint32]chan<- *rtp.Packet

	// https://tools.ietf.org/html/rfc3711#section-3.2.3
	// A cryptographic context SHALL be uniquely identified by the triplet
	//  <SSRC, destination network address, destination transport port number>
	// contexts are keyed by IP:PORT:SSRC
	srtpContextsLock sync.RWMutex
	srtpContexts     map[string]*srtp.Context

	sctpAssociation *sctp.Association

	portsLock sync.RWMutex
	ports     []*port
}

// NewManager creates a new network.Manager
func NewManager(bufferTransportGenerator BufferTransportGenerator, dataChannelEventHandler DataChannelEventHandler, iceNotifier ICENotifier) (m *Manager, err error) {
	m = &Manager{
		iceNotifier:              iceNotifier,
		bufferTransports:         make(map[uint32]chan<- *rtp.Packet),
		srtpContexts:             make(map[string]*srtp.Context),
		bufferTransportGenerator: bufferTransportGenerator,
		dataChannelEventHandler:  dataChannelEventHandler,
	}
	m.dtlsState, err = dtls.NewState(true)
	if err != nil {
		return nil, err
	}

	m.sctpAssociation = sctp.NewAssocation(m.dataChannelOutboundHandler, m.dataChannelInboundHandler)

	m.IceAgent = ice.NewAgent(false /* isControlling TODO */, m.iceOutboundHandler)
	for _, i := range localInterfaces() {
		p, err := newPort(i+":0", m)
		if err != nil {
			return nil, err
		}

		m.ports = append(m.ports, p)
		m.IceAgent.AddLocalCandidate(&ice.CandidateHost{
			CandidateBase: ice.CandidateBase{
				Protocol: ice.TransportUDP,
				Address:  p.listeningAddr.IP.String(),
				Port:     p.listeningAddr.Port,
			},
		})
	}

	return m, err
}

// AddURL takes an ICE Url, allocates any state and adds the candidate
func (m *Manager) AddURL(url *ice.URL) error {
	switch url.Type {
	case ice.ServerTypeSTUN:
		c, err := webrtcStun.Allocate(url)
		if err != nil {
			return err
		}
		p, err := newPort(c.RemoteAddress+":"+strconv.Itoa(c.RemotePort), m)
		if err != nil {
			return err
		}

		m.ports = append(m.ports, p)
		m.IceAgent.AddLocalCandidate(c)
	default:
		return errors.Errorf("%s is not implemented", url.Type.String())
	}

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
	m.portsLock.Lock()
	defer m.portsLock.Unlock()

	local, remote := m.IceAgent.SelectedPair()
	if local == nil || remote == nil {
		return
	}
	for _, p := range m.ports {
		if p.listeningAddr.Equal(local) {
			p.sendRTP(packet, remote)
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
		default:
			fmt.Println("Unhandled DataChannel message", m)
		}
	case sctp.PayloadTypeWebRTCString:
		fallthrough
	case sctp.PayloadTypeWebRTCStringEmpty:
		m.dataChannelEventHandler(&DataChannelMessage{streamIdentifier: streamIdentifier, Payload: &datachannel.PayloadString{Data: data}})
	case sctp.PayloadTypeWebRTCBinary:
		fallthrough
	case sctp.PayloadTypeWebRTCBinaryEmpty:
		m.dataChannelEventHandler(&DataChannelMessage{streamIdentifier: streamIdentifier, Payload: &datachannel.PayloadBinary{Data: data}})
	default:
		fmt.Printf("Unhandled Payload Protocol Identifier %v \n", payloadType)
	}
}

func (m *Manager) dataChannelOutboundHandler(raw []byte) {
	m.portsLock.Lock()
	defer m.portsLock.Unlock()

	local, remote := m.IceAgent.SelectedPair()
	if local == nil || remote == nil {
		return
	}
	for _, p := range m.ports {
		if p.listeningAddr.Equal(local) {
			p.sendSCTP(raw, remote)
		}
	}
}

func (m *Manager) iceOutboundHandler(raw []byte, local *stun.TransportAddr, remote *net.UDPAddr) {
	m.portsLock.Lock()
	defer m.portsLock.Unlock()

	for _, p := range m.ports {
		if p.listeningAddr == local {
			p.sendICE(raw, remote)
			return
		}
	}
}
