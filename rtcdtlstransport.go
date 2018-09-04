package webrtc

type RTCDtlsTransport struct {
	Transport RTCIceTransport
	State     RTCDtlsTransportState

	OnStateChange func()
	OnError       func()
}
