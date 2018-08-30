package webrtc

// RTCRtpSender allows an application to control how a given RTCTrack is encoded and transmitted to a remote peer
type RTCRtpSender struct {
	Track *RTCTrack
	// senderTrack *RTCTrack
	// senderTransport
	// senderRtcpTransport
}

func newRTCRtpSender(track *RTCTrack) *RTCRtpSender {
	s := &RTCRtpSender{
		Track: track,
	}
	return s
}
