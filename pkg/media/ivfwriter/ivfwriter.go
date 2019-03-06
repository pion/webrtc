package ivfwriter

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"

	"github.com/pions/rtp"
	"github.com/pions/rtp/codecs"
)

// IVFWriter is used to take RTP packets and write them to an IVF on disk
type IVFWriter struct {
	stream       io.Writer
	fd           *os.File
	count        uint64
	currentFrame []byte
}

// New builds a new IVF writer
func New(fileName string) (*IVFWriter, error) {
	f, err := os.Create(fileName)
	if err != nil {
		return nil, err
	}
	writer, err := NewWith(f)
	if err != nil {
		return nil, err
	}
	writer.fd = f
	return writer, nil
}

// NewWith initialize a new IVF writer with an io.Writer output
func NewWith(out io.Writer) (*IVFWriter, error) {
	if out == nil {
		return nil, fmt.Errorf("file not opened")
	}

	writer := &IVFWriter{
		stream: out,
	}
	if err := writer.writeHeader(); err != nil {
		return nil, err
	}
	return writer, nil
}

func (i *IVFWriter) writeHeader() error {
	header := make([]byte, 32)
	copy(header[0:], []byte("DKIF"))                // DKIF
	binary.LittleEndian.PutUint16(header[4:], 0)    // Version
	binary.LittleEndian.PutUint16(header[6:], 32)   // Header Size
	copy(header[8:], []byte("VP80"))                // FOURCC
	binary.LittleEndian.PutUint16(header[12:], 640) // Version
	binary.LittleEndian.PutUint16(header[14:], 480) // Header Size
	binary.LittleEndian.PutUint32(header[16:], 30)  // Framerate numerator
	binary.LittleEndian.PutUint32(header[20:], 1)   // Framerate Denominator
	binary.LittleEndian.PutUint32(header[24:], 900) // Frame count, will be updated on first Close() call
	binary.LittleEndian.PutUint32(header[28:], 0)   // Unused

	_, err := i.stream.Write(header)
	return err
}

// AddPacket adds a new packet and writes the appropriate headers for it
func (i *IVFWriter) AddPacket(packet *rtp.Packet) error {
	if i.stream == nil {
		return fmt.Errorf("file not opened")
	}

	vp8Packet := codecs.VP8Packet{}
	_, err := vp8Packet.Unmarshal(packet)
	if err != nil {
		return err
	}

	i.currentFrame = append(i.currentFrame, vp8Packet.Payload[0:]...)

	if !packet.Marker {
		return nil
	} else if len(i.currentFrame) == 0 {
		return nil
	}

	frameHeader := make([]byte, 12)
	binary.LittleEndian.PutUint32(frameHeader[0:], uint32(len(i.currentFrame))) // Frame length
	binary.LittleEndian.PutUint64(frameHeader[4:], i.count)                     // PTS

	i.count++

	if _, err := i.stream.Write(frameHeader); err != nil {
		return err
	} else if _, err := i.stream.Write(i.currentFrame); err != nil {
		return err
	}

	i.currentFrame = nil
	return nil
}

// Close stops the recording
func (i *IVFWriter) Close() error {
	defer func() {
		i.fd = nil
		i.stream = nil
	}()

	if i.fd == nil {
		// Returns no error as it may be convenient to call
		// Close() multiple times
		return nil
	}
	// Update the framecount
	if _, err := i.fd.Seek(24, 0); err != nil {
		return err
	}
	buff := make([]byte, 4)
	binary.LittleEndian.PutUint32(buff, uint32(i.count))
	if _, err := i.fd.Write(buff); err != nil {
		return err
	}

	return i.fd.Close()
}
