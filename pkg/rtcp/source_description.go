package rtcp

import (
	"encoding/binary"

	"github.com/pkg/errors"
)

// RTP SDES item types registered with IANA. See: https://www.iana.org/assignments/rtp-parameters/rtp-parameters.xhtml#rtp-parameters-5
const (
	SDESEnd      = iota // end of SDES list                RFC 3550, 6.5
	SDESCNAME           // canonical name                  RFC 3550, 6.5.1
	SDESName            // user name                       RFC 3550, 6.5.2
	SDESEmail           // user's electronic mail address  RFC 3550, 6.5.3
	SDESPhone           // user's phone number             RFC 3550, 6.5.4
	SDESLocation        // geographic user location        RFC 3550, 6.5.5
	SDESTool            // name of application or tool     RFC 3550, 6.5.6
	SDESNote            // notice about the source         RFC 3550, 6.5.7
	SDESPrivate         // private extensions              RFC 3550, 6.5.8  (not implemented)
)

var (
	errSDESTextTooLong = errors.New("session description must be < 255 octets long")
	errSDESMissingType = errors.New("session description item missing type")
)

const (
	sdesSourceLen        = 4
	sdesTypeLen          = 1
	sdesTypeOffset       = 0
	sdesOctetCountLen    = 1
	sdesOctetCountOffset = 1
	sdesMaxOctetCount    = (1 << 8) - 1
	sdesTextOffset       = 2
)

// A SourceDescription (SDES) packet describes the sources in an RTP stream.
type SourceDescription struct {
	Chunks []SourceDescriptionChunk
}

// Marshal encodes the SourceDescription in binary
func (s SourceDescription) Marshal() ([]byte, error) {
	/*
	 *        +=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+
	 * chunk  |                          SSRC/CSRC_1                          |
	 *   1    +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	 *        |                           SDES items                          |
	 *        |                              ...                              |
	 *        +=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+
	 * chunk  |                          SSRC/CSRC_2                          |
	 *   2    +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	 *        |                           SDES items                          |
	 *        |                              ...                              |
	 *        +=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+
	 */

	rawPacket := make([]byte, 0)
	for _, c := range s.Chunks {
		data, err := c.Marshal()
		if err != nil {
			return nil, err
		}
		rawPacket = append(rawPacket, data...)
	}

	return rawPacket, nil
}

// Unmarshal decodes the SourceDescription from binary
func (s *SourceDescription) Unmarshal(rawPacket []byte) error {
	/*
	 *        +=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+
	 * chunk  |                          SSRC/CSRC_1                          |
	 *   1    +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	 *        |                           SDES items                          |
	 *        |                              ...                              |
	 *        +=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+
	 * chunk  |                          SSRC/CSRC_2                          |
	 *   2    +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	 *        |                           SDES items                          |
	 *        |                              ...                              |
	 *        +=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+
	 */

	for i := 0; i < len(rawPacket); {
		var chunk SourceDescriptionChunk
		if err := chunk.Unmarshal(rawPacket[i:]); err != nil {
			return err
		}
		s.Chunks = append(s.Chunks, chunk)

		i += chunk.len()
	}

	return nil
}

// A SourceDescriptionChunk contains items describing a single RTP source
type SourceDescriptionChunk struct {
	// The source (ssrc) or contributing source (csrc) identifier this packet describes
	Source uint32
	Items  []SourceDescriptionItem
}

// Marshal encodes the SourceDescriptionChunk in binary
func (s SourceDescriptionChunk) Marshal() ([]byte, error) {
	/*
	 *  +=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+
	 *  |                          SSRC/CSRC_1                          |
	 *  +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	 *  |                           SDES items                          |
	 *  |                              ...                              |
	 *  +=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+
	 */

	rawPacket := make([]byte, sdesSourceLen)
	binary.BigEndian.PutUint32(rawPacket, s.Source)

	for _, it := range s.Items {
		data, err := it.Marshal()
		if err != nil {
			return nil, err
		}
		rawPacket = append(rawPacket, data...)
	}

	// The list of items in each chunk MUST be terminated by one or more null octets
	rawPacket = append(rawPacket, SDESEnd)

	// additional null octets MUST be included if needed to pad until the next 32-bit boundary
	if size := len(rawPacket); size%4 != 0 {
		padding := make([]byte, 4-size%4)
		rawPacket = append(rawPacket, padding...)
	}

	return rawPacket, nil
}

