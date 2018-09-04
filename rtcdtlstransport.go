package webrtc

// RTCDtlsTransport allows an application access to information about the DTLS
// transport over which RTP and RTCP packets are sent and received by
// RTCRtpSender and RTCRtpReceiver, as well other data such as SCTP packets sent
// and received by data channels.
type RTCDtlsTransport struct {
	// Transport RTCIceTransport
	// State     RTCDtlsTransportState

	// OnStateChange func()
	// OnError       func()
}
