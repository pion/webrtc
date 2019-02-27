package media

import (
	"fmt"
	"os"
	"testing"

	"github.com/pions/rtp"
	"github.com/stretchr/testify/assert"
)

func TestIVFWriter_FileCreation(t *testing.T) {
	assert := assert.New(t)

	// Create a IVFWriter with an empty filename
	writer, err := NewIVFWriter("")
	assert.NotNil(err, "NewIVFWriter shouldn't create file with an empty name")
	assert.Nil(writer, "NewIVFWriter should return a nil value for the writer, in case of error")

	// Creates a valid file, and tries to open it twice
	writer, err = NewIVFWriter("/tmp/pions_testCreateIVFWriter1.ivf")
	assert.Nil(err, "NewIVFWriter should create file")
	assert.NotNil(writer, "NewIVFWriter shouldn't return a nil value for the writer, in case of success")
	err = writer.open("/tmp/pions_testCreateIVFWriter1.ivf")
	assert.NotNil(err, "IVFWriter shouldn't be able to call open() twice")
	err = os.Remove("/tmp/pions_testCreateIVFWriter1.ivf")
	assert.Nil(err, "File should be removable")
}

type ivfWriterPacketTest struct {
	fileName     string
	message      string
	messageClose string
	packet       *rtp.Packet
	writer       Writer
	err          error
	closeErr     error
}

func TestIVFWriter_AddPacketAndClose(t *testing.T) {
	assert := assert.New(t)

	// The linter misbehave and thinks this code is the same as the tests in opus-writer_test
	// nolint:dupl
	addPacketTestCase := []ivfWriterPacketTest{
		{
			fileName:     "/tmp/pions_testIVFPacket1.ivf",
			message:      "IVFWriter shouldn't be able to write something to a closed file",
			messageClose: "IVFWriter should be able to close an already closed file",
			packet:       nil,
			err:          fmt.Errorf("file not opened"),
			closeErr:     nil,
		},
		{
			fileName:     "/tmp/pions_testIVFPacket2.ivf",
			message:      "IVFWriter shouldn't be able to write something else than an IVF packet",
			messageClose: "IVFWriter should be able to close the file",
			packet:       &rtp.Packet{},
			err:          fmt.Errorf("Payload is not large enough to container header"),
			closeErr:     nil,
		},
		{
			fileName:     "/tmp/pions_testIVFPacket3.ivf",
			message:      "IVFWriter shouldn't be able to write something else than an IVF packet",
			messageClose: "IVFWriter should be able to close the file",
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
			err:      nil, // TODO: Update expected value
			closeErr: nil,
		},
		{
			fileName:     "/tmp/pions_testIVFPacket4.ivf",
			message:      "IVFWriter should be able to write an IVF packet",
			messageClose: "IVFWriter should be able to close the file",
			// TODO: Is this a valid VP8 packet ?
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
			err:      nil,
			closeErr: nil,
		},
	}

	// First test case has a 'nil' file descriptor
	writer, err := NewIVFWriter(addPacketTestCase[0].fileName)
	assert.Nil(err, "IVFWriter should be created")
	assert.NotNil(writer, "Writer shouldn't be nil")
	err = writer.fd.Close()
	assert.Nil(err, "IVFWriter should be able to close the file descriptor")
	writer.fd = nil
	addPacketTestCase[0].writer = writer

	// Second test writes tries to write an empty packet
	writer, err = NewIVFWriter(addPacketTestCase[1].fileName)
	assert.Nil(err, "IVFWriter should be created")
	assert.NotNil(writer, "Writer shouldn't be nil")
	addPacketTestCase[1].writer = writer

	// Third test writes tries to write a Opus packet
	writer, err = NewIVFWriter(addPacketTestCase[2].fileName)
	assert.Nil(err, "IVFWriter should be created")
	assert.NotNil(writer, "Writer shouldn't be nil")
	addPacketTestCase[2].writer = writer

	// Fourth test writes tries to write a valid VP8 packet
	writer, err = NewIVFWriter(addPacketTestCase[3].fileName)
	assert.Nil(err, "IVFWriter should be created")
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
