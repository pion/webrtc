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

func (w *writerCloser) Close() error {
	return errors.New("close error")
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
			"When given a keyframe; it should return true",
			[]byte{0x38, 0x00, 0x03, 0x27, 0x90, 0x90, 0x00, 0x05, 0x28, 0x90, 0x90, 0x90, 0x90},
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
	}{
		{
			"When given an empty payload; it should return nil",
			[]byte{},
			false,
			[]byte{},
			nil,
		},
		{
			"When no keyframe is defined; it should discard the packet",
			[]byte{0x25, 0x90, 0x90},
			false,
			[]byte{},
			nil,
		},
		{
			"When a valid Single NAL Unit packet is given; it should unpack it without error",
			[]byte{0x27, 0x90, 0x90},
			true,
			[]byte{0x00, 0x00, 0x00, 0x01, 0x27, 0x90, 0x90},
			nil,
		},
		{
			"When a valid STAP-A packet is given; it should unpack it without error",
			[]byte{0x38, 0x00, 0x03, 0x27, 0x90, 0x90, 0x00, 0x05, 0x28, 0x90, 0x90, 0x90, 0x90},
			true,
			[]byte{0x00, 0x00, 0x00, 0x01, 0x27, 0x90, 0x90, 0x00, 0x00, 0x00, 0x01, 0x28, 0x90, 0x90, 0x90, 0x90},
			nil,
		},
		{
			"When a valid FU-A start packet is given; it should unpack it without error",
			[]byte{0x3C, 0x85, 0x90, 0x90, 0x90},
			true,
			[]byte{0x00, 0x00, 0x00, 0x01, 0x25, 0x90, 0x90, 0x90},
			nil,
		},
		{
			"When a valid FU-A end packet is given; it should unpack it without error",
			[]byte{0x3C, 0x45, 0x90, 0x90, 0x90},
			true,
			[]byte{0x90, 0x90, 0x90},
			nil,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			writer := &bytes.Buffer{}
			h264Writer := &H264Writer{
				hasKeyFrame: tt.hasKeyFrame,
				writer:      writer,
			}
			packet := &rtp.Packet{
				Payload: tt.payload,
			}

			err := h264Writer.WriteRTP(packet)

			assert.Equal(t, tt.wantErr, err)
			assert.True(t, bytes.Equal(tt.wantBytes, writer.Bytes()))
			assert.Nil(t, h264Writer.Close())
		})
	}
}
