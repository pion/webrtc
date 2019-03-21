package ivfwriter

import (
	"bytes"
	"fmt"
	"io"
	"testing"

	"github.com/pions/rtp"
	"github.com/stretchr/testify/assert"
)

type ivfWriterPacketTest struct {
	buffer       io.Writer
	message      string
	messageClose string
	packet       *rtp.Packet
	writer       *IVFWriter
	err          error
	closeErr     error
}

func TestIVFWriter_AddPacketAndClose(t *testing.T) {

	rawPkt := []byte{
		0x90, 0xe0, 0x69, 0x8f, 0xd9, 0xc2, 0x93, 0xda, 0x1c, 0x64,
		0x27, 0x82, 0x00, 0x01, 0x00, 0x01, 0xFF, 0xFF, 0xFF, 0xFF, 0x98, 0x36, 0xbe, 0x88, 0x9e,
	}

	validPacket := &rtp.Packet{
		Header: rtp.Header{
			Marker:           true,
			Extension:        true,
			ExtensionProfile: 1,
			ExtensionPayload: []byte{0xFF, 0xFF, 0xFF, 0xFF},
			Version:          2,
			PayloadOffset:    20,
			PayloadType:      96,
			SequenceNumber:   27023,
			Timestamp:        3653407706,
			SSRC:             476325762,
			CSRC:             []uint32{},
		},
		Payload: rawPkt[20:],
		Raw:     rawPkt,
	}

	assert := assert.New(t)

	// The linter misbehave and thinks this code is the same as the tests in opuswriter_test
	// nolint:dupl
	addPacketTestCase := []ivfWriterPacketTest{
		{
			buffer:       &bytes.Buffer{},
			message:      "IVFWriter shouldn't be able to write something to a closed file",
			messageClose: "IVFWriter should be able to close an already closed file",
			packet:       nil,
			err:          fmt.Errorf("file not opened"),
			closeErr:     nil,
		},
		{
			buffer:       &bytes.Buffer{},
			message:      "IVFWriter shouldn't be able to write something an empty packet",
			messageClose: "IVFWriter should be able to close the file",
			packet:       &rtp.Packet{},
			err:          fmt.Errorf("Payload is not large enough to container header"),
			closeErr:     nil,
		},
		{
			buffer:       &bytes.Buffer{},
			message:      "IVFWriter should be able to write an IVF packet",
			messageClose: "IVFWriter should be able to close the file",
			packet:       validPacket,
			err:          nil,
			closeErr:     nil,
		},
		{
			buffer:       nil,
			message:      "IVFWriter shouldn't be able to write something to a closed file",
			messageClose: "IVFWriter should be able to close an already closed file",
			packet:       nil,
			err:          fmt.Errorf("file not opened"),
			closeErr:     nil,
		},
	}

	// First test case has a 'nil' file descriptor
	writer, err := NewWith(addPacketTestCase[0].buffer)
	assert.Nil(err, "IVFWriter should be created")
	assert.NotNil(writer, "Writer shouldn't be nil")
	err = writer.Close()
	assert.Nil(err, "IVFWriter should be able to close the stream")
	writer.stream = nil
	addPacketTestCase[0].writer = writer

	// Second test tries to write an empty packet
	writer, err = NewWith(addPacketTestCase[1].buffer)
	assert.Nil(err, "IVFWriter should be created")
	assert.NotNil(writer, "Writer shouldn't be nil")
	addPacketTestCase[1].writer = writer

	// Third test tries to write a valid VP8 packet
	writer, err = NewWith(addPacketTestCase[2].buffer)
	assert.Nil(err, "IVFWriter should be created")
	assert.NotNil(writer, "Writer shouldn't be nil")
	addPacketTestCase[2].writer = writer

	// Fourth test tries to write to a nil stream
	writer, err = NewWith(addPacketTestCase[3].buffer)
	assert.NotNil(err, "IVFWriter shouldn't be created")
	assert.Nil(writer, "Writer should be nil")
	addPacketTestCase[3].writer = writer

	for _, t := range addPacketTestCase {
		if t.writer != nil {
			res := t.writer.AddPacket(t.packet)
			assert.Equal(res, t.err, t.message)
		}
	}

	for _, t := range addPacketTestCase {
		if t.writer != nil {
			res := t.writer.Close()
			assert.Equal(res, t.closeErr, t.messageClose)
		}
	}
}
