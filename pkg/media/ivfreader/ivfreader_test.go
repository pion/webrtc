package ivfreader

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIVFReader_ValidFileHeader(t *testing.T) {
	assert := assert.New(t)

	// Valid IVF file header taken from test-25fps.vp8 file
	rawHeader := []byte{
		0x44, 0x4b, 0x49, 0x46, 0x00, 0x00, 0x20, 0x00,
		0x56, 0x50, 0x38, 0x30, 0x40, 0x01, 0xf0, 0x00,
		0x32, 0x00, 0x00, 0x00, 0x02, 0x00, 0x00, 0x00,
		0xfa, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	}
	stream := bytes.NewBuffer(rawHeader)

	reader, header, err := NewWith(stream)
	assert.Nil(err, "IVFReader should be created")
	assert.NotNil(reader, "Reader shouldn't be nil")
	assert.NotNil(header, "Header shouldn't be nil")

	assert.Equal("DKIF", header.signature, "signature is 'DKIF'")
	assert.Equal(uint16(0), header.version, "version should be 0")
	assert.Equal("VP80", header.fourcc, "FourCC should be 'VP80'")
	assert.Equal(uint16(320), header.width, "width should be 320")
	assert.Equal(uint16(240), header.height, "height should be 240")
	assert.Equal(uint32(50), header.timebaseDenum, "timebase denominator should be 50")
	assert.Equal(uint32(2), header.timebaseNum, "timebase numerator should be 2")
	assert.Equal(uint32(250), header.numFrames, "number of frames should be 250")
	assert.Equal(uint32(0), header.unused, "bytes should be unused")
}

func TestIVFReader_ValidFile(t *testing.T) {
	assert := assert.New(t)

	// TODO: Should this test file go somewhere else?
	stream, _ := os.Open("./test-25fps.vp8")
	reader, header, err := NewWith(stream)
	assert.Nil(err, "IVFReader should be created")
	assert.NotNil(reader, "Reader shouldn't be nil")
	assert.NotNil(header, "Header shouldn't be nil")

	framesParsed := 0
	for {
		payload, _, err := reader.ParseNextFrame()
		assert.Nil(err, "Shouldn't receive parsing errors")
		if payload == nil {
			break
		}
		framesParsed++
	}
	assert.Equal(250, framesParsed, "Should have parsed 250 frames")
}

