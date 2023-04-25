// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package h264writer

import (
	"bytes"
	"errors"
	"testing"

	"github.com/pion/rtp"
	"github.com/stretchr/testify/assert"
)

type writerCloser struct {
	bytes.Buffer
}

var errClose = errors.New("close error")

func (w *writerCloser) Close() error {
	return errClose
}

func TestNewWith(t *testing.T) {
	writer := &writerCloser{}
	h264Writer := NewWith(writer)
	assert.NotNil(t, h264Writer.Close())
}

func TestIsKeyFrame(t *testing.T) {
	tests := []struct {
		name    string
		payload []byte
		want    bool
	}{
		{
			"When given a non-keyframe; it should return false",
			[]byte{0x27, 0x90, 0x90},
			false,
		},
		{
			"When given a SPS packetized with STAP-A; it should return true",
			[]byte{0x38, 0x00, 0x03, 0x27, 0x90, 0x90, 0x00, 0x05, 0x28, 0x90, 0x90, 0x90, 0x90},
			true,
		},
		{
			"When given a SPS with no packetization; it should return true",
			[]byte{0x27, 0x90, 0x90, 0x00},
			true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got := isKeyFrame(tt.payload)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestWriteRTP(t *testing.T) {
	tests := []struct {
		name        string
		payload     []byte
		hasKeyFrame bool
		wantBytes   []byte
		wantErr     error
		reuseWriter bool
	}{
		{
			"When given an empty payload; it should return nil",
			[]byte{},
			false,
			[]byte{},
			nil,
			false,
		},
		{
			"When no keyframe is defined; it should discard the packet",
			[]byte{0x25, 0x90, 0x90},
			false,
			[]byte{},
			nil,
			false,
		},
		{
			"When a valid Single NAL Unit packet is given; it should unpack it without error",
			[]byte{0x27, 0x90, 0x90},
			true,
			[]byte{0x00, 0x00, 0x00, 0x01, 0x27, 0x90, 0x90},
			nil,
			false,
		},
		{
			"When a valid STAP-A packet is given; it should unpack it without error",
			[]byte{0x38, 0x00, 0x03, 0x27, 0x90, 0x90, 0x00, 0x05, 0x28, 0x90, 0x90, 0x90, 0x90},
			true,
			[]byte{0x00, 0x00, 0x00, 0x01, 0x27, 0x90, 0x90, 0x00, 0x00, 0x00, 0x01, 0x28, 0x90, 0x90, 0x90, 0x90},
			nil,
			false,
		},
		{
			"When a valid FU-A start packet is given; it should unpack it without error",
			[]byte{0x3C, 0x85, 0x90, 0x90, 0x90},
			true,
			[]byte{},
			nil,
			true,
		},
		{
			"When a valid FU-A end packet is given; it should unpack it without error",
			[]byte{0x3C, 0x45, 0x90, 0x90, 0x90},
			true,
			[]byte{0x00, 0x00, 0x00, 0x01, 0x25, 0x90, 0x90, 0x90, 0x90, 0x90, 0x90},
			nil,
			false,
		},
	}

	var reuseWriter *bytes.Buffer
	var reuseH264Writer *H264Writer

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			writer := &bytes.Buffer{}
			h264Writer := &H264Writer{
				hasKeyFrame: tt.hasKeyFrame,
				writer:      writer,
			}
			if reuseWriter != nil {
				writer = reuseWriter
			}
			if reuseH264Writer != nil {
				h264Writer = reuseH264Writer
			}

			assert.Equal(t, tt.wantErr, h264Writer.WriteRTP(&rtp.Packet{
				Payload: tt.payload,
			}))
			assert.True(t, bytes.Equal(tt.wantBytes, writer.Bytes()))

			if !tt.reuseWriter {
				assert.Nil(t, h264Writer.Close())
				reuseWriter = nil
				reuseH264Writer = nil
			} else {
				reuseWriter = writer
				reuseH264Writer = h264Writer
			}
		})
	}
}
