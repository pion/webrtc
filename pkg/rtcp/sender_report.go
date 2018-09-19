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
	srHeaderLength      = 24
	srSSRCOffset        = 0
	srNTPOffset         = srSSRCOffset + ssrcLength
	ntpTimeLength       = 8
	srRTPOffset         = srNTPOffset + ntpTimeLength
	rtpTimeLength       = 4
	srPacketCountOffset = srRTPOffset + rtpTimeLength
	srPacketCountLength = 4
	srOctetCountOffset  = srPacketCountOffset + srPacketCountLength
	srOctetCountLength  = 4
	srReportOffset      = srOctetCountOffset + srOctetCountLength
)

// Marshal encodes the SenderReport in binary
func (r SenderReport) Marshal() ([]byte, error) {
	/*
	 *         0                   1                   2                   3
	 *         0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
	 *        +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	 * header |V=2|P|    RC   |   PT=SR=200   |             length            |
	 *        +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	 *        |                         SSRC of sender                        |
	 *        +=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+
	 * sender |              NTP timestamp, most significant word             |
	 * info   +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	 *        |             NTP timestamp, least significant word             |
	 *        +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	 *        |                         RTP timestamp                         |
	 *        +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	 *        |                     sender's packet count                     |
	 *        +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	 *        |                      sender's octet count                     |
	 *        +=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+
	 * report |                 SSRC_1 (SSRC of first source)                 |
	 * block  +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	 *   1    | fraction lost |       cumulative number of packets lost       |
	 *        +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	 *        |           extended highest sequence number received           |
	 *        +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	 *        |                      interarrival jitter                      |
	 *        +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	 *        |                         last SR (LSR)                         |
	 *        +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	 *        |                   delay since last SR (DLSR)                  |
	 *        +=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+
	 * report |                 SSRC_2 (SSRC of second source)                |
	 * block  +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	 *   2    :                               ...                             :
	 *        +=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+
	 *        |                  profile-specific extensions                  |
	 *        +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	 */

	rawPacket := make([]byte, srHeaderLength)

	binary.BigEndian.PutUint32(rawPacket[srSSRCOffset:], r.SSRC)
	binary.BigEndian.PutUint64(rawPacket[srNTPOffset:], r.NTPTime)
	binary.BigEndian.PutUint32(rawPacket[srRTPOffset:], r.RTPTime)
	binary.BigEndian.PutUint32(rawPacket[srPacketCountOffset:], r.PacketCount)
	binary.BigEndian.PutUint32(rawPacket[srOctetCountOffset:], r.OctetCount)

	for _, rp := range r.Reports {
		data, err := rp.Marshal()
		if err != nil {
			return nil, err
		}
		rawPacket = append(rawPacket, data...)
	}

	if len(r.Reports) > countMax {
		return nil, errTooManyReports
	}

	h := Header{
		Count:  uint8(len(r.Reports)),
		Type:   TypeSenderReport,
		Length: uint16(headerLength + len(rawPacket)),
	}
	hData, err := h.Marshal()
	if err != nil {
		return nil, err
	}

	rawPacket = append(hData, rawPacket...)

	return rawPacket, nil
}

// Unmarshal decodes the SenderReport from binary
func (r *SenderReport) Unmarshal(rawPacket []byte) error {
	/*
	 *         0                   1                   2                   3
	 *         0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
	 *        +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	 * header |V=2|P|    RC   |   PT=SR=200   |             length            |
	 *        +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	 *        |                         SSRC of sender                        |
	 *        +=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+
	 * sender |              NTP timestamp, most significant word             |
	 * info   +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	 *        |             NTP timestamp, least significant word             |
	 *        +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	 *        |                         RTP timestamp                         |
	 *        +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	 *        |                     sender's packet count                     |
	 *        +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	 *        |                      sender's octet count                     |
	 *        +=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+
	 * report |                 SSRC_1 (SSRC of first source)                 |
	 * block  +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	 *   1    | fraction lost |       cumulative number of packets lost       |
	 *        +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	 *        |           extended highest sequence number received           |
	 *        +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	 *        |                      interarrival jitter                      |
	 *        +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	 *        |                         last SR (LSR)                         |
	 *        +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	 *        |                   delay since last SR (DLSR)                  |
	 *        +=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+
	 * report |                 SSRC_2 (SSRC of second source)                |
	 * block  +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	 *   2    :                               ...                             :
	 *        +=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+
	 *        |                  profile-specific extensions                  |
	 *        +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	 */

	if len(rawPacket) < (headerLength + srHeaderLength) {
		return errPacketTooShort
	}

	var h Header
	if err := h.Unmarshal(rawPacket); err != nil {
		return err
	}

	if h.Type != TypeSenderReport {
		return errWrongType
	}

	packetBody := rawPacket[headerLength:]

	r.SSRC = binary.BigEndian.Uint32(packetBody[srSSRCOffset:])
	r.NTPTime = binary.BigEndian.Uint64(packetBody[srNTPOffset:])
	r.RTPTime = binary.BigEndian.Uint32(packetBody[srRTPOffset:])
	r.PacketCount = binary.BigEndian.Uint32(packetBody[srPacketCountOffset:])
	r.OctetCount = binary.BigEndian.Uint32(packetBody[srOctetCountOffset:])

	for i := srReportOffset; i < len(packetBody); i += receptionReportLength {
		var rr ReceptionReport
		if err := rr.Unmarshal(packetBody[i:]); err != nil {
			return err
		}
		r.Reports = append(r.Reports, rr)
	}

	if uint8(len(r.Reports)) != h.Count {
		return errInvalidHeader
	}

	return nil
}
