package webrtc

// RTCPeerConnectionState indicates the state of the peer connection
type RTCPeerConnectionState int

const (
	// RTCPeerConnectionStateNew indicates some of the ICE or DTLS transports are in status new
	RTCPeerConnectionStateNew RTCPeerConnectionState = iota + 1

	// RTCPeerConnectionStateConnecting indicates some of the ICE or DTLS transports are in status connecting or checking
	RTCPeerConnectionStateConnecting

	// RTCPeerConnectionStateConnected indicates all of the ICE or DTLS transports are in status connected or completed
	RTCPeerConnectionStateConnected

	// RTCPeerConnectionStateDisconnected indicates some of the ICE or DTLS transports are in status disconnected
	RTCPeerConnectionStateDisconnected

	// RTCPeerConnectionStateFailed indicates some of the ICE or DTLS transports are in status failed
	RTCPeerConnectionStateFailed

	// RTCPeerConnectionStateClosed indicates the peer connection is closed
	RTCPeerConnectionStateClosed
)

func (t RTCPeerConnectionState) String() string {
	switch t {
	case RTCPeerConnectionStateNew:
		return "new"
	case RTCPeerConnectionStateConnecting:
		return "connecting"
	case RTCPeerConnectionStateConnected:
		return "connected"
	case RTCPeerConnectionStateDisconnected:
		return "disconnected"
	case RTCPeerConnectionStateFailed:
		return "failed"
	case RTCPeerConnectionStateClosed:
		// goconst, "closed" is used in different unrelated packages
		const closed = "closed"
		return closed
	default:
		return ErrUnknownType.Error()
	}
}
