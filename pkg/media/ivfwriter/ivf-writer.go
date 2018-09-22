package ivfwriter

import (
	"encoding/binary"
	"fmt"
	"os"

	"github.com/pions/webrtc/pkg/rtp"
	"github.com/pions/webrtc/pkg/rtp/codecs"
)

// IVFWriter is used to take RTP packets and write them to an IVF on disk
type IVFWriter struct {
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

	header := make([]byte, 32)
	copy(header[0:], []byte("DKIF"))                // DKIF
	binary.LittleEndian.PutUint16(header[4:], 0)    // Version
	binary.LittleEndian.PutUint16(header[6:], 32)   // Header Size
	copy(header[8:], []byte("VP80"))                // FOURCC
	binary.LittleEndian.PutUint16(header[12:], 640) // Version
	binary.LittleEndian.PutUint16(header[14:], 480) // Header Size
	binary.LittleEndian.PutUint32(header[16:], 30)  // Framerate numerator
	binary.LittleEndian.PutUint32(header[20:], 1)   // Framerate Denominator
	binary.LittleEndian.PutUint32(header[24:], 900) // Frame count
	binary.LittleEndian.PutUint32(header[28:], 0)   // Unused

	if _, err := f.Write(header); err != nil {
		return nil, err
	}

	return &IVFWriter{fd: f}, nil
}

// AddPacket adds a new packet and writes the appropriate headers for it
func (i *IVFWriter) AddPacket(packet *rtp.Packet) error {

	vp8Packet := codecs.VP8Packet{}
	_, err := vp8Packet.Unmarshal(packet)
	if err != nil {
		return err
	}

	i.currentFrame = append(i.currentFrame, vp8Packet.Payload[0:]...)

	if !packet.Marker {
		return nil
	} else if len(i.currentFrame) == 0 {
		fmt.Println("skipping")
		return nil
	}

	frameHeader := make([]byte, 12)
	binary.LittleEndian.PutUint32(frameHeader[0:], uint32(len(i.currentFrame))) // Frame length
	binary.LittleEndian.PutUint64(frameHeader[4:], i.count)                     // PTS

	i.count++

	if _, err := i.fd.Write(frameHeader); err != nil {
		return err
	} else if _, err := i.fd.Write(i.currentFrame); err != nil {
		return err
	}

	i.currentFrame = nil
	return nil
}
