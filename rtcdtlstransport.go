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

func newRTCDtlsTransport(connection *RTCPeerConnection) (*RTCDtlsTransport, error) {
	t := &RTCDtlsTransport{
		State: RTCDtlsTransportStateNew,
		conn:  connection,
	}
	t.dtls = dtls.NewState()

	var err error

	t.Transport, err = newRTCIceTransport(connection)
	if err != nil {
		return nil, err
	}

	// ice -> dtls
	t.Transport.toDtls = t.dtls.Input

	// ice <- dtls
	t.Transport.fromDtls = t.dtls.Output

	go t.sctpHandler()
	return t, nil
}

func (t *RTCDtlsTransport) sctpHandler() {
	for {
		raw, ok := (<-t.fromSctp).([]byte)
		if !ok {
			return
		}

		pair := t.Transport.GetSelectedCandidatePair()
		// local, remote := m.IceAgent.SelectedPair()
		if pair.remote == nil || pair.local == nil {
			// Send data on any valid pair
			fmt.Println("dataChannelOutboundHandler: no valid candidates, dropping packet")
			return
		}

		m.portsLock.Lock()
		defer m.portsLock.Unlock()
		p, err := m.port(pair.local)
		if err != nil {
			fmt.Println("dataChannelOutboundHandler: no valid port for candidate, dropping packet")
			return

		}

		_, err = t.dtls.Send(raw, p.listeningAddr.String(), dst.String())
		if err != nil {
			fmt.Println(err)
		}
	}
}
