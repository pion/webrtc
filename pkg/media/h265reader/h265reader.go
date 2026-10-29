// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

// Package h265reader implements a H265/HEVC Annex-B Reader
package h265reader

import (
	"errors"
	"io"

	"github.com/pion/webrtc/v5/internal/annexb"
)

// H265Reader reads data from stream and constructs h265 nal units.
type H265Reader struct {
	reader     *annexb.Reader
	includeSEI bool
}

var (
	errNilReader           = errors.New("stream is nil")
	errDataIsNotH265Stream = errors.New("data is not a H265/HEVC bitstream")
)

// NewReader creates new H265Reader.
func NewReader(in io.Reader) (*H265Reader, error) {
	if in == nil {
		return nil, errNilReader
	}

	return &H265Reader{reader: annexb.New(in, errDataIsNotH265Stream)}, nil
}

// Option configures the behavior of H265Reader.
type Option func(*H265Reader) error

// NewReaderWithOptions creates new H265Reader with options.
// The default behavior is to skip SEI NAL units.
func NewReaderWithOptions(in io.Reader, options ...Option) (*H265Reader, error) {
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
	return func(r *H265Reader) error {
		r.includeSEI = include

		return nil
	}
}

// NAL H.265/HEVC Network Abstraction Layer.
type NAL struct {
	PictureOrderCount uint32

	/* NAL Unit header https://datatracker.ietf.org/doc/html/rfc7798#section-1.1.4
	+---------------+---------------+
	|0|1|2|3|4|5|6|7|0|1|2|3|4|5|6|7|
	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	|F|   Type    |  LayerId  | TID |
	+-------------+-----------------+
	*/
	ForbiddenZeroBit bool
	NalUnitType      NalUnitType
	LayerID          uint8
	TemporalIDPlus1  uint8

	Data []byte // header bytes + rbsp
}

// NextNAL returns the next NAL unit, or io.EOF after the final unit.
func (reader *H265Reader) NextNAL() (*NAL, error) {
	for {
		data, err := reader.reader.Next()
		if err != nil {
			return nil, err
		}
		nal := newNal(data)
		nal.parseHeader()
		if !reader.includeSEI && (nal.NalUnitType == NalUnitTypePrefixSei || nal.NalUnitType == NalUnitTypeSuffixSei) {
			continue
		}

		return nal, nil
	}
}

func newNal(data []byte) *NAL {
	return &NAL{
		PictureOrderCount: 0,
		ForbiddenZeroBit:  false,
		NalUnitType:       NalUnitTypeTrailN,
		LayerID:           0,
		TemporalIDPlus1:   0,
		Data:              data,
	}
}

func (h *NAL) parseHeader() {
	if len(h.Data) < 2 {
		return
	}

	// H.265 NAL header is 2 bytes
	firstByte := h.Data[0]
	secondByte := h.Data[1]

	h.ForbiddenZeroBit = (firstByte & 0x80) != 0
	h.NalUnitType = NalUnitType((firstByte & 0x7E) >> 1)
	h.LayerID = ((firstByte & 0x01) << 5) | ((secondByte & 0xF8) >> 3)
	h.TemporalIDPlus1 = secondByte & 0x07
}
