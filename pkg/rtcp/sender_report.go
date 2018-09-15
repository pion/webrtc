package rtcp

import "encoding/binary"

// A SenderReport (SR) packet provides reception quality feedback for an RTP stream
type SenderReport struct {
	// The synchronization source identifier for the originator of this SR packet.
	SSRC uint32
	// The wallclock time when this report was sent so that it may be used in
	// combination with timestamps returned in reception reports from other
	// receivers to measure round-trip propagation to those receivers.
	NTPTime uint64
	// Corresponds to the same time as the NTP timestamp (above), but in
	// the same units and with the same random offset as the RTP
	// timestamps in data packets. This correspondence may be used for
	// intra- and inter-media synchronization for sources whose NTP
	// timestamps are synchronized, and may be used by media-independent
	// receivers to estimate the nominal RTP clock frequency.
	RTPTime uint32
	// The total number of RTP data packets transmitted by the sender
	// since starting transmission up until the time this SR packet was
	// generated.
	PacketCount uint32
	// The total number of payload octets (i.e., not including header or
	// padding) transmitted in RTP data packets by the sender since
	// starting transmission up until the time this SR packet was
	// generated.
	OctetCount uint32
	// Zero or more reception report blocks depending on the number of other
	// sources heard by this sender since the last report. Each reception report
	// block conveys statistics on the reception of RTP packets from a
	// single synchronization source.
	Reports []ReceptionReport
}

var (
	senderReportLength = 24
	ntpTimeOffset      = 4
	rtpTimeOffset      = 12
	packetCountOffset  = 16
	octetCountOffset   = 20
)

// Marshal encodes the SenderReport in binary
func (r SenderReport) Marshal() ([]byte, error) {
	rawPacket := make([]byte, senderReportLength)

	binary.BigEndian.PutUint32(rawPacket, r.SSRC)
	binary.BigEndian.PutUint64(rawPacket[ntpTimeOffset:], r.NTPTime)
	binary.BigEndian.PutUint32(rawPacket[rtpTimeOffset:], r.RTPTime)
	binary.BigEndian.PutUint32(rawPacket[packetCountOffset:], r.PacketCount)
	binary.BigEndian.PutUint32(rawPacket[octetCountOffset:], r.OctetCount)

	for _, rp := range r.Reports {
		data, err := rp.Marshal()
		if err != nil {
			return nil, err
		}
		rawPacket = append(rawPacket, data...)
	}

	return rawPacket, nil
}

// Unmarshal decodes the SenderReport from binary
func (r *SenderReport) Unmarshal(rawPacket []byte) error {
	if len(rawPacket) < senderReportLength {
		return errPacketTooShort
	}

	r.SSRC = binary.BigEndian.Uint32(rawPacket)
	r.NTPTime = binary.BigEndian.Uint64(rawPacket[ntpTimeOffset:])
	r.RTPTime = binary.BigEndian.Uint32(rawPacket[rtpTimeOffset:])
	r.PacketCount = binary.BigEndian.Uint32(rawPacket[packetCountOffset:])
	r.OctetCount = binary.BigEndian.Uint32(rawPacket[octetCountOffset:])

	for i := senderReportLength; i < len(rawPacket); i += receptionReportLength {
		var rr ReceptionReport
		if err := rr.Unmarshal(rawPacket[i:]); err != nil {
			return err
		}
		r.Reports = append(r.Reports, rr)
	}

	return nil
}
