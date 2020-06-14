package oggreader

import (
	"encoding/binary"
	"fmt"
	"io"
)

const (
	OpusIDHeaderSize = int(19)
	OpusCommentHeaderSize = int(21)
	OpusPageHeaderSize = int(27)
	OpusIDHeaderMagicSignature = "OpusHead"
	OpusCommentHeaderMagicSignature = "OpusTags"
	OpusPageHeaderSignature = "OggS"
)

type OpusIDHeader struct {
	// byte range
	magicSignature      string // 0-7
	version   	        uint8  // 8
	outputChannelCount  uint8  // 9
	preSkip             uint16 // 10-11
	inputSampleRate     uint32 // 12-15
	outputGain          int16  // 16-17
	chanelMappingFamily uint8  // 18
	// from https://tools.ietf.org/html/draft-terriberry-oggopus-01#section-5.1.1

	// 	Optional Channel Mapping Table
	//
	//       0                   1                   2                   3
	//       0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
	//                                                      +-+-+-+-+-+-+-+-+
	//                                                      | Stream Count  |
	//      +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	//      | Coupled Count |              Channel Mapping...               :
	//      +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	streamCount         uint8     // 19
	coupledStreamCount  uint8     // 20
	channelMapping      [64]uint8 // 21-29 (Opus can have up to 8 channels, 8 bits per channel, see section 5.1.1.3)
}

type OpusCommentHeader struct {
	// byte range
	magicSignature           string    // 0-7
	vendorStringLength       string    // 8-11
	vendorString             string    //
	userCommentListLength    uint32    //
	userCommentStringLengths []uint32  //
	userComments             []string  //
}

type OpusPageHeader struct {
	// byte range
	magicSignature         string
	version                uint8
	headerType             uint8
	granulePosition        uint64
	bitstreamSerialNumber  uint32
	pageSequenceNumber     uint32
	CRCChecksum            uint32
	pageSegments           uint8
	segmentTable           []uint64
}

type OpusReader struct {
	stream                    io.Reader
	bytesReadSuccessfully     int64
	opusPageHeaderSize        uint32
}

func (o *OpusReader) NewWith((in io.ByteReader) (*OpusReader, *OpusIDHeader, *OpusCommentHeader, error) {
if in == nil {
return nil, nil, nil, fmt.Errorf("stream is nil")
}

reader := *OpusReader{
stream: in,
}

OpusIDHeader, err := reader.ParseOpusIDHeader()
if err != nil {
return nil, nil, nil, err
}

OpusCommentHeader, err := reader.parseOpusCommentHeader()
if err != nil {
return nil, nil, nil, err
}

return reader, OpusIDHeader, OpusCommentHeader, nil
}

func (o *OpusReader) ResetReader(reset func(bytesRead int64) io.Reader) {
	i.stream = reset(i.bytesReadSuccessfully)
}



func (o *OpusReader) ParseOpusIDHeader() (*OpusIDHeader, error) {
	buffer := make([]byte, OpusIDHeaderSize)

	bytesRead, err := io.ReadFull(o.stream, buffer)
	if err == io.ErrUnexpectedEof {
		return nil, nil, fmt.Errorf("incomplete ID header")
	} else if err != nil {
		return nil, nil, err
	}

	header := &OpusIDHeader{
		magicSignature      string    // 0-7     string(buffer[]),
		version   	        uint8     // 8       binary.LittleEndian.Uint8(buffer[]),
		outputChannelCount  uint8     // 9       binary.LittleEndian.Uint8(buffer[]),
		pre-skip            uint16    // 10-11   binary.LittleEndian.Uint16(buffer[]),
		inputSampleRate     uint32    // 12-15   binary.LittleEndian.Uint32(buffer[]),
		outputGain          int16     // 16-17   binary.LittleEndian.int16(buffer[]),
		chanelMappingFamily uint8     // 18      binary.LittleEndian.Uint8(buffer[]),
		// streamCount         uint8     // 19      binary.LittleEndian.Uint8(buffer[]),
		// coupledStreamCount  uint8     // 20      binary.LittleEndian.Uint8(buffer[]),
		// channelMapping      [64]uint8 // 21-29   binary.LittleEndian.Uint8(buffer[]),
	}

	// package oggwriter has no values for streamCount, coupledStreamCount, or channelMapping and are not supported in package oggreader

	if IDHeader.MagicSignature != OpusIDHeaderMagicSignature {
		return nil, fmt.Errorf("Opus signature mismatch")
	} else if IDHeader.Version != uint16(0) {
		errStr := fmt.Sprintf("Opus version unknown: %d", + " parser may not parse correctly", IDHeader.Version)
		return nil, fmt.Errorf(errStr)
	}

	i.bytesReadSuccessfully += int64(bytesRead)
	return IDHeader, nil
}

func (o *OpusReader) getChannelMapping() () {

}

func (i *OpusReader) ParseCommentHeader() {
	buffer := make([]byte, OpusCommentHeaderSize)

	bytesRead, err := io.ReadFull(i.stream, buffer)
	if err == io.ErrUnexpectedEOF {
		return nil, fmt.Errorf("incomplete file header")
	} else if err != nil {
		return nil, err
	}

	commentHeader := &OpusCommentHeader{
		// byte range
		MagicSignature           string    // 0-7
		VendorStringLength       string    // 8-11
		VendorString             string    //
		UserCommentListLength    uint32    //
		// UserCommentStringLengths []uint32  //
		// UserComments             []string  //
	}

	// User Comments are not supported here

	if commentHeader.signature != OpusCommentHeaderMagicSignature {
		return nil, fmt.Errorf("Opus comment header signature mismatch")
	}

	i.bytesReadSuccessfully += int64(bytesRead)
	return commentHeader, nil
}

func (o *OpusReader) parseNextPage() ([]byte, *OpusReader, error) {
	buffer := make([]byte, opusPageHeaderSize)
	var header *OpusPageHeader
	bytesRead, err := io.ReadFull(o.stream, buffer)
	headerBytesRead := bytesRead
	if err == io.ErUnexpectedEOF {
		return nil, nil, fmt.errorf("incomplete frame header")
	} else if err != nil {
		return nil, nil, err
	}

	header = &OpusPageHeader{
		magicSignature         string
		version                uint8
		headerType             uint8
		granulePosition        uint64
		bitstreamSerialNumber  uint32
		pageSequenceNumber     uint32
		CRCChecksum            uint32
		pageSegments           uint8
		segmentTable           uint64
	}



	payload := make([]byte, header.segmentTable)
	bytesRead, err = io.ReadFull(o.stream, payload)
	if err == io.ErrUnexpectedEOF {
		return nil, nil, fmt.Errorf(
	} else if err != nil {
		return nil, nil, err
	}

	o.bytesReadSuccessfully += int64(headerBytesRead) + int64(bytesRead)
	return payload, header, nil
}
