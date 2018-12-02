package rtcp

// Packet represents an RTCP packet, a protocol used for out-of-band statistics and control information for an RTP session
type Packet interface {
	Header() Header

	Marshal() ([]byte, error)
	Unmarshal(rawPacket []byte) error
}

// Unmarshal parses a raw RTCP byte array and returns a Packet.
//
// If the packet has an implemented type you can use a type assertion
// to access the fields of the packet. If the packet's type is unknown
// then a RawPacket will be returned instead.
func Unmarshal(rawPacket []byte) (Packet, error) {
	var h Header
	var p Packet

	err := h.Unmarshal(rawPacket)
	if err != nil {
		return nil, err
	}

	switch h.Type {
	case TypeSenderReport:
		p = new(SenderReport)

	case TypeReceiverReport:
		p = new(ReceiverReport)

	case TypeSourceDescription:
		p = new(SourceDescription)

	case TypeGoodbye:
		p = new(Goodbye)

	case TypeTransportSpecificFeedback:
		p = new(RapidResynchronizationRequest)

	case TypePayloadSpecificFeedback:
		p = new(PictureLossIndication)

	default:
		p = new(RawPacket)
	}

	err = p.Unmarshal(rawPacket)
	return p, err
}
