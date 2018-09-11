package rtcp

import "encoding/binary"

// A ReceiverReport (RR) packet provides reception quality feedback for an RTP stream
type ReceiverReport struct {
	// The synchronization source identifier for the originator of this RR packet.
	SSRC uint32
	// Zero or more reception report blocks depending on the number of other
	// sources heard by this sender since the last report. Each reception report
	// block conveys statistics on the reception of RTP packets from a
	// single synchronization source.
	Reports []ReceptionReport
}

// Marshal encodes the ReceiverReport in binary
func (r ReceiverReport) Marshal() ([]byte, error) {
	rawPacket := make([]byte, 4)

	binary.BigEndian.PutUint32(rawPacket, r.SSRC)

	for _, rp := range r.Reports {
		data, err := rp.Marshal()
		if err != nil {
			return nil, err
		}
		rawPacket = append(rawPacket, data...)
	}

	return rawPacket, nil
}

// Unmarshal decodes the ReceiverReport from binary
func (r *ReceiverReport) Unmarshal(rawPacket []byte) error {
	if len(rawPacket) < 4 {
		return errPacketTooShort
	}

	r.SSRC = binary.BigEndian.Uint32(rawPacket)

	for i := 4; i < len(rawPacket); i += receptionReportLength {
		var rr ReceptionReport
		if err := rr.Unmarshal(rawPacket[i:]); err != nil {
			return err
		}
		r.Reports = append(r.Reports, rr)
	}

	return nil
}
