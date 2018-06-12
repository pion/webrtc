package main

import (
	"encoding/binary"
	"fmt"
	"os"

	"github.com/pions/webrtc/pkg/rtp"
)

type IVFWriter struct {
	fd           *os.File
	count        uint64
	currentFrame []byte
}

type VP8RTPPacket struct {
	// Required Header
	X   uint8 /* extended controlbits present */
	N   uint8 /* (non-reference frame)  when set to 1 this frame can be discarded */
	S   uint8 /* start of VP8 partition */
	PID uint8 /* partition index */

	// Optional Header
	I         uint8  /* 1 if PictureID is present */
	L         uint8  /* 1 if TL0PICIDX is present */
	T         uint8  /* 1 if TID is present */
	K         uint8  /* 1 if KEYIDX is present */
	PictureID uint16 /* 8 or 16 bits, picture ID */
	TL0PICIDX uint8  /* 8 bits temporal level zero index */

	Payload []byte
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

func (i *IVFWriter) DecodeVP8RTPPacket(packet *rtp.Packet) (*VP8RTPPacket, error) {
	p := packet.Payload

	vp8Packet := &VP8RTPPacket{}

	payloadIndex := 0
	vp8Packet.X = (p[payloadIndex] & 0x80) >> 7
	vp8Packet.N = (p[payloadIndex] & 0x20) >> 5
	vp8Packet.S = (p[payloadIndex] & 0x10) >> 4
	vp8Packet.PID = p[payloadIndex] & 0x07

	payloadIndex++

	if vp8Packet.X == 1 {
		vp8Packet.I = (p[payloadIndex] & 0x80) >> 7
		vp8Packet.L = (p[payloadIndex] & 0x40) >> 6
		vp8Packet.T = (p[payloadIndex] & 0x20) >> 5
		vp8Packet.K = (p[payloadIndex] & 0x10) >> 4
		payloadIndex++
	}

	if vp8Packet.I == 1 { // PID present?
		if p[payloadIndex]&0x80 > 0 { // M == 1, PID is 16bit
			payloadIndex += 2
		} else {
			payloadIndex++
		}
	}

	if vp8Packet.L == 1 {
		payloadIndex++
	}

	if vp8Packet.T == 1 || vp8Packet.K == 1 {
		payloadIndex++
	}

	vp8Packet.Payload = p[payloadIndex:]

	return vp8Packet, nil
}

func (i *IVFWriter) AddPacket(packet *rtp.Packet) {

	vp8Packet, err := i.DecodeVP8RTPPacket(packet)
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
