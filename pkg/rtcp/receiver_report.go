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

const (
	ssrcLength     = 4
	rrSSRCOffset   = headerLength
	rrReportOffset = rrSSRCOffset + ssrcLength
)

// Marshal encodes the ReceiverReport in binary
func (r ReceiverReport) Marshal() ([]byte, error) {
	/*
	 *         0                   1                   2                   3
	 *         0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
	 *        +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	 * header |V=2|P|    RC   |   PT=RR=201   |             length            |
	 *        +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	 *        |                     SSRC of packet sender                     |
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

	rawPacket := make([]byte, ssrcLength)

	binary.BigEndian.PutUint32(rawPacket, r.SSRC)

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
		Type:   TypeReceiverReport,
		Length: uint16(headerLength + len(rawPacket)),
	}
	hData, err := h.Marshal()
	if err != nil {
		return nil, err
	}

	rawPacket = append(hData, rawPacket...)

	return rawPacket, nil
}

// Unmarshal decodes the ReceiverReport from binary
func (r *ReceiverReport) Unmarshal(rawPacket []byte) error {
	/*
	 *         0                   1                   2                   3
	 *         0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
	 *        +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	 * header |V=2|P|    RC   |   PT=RR=201   |             length            |
	 *        +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	 *        |                     SSRC of packet sender                     |
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

	if len(rawPacket) < (headerLength + ssrcLength) {
		return errPacketTooShort
	}

	var h Header
	if err := h.Unmarshal(rawPacket); err != nil {
		return err
	}

	if h.Type != TypeReceiverReport {
		return errWrongType
	}

	r.SSRC = binary.BigEndian.Uint32(rawPacket[rrSSRCOffset:])

	for i := rrReportOffset; i < len(rawPacket); i += receptionReportLength {
		var rr ReceptionReport
		if err := rr.Unmarshal(rawPacket[i:]); err != nil {
			return err
		}
		r.Reports = append(r.Reports, rr)
	}

	if uint8(len(r.Reports)) != h.Count {
		return errInvalidHeader
	}

	return nil
}
