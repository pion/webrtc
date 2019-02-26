package media

import (
	"encoding/binary"
	"fmt"
	"math"
	"math/rand"
	"os"
	"time"

	"github.com/pions/rtp"
	"github.com/pions/rtp/codecs"
)

// OpusWriter is used to take RTP packets and write them to an OGG on disk
type OpusWriter struct {
	fd                      *os.File
	sampleRate              uint32
	channelCount            uint16
	serial                  uint32
	pageIndex               uint32
	checksumTable           []int
	previousGranulePosition uint64
	previousTimestamp       uint32
}

/*
    ref: https://tools.ietf.org/html/rfc7845.html
    https://git.xiph.org/?p=opus-tools.git;a=blob;f=src/opus_header.c#l219

       Page 0         Pages 1 ... n        Pages (n+1) ...
    +------------+ +---+ +---+ ... +---+ +-----------+ +---------+ +--
    |            | |   | |   |     |   | |           | |         | |
    |+----------+| |+-----------------+| |+-------------------+ +-----
    |||ID Header|| ||  Comment Header || ||Audio Data Packet 1| | ...
    |+----------+| |+-----------------+| |+-------------------+ +-----
    |            | |   | |   |     |   | |           | |         | |
    +------------+ +---+ +---+ ... +---+ +-----------+ +---------+ +--
    ^      ^                           ^
    |      |                           |
    |      |                           Mandatory Page Break
    |      |
    |      ID header is contained on a single page
    |
    'Beginning Of Stream'

   Figure 1: Example Packet Organization for a Logical Ogg Opus Stream
*/

// NewOpusWriter builds a new OGG Opus writer
func NewOpusWriter(fileName string, sampleRate uint32, channelCount uint16) (*OpusWriter, error) {
	// Since the seed and the creation date of the
	// file will be really close, is this an issue ?
	r := rand.NewSource(time.Now().UnixNano())
	r1 := rand.New(r)
	serial := uint32(math.Ceil(r1.Float64() * math.Pow(2, 32)))
	writer := &OpusWriter{
		sampleRate:    sampleRate,
		channelCount:  channelCount,
		serial:        serial,
		checksumTable: make([]int, 256),
	}
	if err := writer.open(fileName); err != nil {
		return nil, err
	}
	writer.initChecksum()

	// ID Header
	oggIDHeader := make([]byte, 19)

	copy(oggIDHeader[0:], []byte("OpusHead"))                   // Magic Signature 'OpusHead'
	oggIDHeader[8] = 1                                          // Version
	oggIDHeader[9] = uint8(channelCount)                        // Channel count
	binary.LittleEndian.PutUint16(oggIDHeader[10:], 0)          // pre-skip, don't need to skip any value
	binary.LittleEndian.PutUint32(oggIDHeader[12:], sampleRate) // original sample rate, any valid sample e.g 48000
	binary.LittleEndian.PutUint16(oggIDHeader[16:], 0)          // output gain
	oggIDHeader[18] = 0                                         // channel map 0 = one stream: mono or stereo

	// Reference: https://tools.ietf.org/html/rfc7845.html#page-6
	// RFC specifies that the ID Header page should have a granule position of 0 and a Header Type set to 2 (StartOfStream)
	data := writer.createPage(oggIDHeader, 2, 0)
	if _, err := writer.fd.Write(data); err != nil {
		return nil, err
	}

	// Comment Header
	oggCommentHeader := make([]byte, 21)
	copy(oggCommentHeader[0:], []byte("OpusTags"))          // Magic Signature 'OpusTags'
	binary.LittleEndian.PutUint32(oggCommentHeader[8:], 5)  // Vendor Length
	copy(oggCommentHeader[12:], []byte("pions"))            // Vendor name 'pions'
	binary.LittleEndian.PutUint32(oggCommentHeader[17:], 0) // User Comment List Length

	// RFC specifies that the page where the CommentHeader completes should have a granule position of 0
	data = writer.createPage(oggCommentHeader, 0, 0)
	if _, err := writer.fd.Write(data); err != nil {
		return nil, err
	}

	return writer, nil
}

