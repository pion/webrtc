package ivfreader

import (
	"encoding/binary"
	"fmt"
	"io"
)

const (
	ivfFileHeaderSignature = "DKIF"
	ivfFileHeaderSize      = 32
	ivfFrameHeaderSize     = 12
)

// IVFFileHeader 32-byte header for IVF files
// https://wiki.multimedia.cx/index.php/IVF
type IVFFileHeader struct {
	signature     string // 0-3
	version       uint16 // 4-5
	headerSize    uint16 // 6-7
	fourcc        string // 8-11
	width         uint16 // 12-13
	height        uint16 // 14-15
	timebaseDenum uint32 // 16-19
	timebaseNum   uint32 // 20-23
	numFrames     uint32 // 24-27
	unused        uint32 // 28-31
}

// IVFFrameHeader 12-byte header for IVF frames
// https://wiki.multimedia.cx/index.php/IVF
type IVFFrameHeader struct {
	frameSize uint32 // 0-3
	timestamp uint64 // 4-11
}

// IVFReader is used to read IVF files and return frame payloads
type IVFReader struct {
	stream io.Reader
}

// NewWith returns a new IVF reader and IVF file header
// with an io.Reader input
func NewWith(in io.Reader) (*IVFReader, *IVFFileHeader, error) {
	if in == nil {
		return nil, nil, fmt.Errorf("stream is nil")
	}

	reader := &IVFReader{
		stream: in,
	}

	header, err := reader.parseFileHeader()
	if err != nil {
		return nil, nil, err
	}

	return reader, header, nil
}

// ParseNextFrame reads from stream and returns IVF frame payload, header,
// and an error if there is incomplete frame data.
// Returns all nil values when no more frames are available.
func (i *IVFReader) ParseNextFrame() ([]byte, *IVFFrameHeader, error) {
	buffer := make([]byte, ivfFrameHeaderSize)
	var header *IVFFrameHeader

	bytesRead, err := i.stream.Read(buffer)
	if err != nil {
		return nil, nil, err
	} else if bytesRead != ivfFrameHeaderSize {
		// io.Reader.Read(n) may not return EOF err when n > 0 bytes
		// are read and instead return 0, EOF in subsequent call
		return nil, nil, fmt.Errorf("incomplete frame header")
	}

	header = &IVFFrameHeader{
		frameSize: binary.LittleEndian.Uint32(buffer[:4]),
		timestamp: binary.LittleEndian.Uint64(buffer[4:12]),
	}

	payload := make([]byte, header.frameSize)
	bytesRead, err = i.stream.Read(payload)
	if err != nil {
		return nil, nil, err
	} else if bytesRead != int(header.frameSize) {
		return nil, nil, fmt.Errorf("incomplete frame data")
	}
	return payload, header, nil
}

// parseFileHeader reads 32 bytes from stream and returns
// IVF file header. This is always called before ParseNextFrame()
func (i *IVFReader) parseFileHeader() (*IVFFileHeader, error) {
	buffer := make([]byte, ivfFileHeaderSize)

	bytesRead, err := i.stream.Read(buffer)
	if err != nil {
		return nil, err
	} else if bytesRead != ivfFileHeaderSize {
		return nil, fmt.Errorf("incomplete file header")
	}

	header := &IVFFileHeader{
		signature:     string(buffer[:4]),
		version:       binary.LittleEndian.Uint16(buffer[4:6]),
		headerSize:    binary.LittleEndian.Uint16(buffer[6:8]),
		fourcc:        string(buffer[8:12]),
		width:         binary.LittleEndian.Uint16(buffer[12:14]),
		height:        binary.LittleEndian.Uint16(buffer[14:16]),
		timebaseDenum: binary.LittleEndian.Uint32(buffer[16:20]),
		timebaseNum:   binary.LittleEndian.Uint32(buffer[20:24]),
		numFrames:     binary.LittleEndian.Uint32(buffer[24:28]),
		unused:        binary.LittleEndian.Uint32(buffer[28:32]),
	}

	if header.signature != ivfFileHeaderSignature {
		return nil, fmt.Errorf("IVF signature mismatch")
	} else if header.version != uint16(0) {
		errStr := fmt.Sprintf("IVF version unknown: %d,"+
			" parser may not parse correctly", header.version)
		return nil, fmt.Errorf(errStr)
	}

	return header, nil
}
