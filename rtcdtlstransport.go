package webrtc

import (
	"fmt"
	"sync"

	"github.com/pions/webrtc/internal/dtls"
)

// RTCDtlsTransport allows an application access to information about the DTLS
// transport over which RTP and RTCP packets are sent and received by
// RTCRtpSender and RTCRtpReceiver, as well other data such as SCTP packets sent
// and received by data channels.
type RTCDtlsTransport struct {
	sync.RWMutex

	Transport *RTCIceTransport
	State     RTCDtlsTransportState

	// OnStateChange func()
	// OnError       func()

	conn     *RTCPeerConnection
	dtls     *dtls.State
	toSctp   chan interface{}
	fromSctp chan interface{}
}

func newRTCDtlsTransport(connection *RTCPeerConnection) *RTCDtlsTransport {
	t := &RTCDtlsTransport{
		conn: connection,
	}
	t.Transport = newRTCIceTransport(connection)

	return t
}

func (r *RTCDtlsTransport) send(raw []byte) {
	// pair := r.Transport.GetSelectedCandidatePair()
	local, remote := m.IceAgent.SelectedPair()
	if remote == nil || local == nil {
		// Send data on any valid pair
		fmt.Println("dataChannelOutboundHandler: no valid candidates, dropping packet")
		return
	}

	m.portsLock.Lock()
	defer m.portsLock.Unlock()
	p, err := m.port(local)
	if err != nil {
		fmt.Println("dataChannelOutboundHandler: no valid port for candidate, dropping packet")
		return

	}
	// p.sendSCTP(raw, remote)

	_, err := p.m.dtlsState.Send(raw, p.listeningAddr.String(), dst.String())
	if err != nil {
		fmt.Println(err)
	}
}
