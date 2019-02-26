package media

import (
	"fmt"
	"os"
	"testing"

	"github.com/pions/rtp"
	"github.com/stretchr/testify/assert"
)

// PayloadTypes for the default codecs,
// taken from pions/webrtc/mediaengine.go, to avoid cyclic dependency
const (
	defaultPayloadTypeOpus = 111
	defaultPayloadTypeVP8  = 96
)

func TestOpusWriter_FileCreation(t *testing.T) {
	assert := assert.New(t)

	// Create a OpusWriter with an empty filename
	writer, err := NewOpusWriter("", 4800, 2)
	assert.NotNil(err, "NewOpusWriter shouldn't create file with an empty name")
	assert.Nil(writer, "NewOpusWriter should return a nil value for the writer, in case of error")

	// Creates a valid file, and tries to open it twice
	writer, err = NewOpusWriter("/tmp/pions_testCreateOpusWriter1.opus", 4800, 2)
	assert.Nil(err, "NewOpusWriter should create file")
	assert.NotNil(writer, "NewOpusWriter shouldn't return a nil value for the writer, in case of success")
	err = writer.open("/tmp/pions_testCreateOpusWriter1.opus")
	assert.NotNil(err, "OpusWriter shouldn't be able to call open() twice")
	err = os.Remove("/tmp/pions_testCreateOpusWriter1.opus")
	assert.Nil(err, "File should be removable")
}

type opusWriterPacketTest struct {
	fileName     string
	message      string
	messageClose string
	packet       *rtp.Packet
	writer       *OpusWriter
	err          error
	closeErr     error
}

var rawPkt = []byte{
	0x90, 0xe0, 0x69, 0x8f, 0xd9, 0xc2, 0x93, 0xda, 0x1c, 0x64,
	0x27, 0x82, 0x00, 0x01, 0x00, 0x01, 0xFF, 0xFF, 0xFF, 0xFF, 0x98, 0x36, 0xbe, 0x88, 0x9e,
}

func TestOpusWriter_AddPacketAndClose(t *testing.T) {
	assert := assert.New(t)

	// The linter misbehave and thinks this code is the same as the tests in ivf-writer_test
	// nolint:dupl
	addPacketTestCase := []opusWriterPacketTest{
		{
			fileName:     "/tmp/pions_testOpusPacket1.opus",
			message:      "OpusWriter shouldn't be able to write something to a closed file",
			messageClose: "OpusWriter should be able to close an already closed file",
			packet:       nil,
			err:          fmt.Errorf("file not opened"),
			closeErr:     nil,
		},
		{
			fileName:     "/tmp/pions_testOpusPacket2.opus",
			message:      "OpusWriter shouldn't be able to write something else than an Opus packet",
			messageClose: "OpusWriter should be able to close the file",
			packet:       &rtp.Packet{},
			err:          nil, // TODO: Update pions/rpt Opus unmarshal, so it returns an error, and update expected value
			closeErr:     nil,
		},
		{
			fileName:     "/tmp/pions_testOpusPacket3.opus",
			message:      "OpusWriter shouldn't be able to write something else than an Opus packet",
			messageClose: "OpusWriter should be able to close the file",
			packet: &rtp.Packet{
				Header: rtp.Header{
					Marker:           true,
					Extension:        true,
					ExtensionProfile: 1,
					ExtensionPayload: []byte{0xFF, 0xFF, 0xFF, 0xFF},
					Version:          2,
					PayloadOffset:    20,
					PayloadType:      defaultPayloadTypeVP8,
					SequenceNumber:   27023,
					Timestamp:        3653407706,
					SSRC:             476325762,
					CSRC:             []uint32{},
				},
				Payload: rawPkt[20:],
				Raw:     rawPkt,
			},
			err:      nil, // TODO: Update pions/rpt Opus unmarshal, so it returns an error, and update expected value
			closeErr: nil,
		},
		{
			fileName:     "/tmp/pions_testOpusPacket4.opus",
			message:      "OpusWriter should be able to write an Opus packet",
			messageClose: "OpusWriter should be able to close the file",
			// TODO: Is this a valid Opus packet ?
			packet: &rtp.Packet{
				Header: rtp.Header{
					Marker:           true,
					Extension:        true,
					ExtensionProfile: 1,
					ExtensionPayload: []byte{0xFF, 0xFF, 0xFF, 0xFF},
					Version:          2,
					PayloadOffset:    20,
					PayloadType:      defaultPayloadTypeOpus,
					SequenceNumber:   27023,
					Timestamp:        3653407706,
					SSRC:             476325762,
					CSRC:             []uint32{},
				},
				Payload: rawPkt[20:],
				Raw:     rawPkt,
			},
			err:      nil,
			closeErr: nil,
		},
	}

	// First test case has a 'nil' file descriptor
	writer, err := NewOpusWriter(addPacketTestCase[0].fileName, 48000, 2)
	assert.Nil(err, "OpusWriter should be created")
	assert.NotNil(writer, "Writer shouldn't be nil")
	err = writer.fd.Close()
	assert.Nil(err, "OpusWriter should be able to close the file descriptor")
	writer.fd = nil
	addPacketTestCase[0].writer = writer

	// Second test writes tries to write an empty packet
	writer, err = NewOpusWriter(addPacketTestCase[1].fileName, 48000, 2)
	assert.Nil(err, "OpusWriter should be created")
	assert.NotNil(writer, "Writer shouldn't be nil")
	addPacketTestCase[1].writer = writer

	// Third test writes tries to write a VP8 packet
	writer, err = NewOpusWriter(addPacketTestCase[2].fileName, 48000, 2)
	assert.Nil(err, "OpusWriter should be created")
	assert.NotNil(writer, "Writer shouldn't be nil")
	addPacketTestCase[2].writer = writer

	// Fourth test writes tries to write a valid Opus packet
	writer, err = NewOpusWriter(addPacketTestCase[3].fileName, 48000, 2)
	assert.Nil(err, "OpusWriter should be created")
	assert.NotNil(writer, "Writer shouldn't be nil")
	addPacketTestCase[3].writer = writer

	for _, t := range addPacketTestCase {
		res := t.writer.AddPacket(t.packet)
		assert.Equal(res, t.err, t.message)
	}

	for _, t := range addPacketTestCase {
		res := t.writer.Close()
		assert.Equal(res, t.closeErr, t.messageClose)
		err = os.Remove(t.fileName)
		assert.Nil(err, "File should be removable")
	}
}
