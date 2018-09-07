package sctp

type ReceiveEvent struct {
	Buffer            []byte
	StreamID          uint16
	PayloadProtocolID PayloadProtocolID
}

type CommunicationUpEvent struct {
	outboundStreamCount uint16
}
