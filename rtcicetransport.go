package webrtc

import (
	"github.com/pions/webrtc/pkg/ice"
)

// RTCIceTransport allows an application access to information about the ICE
// transport over which packets are sent and received.
type RTCIceTransport struct {
	// Role RTCIceRole
	// Component RTCIceComponent
	// State RTCIceTransportState
	// gatheringState RTCIceGathererState

	agent *ice.Agent
	conn  *RTCPeerConnection
}

func newRTCIceTransport(connection *RTCPeerConnection) *RTCIceTransport {
	t := &RTCIceTransport{
		conn: connection,
	}

	return t
}

// func (t *RTCIceTransport) GetLocalCandidates() []RTCIceCandidate {
//
// }
//
// func (t *RTCIceTransport) GetRemoteCandidates() []RTCIceCandidate {
//
// }

func (t *RTCIceTransport) GetSelectedCandidatePair() RTCIceCandidatePair {

}

// func (t *RTCIceTransport) GetLocalParameters() RTCIceParameters {
//
// }
//
// func (t *RTCIceTransport) GetRemoteParameters() RTCIceParameters {
//
// }
