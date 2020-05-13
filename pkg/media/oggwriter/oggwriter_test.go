package oggwriter

import (
	"bytes"
	"fmt"
	"io"
	"testing"

	"github.com/pion/rtp"
	"github.com/stretchr/testify/assert"
)

type oggWriterPacketTest struct {
	buffer       io.Writer
	message      string
	messageClose string
	packet       *rtp.Packet
	writer       *OggWriter
	err          error
	closeErr     error
}

func TestOggWriter_AddPacketAndClose(t *testing.T) {
	rawPkt := []byte{
		0x90, 0xe0, 0x69, 0x8f, 0xd9, 0xc2, 0x93, 0xda, 0x1c, 0x64,
		0x27, 0x82, 0x00, 0x01, 0x00, 0x01, 0xFF, 0xFF, 0xFF, 0xFF, 0x98, 0x36, 0xbe, 0x88, 0x9e,
	}

	validPacket := &rtp.Packet{
		Header: rtp.Header{
			Marker:           true,
			Extension:        true,
			ExtensionProfile: 1,
			Version:          2,
			PayloadOffset:    20,
			PayloadType:      111,
			SequenceNumber:   27023,
			Timestamp:        3653407706,
			SSRC:             476325762,
			CSRC:             []uint32{},
		},
		Payload: rawPkt[20:],
		Raw:     rawPkt,
	}
	assert.NoError(t, validPacket.SetExtension(0, []byte{0xFF, 0xFF, 0xFF, 0xFF}))

	assert := assert.New(t)

	// The linter misbehave and thinks this code is the same as the tests in ivf-writer_test
	// nolint:dupl
	addPacketTestCase := []oggWriterPacketTest{
		{
			buffer:       &bytes.Buffer{},
			message:      "OggWriter shouldn't be able to write something to a closed file",
			messageClose: "OggWriter should be able to close an already closed file",
			packet:       validPacket,
			err:          fmt.Errorf("file not opened"),
			closeErr:     nil,
		},
		{
			buffer:       &bytes.Buffer{},
			message:      "OggWriter shouldn't be able to write an empty packet",
			messageClose: "OggWriter should be able to close the file",
			packet:       &rtp.Packet{},
			err:          fmt.Errorf("invalid nil packet"),
			closeErr:     nil,
		},
		{
			buffer:       &bytes.Buffer{},
			message:      "OggWriter should be able to write an Opus packet",
			messageClose: "OggWriter should be able to close the file",
			packet:       validPacket,
			err:          nil,
			closeErr:     nil,
		},
		{
			buffer:       nil,
			message:      "OggWriter shouldn't be able to write something to a closed file",
			messageClose: "OggWriter should be able to close an already closed file",
			packet:       nil,
			err:          fmt.Errorf("file not opened"),
			closeErr:     nil,
		},
	}

	// First test case has a 'nil' file descriptor
	writer, err := NewWith(addPacketTestCase[0].buffer, 48000, 2)
	assert.Nil(err, "OggWriter should be created")
	assert.NotNil(writer, "Writer shouldn't be nil")
	err = writer.Close()
	assert.Nil(err, "OggWriter should be able to close the file descriptor")
	writer.stream = nil
	addPacketTestCase[0].writer = writer

	// Second test writes tries to write an empty packet
	writer, err = NewWith(addPacketTestCase[1].buffer, 48000, 2)
	assert.Nil(err, "OggWriter should be created")
	assert.NotNil(writer, "Writer shouldn't be nil")
	addPacketTestCase[1].writer = writer

	// Third test writes tries to write a valid Opus packet
	writer, err = NewWith(addPacketTestCase[2].buffer, 48000, 2)
	assert.Nil(err, "OggWriter should be created")
	assert.NotNil(writer, "Writer shouldn't be nil")
	addPacketTestCase[2].writer = writer

	// Fourth test tries to write to a nil stream
	writer, err = NewWith(addPacketTestCase[3].buffer, 4800, 2)
	assert.NotNil(err, "IVFWriter shouldn't be created")
	assert.Nil(writer, "Writer should be nil")
	addPacketTestCase[3].writer = writer

	for _, t := range addPacketTestCase {
		if t.writer != nil {
			res := t.writer.WriteRTP(t.packet)
			assert.Equal(t.err, res, t.message)
		}
	}

	for _, t := range addPacketTestCase {
		if t.writer != nil {
			res := t.writer.Close()
			assert.Equal(t.closeErr, res, t.messageClose)
		}
	}
}
