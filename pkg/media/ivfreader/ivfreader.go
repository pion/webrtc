// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

// Package ivfreader implements IVF media container reader
package ivfreader

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

const (
	ivfFileHeaderSignature = "DKIF"
	ivfFileHeaderSize      = 32
	ivfFrameHeaderSize     = 12
)

var (
	errNilStream             = errors.New("stream is nil")
	errIncompleteFrameHeader = errors.New("incomplete frame header")
	errIncompleteFrameData   = errors.New("incomplete frame data")
	errIncompleteFileHeader  = errors.New("incomplete file header")
	errSignatureMismatch     = errors.New("IVF signature mismatch")
	errUnknownIVFVersion     = errors.New("IVF version unknown, parser may not parse correctly")
)

// IVFFileHeader 32-byte header for IVF files
// https://wiki.multimedia.cx/index.php/IVF
type IVFFileHeader struct {
	signature           string // 0-3
	version             uint16 // 4-5
	headerSize          uint16 // 6-7
	FourCC              string // 8-11
	Width               uint16 // 12-13
	Height              uint16 // 14-15
	TimebaseDenominator uint32 // 16-19
	TimebaseNumerator   uint32 // 20-23
	NumFrames           uint32 // 24-27
	unused              uint32 // 28-31
}

// IVFFrameHeader 12-byte header for IVF frames
// https://wiki.multimedia.cx/index.php/IVF
type IVFFrameHeader struct {
	FrameSize uint32 // 0-3
	Timestamp uint64 // 4-11
}

// IVFReader is used to read IVF files and return frame payloads
type IVFReader struct {
	stream               io.Reader
	bytesReadSuccesfully int64
}

// NewWith returns a new IVF reader and IVF file header
// with an io.Reader input
func NewWith(in io.Reader) (*IVFReader, *IVFFileHeader, error) {
	if in == nil {
		return nil, nil, errNilStream
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

// ResetReader resets the internal stream of IVFReader. This is useful
// for live streams, where the end of the file might be read without the
// data being finished.
func (i *IVFReader) ResetReader(reset func(bytesRead int64) io.Reader) {
	i.stream = reset(i.bytesReadSuccesfully)
}

// ParseNextFrame reads from stream and returns IVF frame payload, header,
// and an error if there is incomplete frame data.
// Returns all nil values when no more frames are available.
func (i *IVFReader) ParseNextFrame() ([]byte, *IVFFrameHeader, error) {
	buffer := make([]byte, ivfFrameHeaderSize)
	var header *IVFFrameHeader

	bytesRead, err := io.ReadFull(i.stream, buffer)
	headerBytesRead := bytesRead
	if errors.Is(err, io.ErrUnexpectedEOF) {
		return nil, nil, errIncompleteFrameHeader
	} else if err != nil {
		return nil, nil, err
	}

	header = &IVFFrameHeader{
		FrameSize: binary.LittleEndian.Uint32(buffer[:4]),
		Timestamp: binary.LittleEndian.Uint64(buffer[4:12]),
	}

	payload := make([]byte, header.FrameSize)
	bytesRead, err = io.ReadFull(i.stream, payload)
	if errors.Is(err, io.ErrUnexpectedEOF) {
		return nil, nil, errIncompleteFrameData
	} else if err != nil {
		return nil, nil, err
	}

	i.bytesReadSuccesfully += int64(headerBytesRead) + int64(bytesRead)
	return payload, header, nil
}

// parseFileHeader reads 32 bytes from stream and returns
// IVF file header. This is always called before ParseNextFrame()
func (i *IVFReader) parseFileHeader() (*IVFFileHeader, error) {
	buffer := make([]byte, ivfFileHeaderSize)

	bytesRead, err := io.ReadFull(i.stream, buffer)
	if errors.Is(err, io.ErrUnexpectedEOF) {
		return nil, errIncompleteFileHeader
	} else if err != nil {
		return nil, err
	}

	header := &IVFFileHeader{
		signature:           string(buffer[:4]),
		version:             binary.LittleEndian.Uint16(buffer[4:6]),
		headerSize:          binary.LittleEndian.Uint16(buffer[6:8]),
		FourCC:              string(buffer[8:12]),
		Width:               binary.LittleEndian.Uint16(buffer[12:14]),
		Height:              binary.LittleEndian.Uint16(buffer[14:16]),
		TimebaseDenominator: binary.LittleEndian.Uint32(buffer[16:20]),
		TimebaseNumerator:   binary.LittleEndian.Uint32(buffer[20:24]),
		NumFrames:           binary.LittleEndian.Uint32(buffer[24:28]),
		unused:              binary.LittleEndian.Uint32(buffer[28:32]),
	}

	if header.signature != ivfFileHeaderSignature {
		return nil, errSignatureMismatch
	} else if header.version != uint16(0) {
		return nil, fmt.Errorf("%w: expected(0) got(%d)", errUnknownIVFVersion, header.version)
	}

	i.bytesReadSuccesfully += int64(bytesRead)
	return header, nil
}
