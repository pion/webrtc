// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

// Package h264reader implements a H264 Annex-B Reader
package h264reader

import (
	"errors"
	"io"

	"github.com/pion/webrtc/v5/internal/annexb"
)

// H264Reader reads data from stream and constructs h264 nal units.
type H264Reader struct {
	reader     *annexb.Reader
	includeSEI bool
}

var (
	errNilReader           = errors.New("stream is nil")
	errDataIsNotH264Stream = errors.New("data is not a H264 bitstream")
)

// NewReader creates new H264Reader.
func NewReader(in io.Reader) (*H264Reader, error) {
	if in == nil {
		return nil, errNilReader
	}

	return &H264Reader{reader: annexb.New(in, errDataIsNotH264Stream)}, nil
}

// Option configures the behavior of H264Reader.
type Option func(*H264Reader) error

// NewReaderWithOptions creates new H264Reader with options.
// The default behavior is to skip SEI NAL units.
func NewReaderWithOptions(in io.Reader, options ...Option) (*H264Reader, error) {
	reader, err := NewReader(in)
	if err != nil {
		return nil, err
	}

	for _, option := range options {
		if err := option(reader); err != nil {
			return nil, err
		}
	}

	return reader, nil
}

// WithIncludeSEI controls whether SEI (Supplemental Enhancement Information) NAL units are returned.
// Default is false (SEI is skipped).
func WithIncludeSEI(include bool) Option {
	return func(r *H264Reader) error {
		r.includeSEI = include

		return nil
	}
}

// NAL H.264 Network Abstraction Layer.
type NAL struct {
	PictureOrderCount uint32

	// NAL header
	ForbiddenZeroBit bool
	RefIdc           uint8
	UnitType         NalUnitType

	Data []byte // header byte + rbsp
}

// NextNAL returns the next NAL unit, or io.EOF after the final unit.
func (reader *H264Reader) NextNAL() (*NAL, error) {
	for {
		data, err := reader.reader.Next()
		if err != nil {
			return nil, err
		}
		nal := newNal(data)
		nal.parseHeader()
		if !reader.includeSEI && nal.UnitType == NalUnitTypeSEI {
			continue
		}

		return nal, nil
	}
}

func newNal(data []byte) *NAL {
	return &NAL{PictureOrderCount: 0, ForbiddenZeroBit: false, RefIdc: 0, UnitType: NalUnitTypeUnspecified, Data: data}
}

func (h *NAL) parseHeader() {
	firstByte := h.Data[0]
	h.ForbiddenZeroBit = (((firstByte & 0x80) >> 7) == 1) // 0x80 = 0b10000000
	h.RefIdc = (firstByte & 0x60) >> 5                    // 0x60 = 0b01100000
	h.UnitType = NalUnitType((firstByte & 0x1F) >> 0)     // 0x1F = 0b00011111
}
