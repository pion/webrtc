package rtcp

// Packet represents an RTCP packet, a protocol used for out-of-band statistics and control information for an RTP session
type Packet interface {
	Header() Header
	// DestinationSSRC returns an array of SSRC values that this packet refers to.
	DestinationSSRC() []uint32

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
		switch h.Count {
		case FormatTLN:
			p = new(TransportLayerNack)
		case FormatRRR:
			p = new(RapidResynchronizationRequest)
		default:
			p = new(RawPacket)
		}

	case TypePayloadSpecificFeedback:
		switch h.Count {
		case FormatPLI:
			p = new(PictureLossIndication)
		case FormatSLI:
			p = new(SliceLossIndication)
		default:
			p = new(RawPacket)
		}

	default:
		p = new(RawPacket)
	}

	err = p.Unmarshal(rawPacket)
	return p, h, err
}