func (i *OpusWriter) open(fileName string) error {
	if i.fd != nil {
		return fmt.Errorf("File already opened")
	}

	f, err := os.Create(fileName)
	if err != nil {
		return err
	}
	i.fd = f
	return nil
}

// CRC32
func (i *OpusWriter) initChecksum() {
	for ndx := range i.checksumTable {
		r := ndx << 24
		for i := 0; i < 8; i++ {
			if (r & 0x80000000) != 0 {
				r = (r << 1) ^ 0x04c11db7
			} else {
				r <<= 1
			}
		}
		i.checksumTable[ndx] = r & 0xffffffff
	}
}

func (i *OpusWriter) getChecksum(payload []uint8) uint32 {
	var checksum uint32
	for ndx := range payload {
		tableNdx := uint8(((checksum >> 24) & 0xff)) ^ payload[ndx]
		checksum = (checksum << 8) ^ uint32(i.checksumTable[tableNdx])
	}
	return checksum
}

const (
	pageHeaderSize = 27
)

func (i *OpusWriter) createPage(payload []uint8, headerType uint8, granulePos uint64) []byte {
	payloadLen := len(payload)
	page := make([]byte, pageHeaderSize+1+payloadLen)

	copy(page[0:], []byte("OggS"))                        // page headers starts with 'OggS'
	page[4] = 0                                           // Version
	page[5] = headerType                                  // 1 = continuation, 2 = beginning of stream, 4 = end of stream
	binary.LittleEndian.PutUint64(page[6:], granulePos)   // granule position
	binary.LittleEndian.PutUint32(page[14:], i.serial)    // Bitstream serial number
	binary.LittleEndian.PutUint32(page[18:], i.pageIndex) // Page sequence number
	i.pageIndex++
	page[26] = 1                                                     // Number of segments in page, giving always 1 segment
	page[27] = uint8(payloadLen)                                     // Segment Table inserting at 27th position since page header length is 27
	copy(page[28:], payload)                                         // inserting at 28th since Segment Table(1) + header length(27)
	binary.LittleEndian.PutUint32(page[22:], i.getChecksum(payload)) // Checksum - generating for page data and inserting at 22th position into 32 bits
	return page
}

// AddPacket adds a new packet and writes the appropriate headers for it
func (i *OpusWriter) AddPacket(packet *rtp.Packet) error {
	if i.fd == nil {
		return fmt.Errorf("File not opened")
	}
	opusPacket := codecs.OpusPacket{}
	_, err := opusPacket.Unmarshal(packet)
	if err != nil {
		// Only handle Opus packets
		return err
	}

	payload := opusPacket.Payload[0:]

	// Should be equivalent to sampleRate * duration
	if i.previousTimestamp != 0 {
		increment := packet.Timestamp - i.previousTimestamp
		i.previousGranulePosition += uint64(increment)
	}
	i.previousTimestamp = packet.Timestamp

	data := i.createPage(payload, 0, i.previousGranulePosition)

	_, err = i.fd.Write(data)
	return err
}

// Close stops the recording
func (i *OpusWriter) Close() error {
	if i.fd == nil {
		return fmt.Errorf("File not opened")
	}

	// RFC specifies that the last page should have a Header Type set to 4 (EndOfStream)
	// The granule position here is the magic value '-1'
	data := i.createPage(make([]uint8, 0), 4, 0xFFFFFFFFFFFFFFFF)
	if _, err := i.fd.Write(data); err != nil {
		if e2 := i.fd.Close(); e2 != nil {
			return fmt.Errorf("Error writing file (%v); error deleting file (%v)", err, e2)
		}
		return err
	}
	return i.fd.Close()
}
