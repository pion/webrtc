package rtcp

// Packet represents an RTCP packet, a protocol used for out-of-band statistics and control information for an RTP session
type Packet interface {
	Marshal() ([]byte, error)
	Unmarshal(rawPacket []byte) error
}

// Unmarshal is a factory a polymorphic RTCP packet, and its header,
func Unmarshal(rawPacket []byte) (Packet, Header, error) {
	var h Header
	var p Packet

	err := h.Unmarshal(rawPacket)
	if err != nil {
		return nil, h, err
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
	return p, h, err
}
