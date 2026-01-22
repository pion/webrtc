// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

// Package h265writer implements H265/HEVC media container writer
package h265writer

import (
	"bytes"
	"encoding/binary"
	"io"
	"os"

	"github.com/pion/rtp"
	"github.com/pion/rtp/codecs"
	"github.com/pion/webrtc/v4/pkg/media/h265reader"
)

const (
	typeAP = 48 // Aggregation Packet
	typeFU = 49 // Fragmentation Unit
)

// H265Writer is used to take H.265/HEVC RTP packets defined in RFC 7798, parse them and
// write the data to an io.Writer.
type H265Writer struct {
	writer       io.Writer
	hasKeyFrame  bool
	cachedPacket *codecs.H265Depacketizer
}

// New builds a new H265 writer.
func New(filename string) (*H265Writer, error) {
	f, err := os.Create(filename) //nolint:gosec
	if err != nil {
		return nil, err
	}

	return NewWith(f), nil
}

// NewWith initializes a new H265 writer with an io.Writer output.
func NewWith(w io.Writer) *H265Writer {
	return &H265Writer{
		writer: w,
	}
}

// WriteRTP adds a new packet and writes the appropriate headers for it.
func (h *H265Writer) WriteRTP(packet *rtp.Packet) error {
	if len(packet.Payload) == 0 {
		return nil
	}

	if !h.hasKeyFrame {
		if h.hasKeyFrame = isKeyFrame(packet.Payload); !h.hasKeyFrame {
			// key frame not defined yet. discarding packet
			return nil
		}
	}

	if h.cachedPacket == nil {
		h.cachedPacket = &codecs.H265Depacketizer{}
	}

	data, err := h.cachedPacket.Unmarshal(packet.Payload)
	if err != nil || len(data) == 0 {
		return err
	}

	_, err = h.writer.Write(data)

	return err
}

// Close closes the underlying writer.
func (h *H265Writer) Close() error {
	h.cachedPacket = nil
	if h.writer != nil {
		if closer, ok := h.writer.(io.Closer); ok {
			return closer.Close()
		}
	}

	return nil
}

func isKeyFrame(data []byte) bool {
	if len(data) < 2 {
		return false
	}

	// Get NAL unit type from first byte (bits 6-1)
	naluType := (data[0] & 0x7E) >> 1
	if isKeyFrameNalu(h265reader.NalUnitType(naluType)) {
		return true
	}

	// Check for parameter sets or IDR frames
	switch naluType {
	case typeAP:
		// For aggregation packets, check if any contained NAL is a key frame
		return checkAggregationPacketForKeyFrame(data)
	case typeFU:
		// For fragmentation units, check the NAL type in the FU header
		if len(data) < 3 {
			return false
		}
		fuNaluType := h265reader.NalUnitType((data[2] & 0x7E) >> 1)

		return isKeyFrameNalu(fuNaluType)
	}

	return false
}

func checkAggregationPacketForKeyFrame(data []byte) bool {
	// Skip the payload header (2 bytes for H.265)
	offset := 2

	for offset < len(data) {
		if offset+2 > len(data) {
			break
		}

		// Read NAL unit size (2 bytes in network byte order)
		var naluSize uint16
		buf := bytes.NewReader(data[offset : offset+2])
		if err := binary.Read(buf, binary.BigEndian, &naluSize); err != nil {
			break
		}
		offset += 2

		if offset+int(naluSize) > len(data) {
			break
		}

		if naluSize > 0 {
			// Check NAL unit type
			naluType := h265reader.NalUnitType((data[offset] & 0x7E) >> 1)
			if isKeyFrameNalu(naluType) {
				return true
			}
		}

		offset += int(naluSize)
	}

	return false
}

func isKeyFrameNalu(naluType h265reader.NalUnitType) bool {
	switch naluType {
	case h265reader.NalUnitTypeVps, h265reader.NalUnitTypeSps, h265reader.NalUnitTypePps,
		h265reader.NalUnitTypeIdrWRadl, h265reader.NalUnitTypeIdrNLp:
		return true
	default:
		return false
	}
}
