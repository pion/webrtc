package webrtc

import (
	"fmt"

	"github.com/pions/webrtc/pkg/ice"
)

// RTCIceTransport allows an application access to information about the ICE
// transport over which packets are sent and received.
type RTCIceTransport struct {
	// Role RTCIceRole
	Component RTCIceComponent
	// State RTCIceTransportState
	// gatheringState RTCIceGathererState

	// OnStateChange func()
	// OnGatheringStateChange func()
	// OnSelectedCandidatePairChange func()

	agent    *ice.Agent
	conn     *RTCPeerConnection
	toDtls   chan interface{}
	fromDtls chan interface{}
}

func newRTCIceTransport(connection *RTCPeerConnection) (*RTCIceTransport, error) {
	t := &RTCIceTransport{
		conn: connection,
	}
	var err error

	t.agent, err = ice.NewAgent()
	if err != nil {
		return nil, err
	}

	go t.dtlsHandler()
	go t.iceHandler()
	return t, nil
}

func (t *RTCIceTransport) dtlsHandler() {
	for {
		raw, ok := (<-t.fromDtls).([]byte)
		if !ok {
			return
		}

		// TODO do stuff here
	}
}

func (t *RTCIceTransport) iceHandler() {
	for {
		raw, ok := <-t.agent.Output
		if !ok {
			return
		}
	}

	// in, socketOpen := <-incomingPackets
	// if !socketOpen {
	// 	// incomingPackets channel has closed, this port is finished processing
	// 	dtls.RemoveListener(p.listeningAddr.String())
	// 	return
	// }

	if len(buffer) == 0 {
		fmt.Println("Inbound buffer is not long enough to demux")
		return
	}

	// https://tools.ietf.org/html/rfc5764#page-14
	if 127 < buffer[0] && buffer[0] < 192 {
		p.handleSRTP(buffer)
	} else if 19 < buffer[0] && buffer[0] < 64 {
		p.handleDTLS(buffer, remoteAddr.String())
	} else if buffer[0] < 2 {
		p.m.IceAgent.HandleInbound(buffer, p.listeningAddr, remoteAddr.String())
	}

	p.m.certPairLock.RLock()
	if !p.m.isOffer && p.m.certPair == nil {
		p.m.dtlsState.DoHandshake(p.listeningAddr.String(), remoteAddr.String())
	}
	p.m.certPairLock.RUnlock()
}

func (t *RTCIceTransport) GetLocalCandidates() []RTCIceCandidate {
	return []RTCIceCandidate{}
}

func (t *RTCIceTransport) GetRemoteCandidates() []RTCIceCandidate {
	return []RTCIceCandidate{}
}

func (t *RTCIceTransport) GetSelectedCandidatePair() RTCIceCandidatePair {
	return RTCIceCandidatePair{}
}

func (t *RTCIceTransport) GetLocalParameters() RTCIceParameters {
	return RTCIceParameters{}
}

func (t *RTCIceTransport) GetRemoteParameters() RTCIceParameters {
	return RTCIceParameters{}
}
