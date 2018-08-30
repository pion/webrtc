package webrtc

// RTCRtpTransceiverDirection indicates the direction of the RTCRtpTransceiver.
type RTCRtpTransceiverDirection int

const (
	// RTCRtpTransceiverDirectionSendrecv indicates the RTCRtpSender will offer
	// to send RTP and RTCRtpReceiver the will offer to receive RTP.
	RTCRtpTransceiverDirectionSendrecv RTCRtpTransceiverDirection = iota + 1

	// RTCRtpTransceiverDirectionSendonly indicates the RTCRtpSender will offer
	// to send RTP.
	RTCRtpTransceiverDirectionSendonly

	// RTCRtpTransceiverDirectionRecvonly indicates the RTCRtpReceiver the will
	// offer to receive RTP.
	RTCRtpTransceiverDirectionRecvonly

	// RTCRtpTransceiverDirectionInactive indicates the RTCRtpSender won't offer
	// to send RTP and RTCRtpReceiver the won't offer to receive RTP.
	RTCRtpTransceiverDirectionInactive
)

// This is done this way because of a linter.
const (
	rtcRtpTransceiverDirectionSendrecvStr = "sendrecv"
	rtcRtpTransceiverDirectionSendonlyStr = "sendonly"
	rtcRtpTransceiverDirectionRecvonlyStr = "recvonly"
	rtcRtpTransceiverDirectionInactiveStr = "inactive"
)

// NewRTCRtpTransceiverDirection defines a procedure for creating a new
// RTCRtpTransceiverDirection from a raw string naming the transceiver direction.
func NewRTCRtpTransceiverDirection(raw string) RTCRtpTransceiverDirection {
	switch raw {
	case rtcRtpTransceiverDirectionSendrecvStr:
		return RTCRtpTransceiverDirectionSendrecv
	case rtcRtpTransceiverDirectionSendonlyStr:
		return RTCRtpTransceiverDirectionSendonly
	case rtcRtpTransceiverDirectionRecvonlyStr:
		return RTCRtpTransceiverDirectionRecvonly
	case rtcRtpTransceiverDirectionInactiveStr:
		return RTCRtpTransceiverDirectionInactive
	default:
		return RTCRtpTransceiverDirection(Unknown)
	}
}

func (t RTCRtpTransceiverDirection) String() string {
	switch t {
	case RTCRtpTransceiverDirectionSendrecv:
		return rtcRtpTransceiverDirectionSendrecvStr
	case RTCRtpTransceiverDirectionSendonly:
		return rtcRtpTransceiverDirectionSendonlyStr
	case RTCRtpTransceiverDirectionRecvonly:
		return rtcRtpTransceiverDirectionRecvonlyStr
	case RTCRtpTransceiverDirectionInactive:
		return rtcRtpTransceiverDirectionInactiveStr
	default:
		return ErrUnknownType.Error()
	}
}
