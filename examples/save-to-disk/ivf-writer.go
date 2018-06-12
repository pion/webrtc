package main

import (
	"encoding/binary"
	"fmt"
	"os"

	"github.com/pions/webrtc/pkg/rtp"
	"github.com/pions/webrtc/pkg/rtp/codecs"
)

type IVFWriter struct {
	fd           *os.File
	count        uint64
	currentFrame []byte
}

func panicWrite(fd *os.File, data []byte) {
	if _, err := fd.Write(data); err != nil {
		panic(err)
	}
}

func NewIVFWriter(fileName string) (*IVFWriter, error) {
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

	panicWrite(f, header)

	i := &IVFWriter{fd: f}
	return i, nil
}

func (i *IVFWriter) AddPacket(packet *rtp.Packet) {

	vp8Packet := codecs.VP8Packet{}
	err := vp8Packet.Unmarshal(packet)
	if err != nil {
		panic(err)
	}

	i.currentFrame = append(i.currentFrame, vp8Packet.Payload[0:]...)

	if !packet.Marker {
		return
	} else if len(i.currentFrame) == 0 {
		fmt.Println("skipping")
		return
	}

	frameHeader := make([]byte, 12)
	binary.LittleEndian.PutUint32(frameHeader[0:], uint32(len(i.currentFrame))) // Frame length
	binary.LittleEndian.PutUint64(frameHeader[4:], i.count)                     // PTS

	i.count += 1

	panicWrite(i.fd, frameHeader)
	panicWrite(i.fd, i.currentFrame)

	i.currentFrame = nil
}

func (i *IVFWriter) Close() error {
	return i.fd.Close()
}
