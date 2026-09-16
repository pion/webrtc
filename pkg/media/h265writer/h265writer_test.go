// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package h265writer

import (
	"bytes"
	"testing"

	"github.com/pion/rtp"
	"github.com/stretchr/testify/assert"
)

func TestH265Writer_WriteRTP(t *testing.T) {
	buf := &bytes.Buffer{}
	writer := NewWith(buf)
	defer func() {
		assert.NoError(t, writer.Close())
	}()

	// Test with empty payload
	packet := &rtp.Packet{Payload: []byte{}}
	err := writer.WriteRTP(packet)
	assert.NoError(t, err)

	// Test with VPS packet (key frame)
	vpsPayload := []byte{0x40, 0x01, 0x0C, 0x01, 0xFF, 0xFF, 0x01, 0x60}
	packet = &rtp.Packet{Payload: vpsPayload}

	err = writer.WriteRTP(packet)
	assert.NoError(t, err)

	// Check that the buffer contains the expected start code + VPS data
	expectedContent := append([]byte{0x00, 0x00, 0x00, 0x01}, vpsPayload...)
	assert.Equal(t, expectedContent, buf.Bytes(), "Buffer should contain start code followed by VPS payload")
}

func TestH265Writer_WriteRTP_FragmentedKeyFrame(t *testing.T) {
	buf := &bytes.Buffer{}
	writer := NewWith(buf)
	defer func() {
		assert.NoError(t, writer.Close())
	}()

	// an IDR_W_RADL NAL unit split over two FUs, with no parameter set sent in-band
	assert.NoError(t, writer.WriteRTP(&rtp.Packet{Payload: []byte{0x62, 0x01, 0x93, 0xAF, 0x06}}))
	assert.NoError(t, writer.WriteRTP(&rtp.Packet{Payload: []byte{0x62, 0x01, 0x53, 0x07, 0x08}}))

	assert.Equal(t, []byte{0x00, 0x00, 0x00, 0x01, 0x26, 0x01, 0xAF, 0x06, 0x07, 0x08}, buf.Bytes())
}

func TestH265Writer_WriteRTP_FragmentedKeyFrameWithoutStart(t *testing.T) {
	buf := &bytes.Buffer{}
	writer := NewWith(buf)
	defer func() {
		assert.NoError(t, writer.Close())
	}()

	// the start fragment of an IDR_W_RADL was lost, only its end fragment arrives
	assert.NoError(t, writer.WriteRTP(&rtp.Packet{Payload: []byte{0x62, 0x01, 0x53, 0x07, 0x08}}))
	// followed by a TRAIL_R NAL unit, which must not be written before a key frame
	assert.NoError(t, writer.WriteRTP(&rtp.Packet{Payload: []byte{0x02, 0x01, 0xAF, 0x06}}))

	assert.Empty(t, buf.Bytes())
}

func TestIsKeyFrame(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected bool
	}{
		{
			name:     "VPS NAL unit",
			data:     []byte{0x40, 0x01, 0x0C, 0x01}, // VPS (type 32)
			expected: true,
		},
		{
			name:     "SPS NAL unit",
			data:     []byte{0x42, 0x01, 0x01, 0x01}, // SPS (type 33)
			expected: true,
		},
		{
			name:     "PPS NAL unit",
			data:     []byte{0x44, 0x01, 0xC1, 0x73}, // PPS (type 34)
			expected: true,
		},
		{
			name:     "IDR_W_RADL NAL unit",
			data:     []byte{0x26, 0x01, 0xAF, 0x06}, // IDR_W_RADL (type 19)
			expected: true,
		},
		{
			name:     "IDR_N_LP NAL unit",
			data:     []byte{0x28, 0x01, 0xAF, 0x06}, // IDR_N_LP (type 20)
			expected: true,
		},
		{
			name:     "TRAIL_R NAL unit",
			data:     []byte{0x02, 0x01, 0xAF, 0x06}, // TRAIL_R (type 1)
			expected: false,
		},
		{
			name:     "Empty data",
			data:     []byte{},
			expected: false,
		},
		{
			name:     "Single byte",
			data:     []byte{0x40},
			expected: false,
		},
		{
			name:     "Fragmentation Unit start with IDR_W_RADL",
			data:     []byte{0x62, 0x01, 0x93}, // FU header: S=1, E=0, FuType=19
			expected: true,
		},
		{
			name:     "Fragmentation Unit start with IDR_N_LP",
			data:     []byte{0x62, 0x01, 0x94}, // FU header: S=1, E=0, FuType=20
			expected: true,
		},
		{
			name:     "Fragmentation Unit middle with IDR_W_RADL",
			data:     []byte{0x62, 0x01, 0x13}, // FU header: S=0, E=0, FuType=19
			expected: false,
		},
		{
			name:     "Fragmentation Unit end with IDR_W_RADL",
			data:     []byte{0x62, 0x01, 0x53}, // FU header: S=0, E=1, FuType=19
			expected: false,
		},
		{
			name:     "Fragmentation Unit start with TRAIL_R",
			data:     []byte{0x62, 0x01, 0x81}, // FU header: S=1, E=0, FuType=1
			expected: false,
		},
		{
			name:     "Fragmentation Unit end with TRAIL_N",
			data:     []byte{0x62, 0x01, 0x40}, // FU header: S=0, E=1, FuType=0
			expected: false,
		},
		{
			name:     "Fragmentation Unit start with PREFIX_SEI",
			data:     []byte{0x62, 0x01, 0xA7}, // FU header: S=1, E=0, FuType=39
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isKeyFrame(tt.data)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCheckAggregationPacketForKeyFrame(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected bool
	}{
		{
			name: "AP with VPS",
			data: []byte{
				0x60, 0x01, // AP header
				0x00, 0x04, // NALU size (4 bytes)
				0x40, 0x01, 0x0C, 0x01, // VPS NAL unit
			},
			expected: true,
		},
		{
			name: "AP with TRAIL_R",
			data: []byte{
				0x60, 0x01, // AP header
				0x00, 0x04, // NALU size (4 bytes)
				0x02, 0x01, 0xAF, 0x06, // TRAIL_R NAL unit
			},
			expected: false,
		},
		{
			name: "AP with multiple NALUs including SPS",
			data: []byte{
				0x60, 0x01, // AP header
				0x00, 0x04, // First NALU size
				0x02, 0x01, 0xAF, 0x06, // TRAIL_R NAL unit
				0x00, 0x04, // Second NALU size
				0x42, 0x01, 0x01, 0x01, // SPS NAL unit
			},
			expected: true,
		},
		{
			name:     "Malformed AP - insufficient data",
			data:     []byte{0x60, 0x01, 0x00}, // AP header + incomplete size
			expected: false,
		},
		{
			name:     "Empty AP",
			data:     []byte{0x60, 0x01}, // AP header only
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := checkAggregationPacketForKeyFrame(tt.data)
			assert.Equal(t, tt.expected, result)
		})
	}
}
