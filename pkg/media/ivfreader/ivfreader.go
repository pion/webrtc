package ivfreader

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"github.com/pion/rtp"
)

// TODO: Description
// TODO: Formatting, move to separate file
type IVFHeader struct {
	signature string // 0-3
	version uint16 // 4-5
	header_size uint16 // 6-7
	fourcc string // 8-11
	width uint16 // 12-13
	height uint16 // 14-15
	timebase_denum uint32 // 16-19
	timebase_num uint32 // 20-23
	num_frames uint32 // 24-27
	unused uint32 // 28-31
}

// IVFReader is used to read RTP packets
// TODO: Formatting
type IVFReader struct {
	stream io.Reader
	fd *os.File
	count uint64
	currentFrame []byte
}

// NewWith initialize a new IVF reader with an io.Reader input
func NewWith(in io.Reader) (*IVFReader, error) {
	if in == nil {
		return nil, fmt.Errorf("stream is nil")
	}

	reader := &IVFReader{
		stream: in,
	}
	return reader, nil
}

func (i *IVFReader) ParseNextFrame(h *IVFHeader) (*rtp.Packet, error) {
	// TODO
	return nil, nil
}

func (i *IVFReader) ParseFileHeader() (*IVFHeader, error) {
	buffer := make([]byte, 32)

	header := &IVFHeader{}

	bytes_read, err := i.stream.Read(buffer)
	if err != nil {
		return nil, err
	} else if bytes_read != 32 {
		return nil, nil // TODO: Throw error
	}

	header.signature = string(buffer[:4])
	header.version = binary.LittleEndian.Uint16(buffer[4:6])
	header.header_size = binary.LittleEndian.Uint16(buffer[6:8])
	header.fourcc = string(buffer[8:12])
	header.width = binary.LittleEndian.Uint16(buffer[12:14])
	header.height = binary.LittleEndian.Uint16(buffer[14:16])
	header.timebase_denum = binary.LittleEndian.Uint32(buffer[16:20])
	header.timebase_num = binary.LittleEndian.Uint32(buffer[20:24])
	header.num_frames = binary.LittleEndian.Uint32(buffer[24:28])
	header.unused = binary.LittleEndian.Uint32(buffer[28:32])

	return header, nil
}
