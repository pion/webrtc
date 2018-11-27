package rtcp

// Packet represents an RTCP packet, a protocol used for out-of-band statistics and control information for an RTP session
type Packet interface {
	Marshal() ([]byte, error)
	Unmarshal(rawPacket []byte) error
}

// PacketWithHeader is a pair to represent an RTCP header and it's
// packet's polymorphic parsed and unparsed forms.
type PacketWithHeader struct {
	Header
	Packet
	RawPacket []byte
}

//Marshal a PakcetWithHeader to a bytearray
func (p PacketWithHeader) Marshal() ([]byte, error) {
	return p.Packet.Marshal()
}

//Unmarshal a bytearray to a header-packet pair
func (p *PacketWithHeader) Unmarshal(rawPacket []byte) error {

	p.RawPacket = rawPacket

	if err := p.Header.Unmarshal(rawPacket); err != nil {
		return err
	}

	switch p.Header.Type {
	case TypeSenderReport:
		sr := new(SenderReport)
		err := sr.Unmarshal(rawPacket)
		if err == nil {
			p.Packet = sr
		} else {
			return err
		}
	case TypeReceiverReport:
		rr := new(ReceiverReport)
		err := rr.Unmarshal(rawPacket)
		if err == nil {
			p.Packet = rr
		} else {
			return err
		}
	case TypeSourceDescription:
		sdes := new(SourceDescription)
		err := sdes.Unmarshal(rawPacket)
		if err == nil {
			p.Packet = sdes
		} else {
			return err
		}
	case TypeGoodbye:
		bye := new(Goodbye)
		err := bye.Unmarshal(rawPacket)
		if err == nil {
			p.Packet = bye
		} else {
			return err
		}
	case TypeTransportSpecificFeedback:
		rrr := new(RapidResynchronizationRequest)
		err := rrr.Unmarshal(rawPacket)
		if err == nil {
			p.Packet = rrr
		} else {
			return err
		}
	case TypePayloadSpecificFeedback:
		psfb := new(PictureLossIndication)
		err := psfb.Unmarshal(rawPacket)
		if err == nil {
			p.Packet = psfb
		} else {
			return err
		}
	default:
		return errWrongType
	}

	return nil
}
