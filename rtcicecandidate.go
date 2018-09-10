package webrtc

type RTCIceCandidate struct {
	candidate        string
	sdpMid           *string
	sdpMLineIndex    *uint16
	foundation       *string
	component        *RTCIceComponent
	priority         *uint64
	ip               *string
	protocol         *RTCIceProtocol
	port             *uint16
	CandidateType    *RTCIceCandidateType
	tcpType          *RTCIceTcpCandidateType
	relatedAddress   *string
	relatedPort      *uint16
	usernameFragment *string
}
