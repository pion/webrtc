package webrtc

import (
	"container/list"

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
	toDtls   chan []byte
	fromDtls chan []byte
}

func newRTCIceTransport(pc *RTCPeerConnection) (*RTCIceTransport, error) {
	t := &RTCIceTransport{
		conn: pc,
	}

	iceServer, err := pc.configuration.getIceServers()
	if err != nil {
		return nil, err
	}

	t.agent, err = ice.NewAgent(iceServer)
	if err != nil {
		return nil, err
	}
	t.agent.OnReceive = t.onRecieveHandler

	return t, nil
}

func (t *RTCIceTransport) dtlsHandler() {
	queue := list.New()
	for {
		if front := queue.Front(); front == nil {
			if s.writer == nil {
				close(s.Output)
				return
			}
			value, ok := <-t.fromDtls
			if !ok {
				close(s.Output)
				return
			}
			queue.PushBack(value)
		} else {
			select {
			case s.Output <- front.Value:
				queue.Remove(front)
			case value, ok := <-s.writer:
				if ok {
					queue.PushBack(value)
				} else {
					s.writer = nil
				}
			}
		}
	}

	for {
		raw, ok := <-t.fromDtls
		if !ok {
			return
		}

		// TODO do stuff here
	}
}

func (t *RTCIceTransport) onRecieveHandler(event ice.ReceiveEvent) {
	t.toDtls <- event.Buffer
	// for {
	// 	raw, ok := <-t.agent.Output
	// 	if !ok {
	// 		return
	// 	}
	// }

	// in, socketOpen := <-incomingPackets
	// if !socketOpen {
	// 	// incomingPackets channel has closed, this port is finished processing
	// 	dtls.RemoveListener(p.listeningAddr.String())
	// 	return
	// }

	// if len(buffer) == 0 {
	// 	fmt.Println("Inbound buffer is not long enough to demux")
	// 	return
	// }
	//
	// // https://tools.ietf.org/html/rfc5764#page-14
	// if 127 < buffer[0] && buffer[0] < 192 {
	// 	p.handleSRTP(buffer)
	// } else if 19 < buffer[0] && buffer[0] < 64 {
	// 	p.handleDTLS(buffer, remoteAddr.String())
	// } else if buffer[0] < 2 {
	// 	p.m.IceAgent.HandleInbound(buffer, p.listeningAddr, remoteAddr.String())
	// }
	//
	// p.m.certPairLock.RLock()
	// if !p.m.isOffer && p.m.certPair == nil {
	// 	p.m.dtlsState.DoHandshake(p.listeningAddr.String(), remoteAddr.String())
	// }
	// p.m.certPairLock.RUnlock()
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