// Unmarshal decodes the SourceDescriptionChunk from binary
func (s *SourceDescriptionChunk) Unmarshal(rawPacket []byte) error {
	/*
	 *  +=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+
	 *  |                          SSRC/CSRC_1                          |
	 *  +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	 *  |                           SDES items                          |
	 *  |                              ...                              |
	 *  +=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+
	 */

	if len(rawPacket) < (sdesSourceLen + sdesTypeLen) {
		return errPacketTooShort
	}

	s.Source = binary.BigEndian.Uint32(rawPacket)

	for i := 4; i < len(rawPacket); {
		if pktType := rawPacket[i]; pktType == SDESEnd {
			return nil
		}

		var it SourceDescriptionItem
		if err := it.Unmarshal(rawPacket[i:]); err != nil {
			return err
		}
		s.Items = append(s.Items, it)
		i += it.len()
	}

	return errPacketTooShort
}

func (s SourceDescriptionChunk) len() int {
	len := sdesSourceLen
	for _, it := range s.Items {
		len += it.len()
	}
	len += sdesTypeLen // for terminating null octet

	// align to 32-bit boundary
	if len%4 != 0 {
		len += 4 - (len % 4)
	}

	return len
}

// A SourceDescriptionItem is a part of a SourceDescription that describes a stream.
type SourceDescriptionItem struct {
	// The type identifier for this item. eg, SDESCNAME for canonical name description.
	//
	// Type zero or SDESEnd is interpreted as the end of an item list and cannot be used.
	Type uint8
	// Text is a unicode text blob associated with the item. Its meaning varies based on the item's Type.
	Text string
}

func (s SourceDescriptionItem) len() int {
	/*
	 *   0                   1                   2                   3
	 *   0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
	 *  +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	 *  |    CNAME=1    |     length    | user and domain name        ...
	 *  +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	 */
	return sdesTypeLen + sdesOctetCountLen + len([]byte(s.Text))
}

// Marshal encodes the SourceDescriptionItem in binary
func (s SourceDescriptionItem) Marshal() ([]byte, error) {
	/*
	 *   0                   1                   2                   3
	 *   0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
	 *  +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	 *  |    CNAME=1    |     length    | user and domain name        ...
	 *  +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	 */

	if s.Type == SDESEnd {
		return nil, errSDESMissingType
	}

	rawPacket := make([]byte, sdesTypeLen+sdesOctetCountLen)

	rawPacket[sdesTypeOffset] = s.Type

	txtBytes := []byte(s.Text)
	octetCount := len(txtBytes)
	if octetCount > sdesMaxOctetCount {
		return nil, errSDESTextTooLong
	}
	rawPacket[sdesOctetCountOffset] = uint8(octetCount)

	rawPacket = append(rawPacket, txtBytes...)

	return rawPacket, nil
}

// Unmarshal decodes the SourceDescriptionItem from binary
func (s *SourceDescriptionItem) Unmarshal(rawPacket []byte) error {
	/*
	 *   0                   1                   2                   3
	 *   0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
	 *  +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	 *  |    CNAME=1    |     length    | user and domain name        ...
	 *  +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	 */

	if len(rawPacket) < (sdesTypeLen + sdesOctetCountLen) {
		return errPacketTooShort
	}

	s.Type = rawPacket[sdesTypeOffset]

	octetCount := int(rawPacket[sdesOctetCountOffset])
	if sdesTextOffset+octetCount > len(rawPacket) {
		return errPacketTooShort
	}

	txtBytes := rawPacket[sdesTextOffset : sdesTextOffset+octetCount]
	s.Text = string(txtBytes)

	return nil
}
