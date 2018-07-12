package sctp

import (
	"testing"

	"github.com/pkg/errors"
)

func TestInitChunk(t *testing.T) {
	pkt := &Packet{}
	rawPkt := []byte{0x13, 0x88, 0x13, 0x88, 0x00, 0x00, 0x00, 0x00, 0x81, 0x46, 0x9d, 0xfc, 0x01, 0x00, 0x00, 0x56, 0x55,
		0xb9, 0x64, 0xa5, 0x00, 0x02, 0x00, 0x00, 0x04, 0x00, 0x08, 0x00, 0xe8, 0x6d, 0x10, 0x30, 0xc0, 0x00, 0x00, 0x04, 0x80,
		0x08, 0x00, 0x09, 0xc0, 0x0f, 0xc1, 0x80, 0x82, 0x00, 0x00, 0x00, 0x80, 0x02, 0x00, 0x24, 0x9f, 0xeb, 0xbb, 0x5c, 0x50,
		0xc9, 0xbf, 0x75, 0x9c, 0xb1, 0x2c, 0x57, 0x4f, 0xa4, 0x5a, 0x51, 0xba, 0x60, 0x17, 0x78, 0x27, 0x94, 0x5c, 0x31, 0xe6,
		0x5d, 0x5b, 0x09, 0x47, 0xe2, 0x22, 0x06, 0x80, 0x04, 0x00, 0x06, 0x00, 0x01, 0x00, 0x00, 0x80, 0x03, 0x00, 0x06, 0x80, 0xc1, 0x00, 0x00}
	err := pkt.Unmarshal(rawPkt)
	if err != nil {
		t.Error(errors.Wrap(err, "Unmarshal failed, has chunk"))
	}

	i, ok := pkt.Chunks[0].(*Init)
	if !ok {
		t.Error("Failed to cast Chunk -> Init")
	}

	if err != nil {
		t.Error(errors.Wrap(err, "Unmarshal init Chunk failed"))
	} else if i.initiateTag != 1438213285 {
		t.Error(errors.Errorf("Unmarshal passed for SCTP packet, but got incorrect initiate tag exp: %d act: %d", 1438213285, i.initiateTag))
	} else if i.advertisedReceiverWindowCredit != 131072 {
		t.Error(errors.Errorf("Unmarshal passed for SCTP packet, but got incorrect advertisedReceiverWindowCredit exp: %d act: %d", 131072, i.advertisedReceiverWindowCredit))
	} else if i.numOutboundStreams != 1024 {
		t.Error(errors.Errorf("Unmarshal passed for SCTP packet, but got incorrect numOutboundStreams tag exp: %d act: %d", 1024, i.numOutboundStreams))
	} else if i.numInboundStreams != 2048 {
		t.Error(errors.Errorf("Unmarshal passed for SCTP packet, but got incorrect numInboundStreams exp: %d act: %d", 2048, i.numInboundStreams))
	} else if i.initialTSN != 3899461680 {
		t.Error(errors.Errorf("Unmarshal passed for SCTP packet, but got incorrect initialTSN exp: %d act: %d", 3899461680, i.initialTSN))
	}
}

func TestInitAck(t *testing.T) {
	pkt := &Packet{}
	rawPkt := []byte{0x13, 0x88, 0x13, 0x88, 0xce, 0x15, 0x79, 0xa2, 0x96, 0x19, 0xe8, 0xb2, 0x02, 0x00, 0x00, 0x1c, 0xeb, 0x81, 0x4e, 0x01, 0x00, 0x00, 0x00, 0x00, 0x04, 0x00, 0x08, 0x00, 0x50, 0xdf, 0x90, 0xd9, 0x00, 0x07, 0x00, 0x08, 0x94, 0x06, 0x2f, 0x93}
	err := pkt.Unmarshal(rawPkt)
	if err != nil {
		t.Error(errors.Wrap(err, "Unmarshal failed, has chunk"))
	}

	_, ok := pkt.Chunks[0].(*InitAck)
	if !ok {
		t.Error("Failed to cast Chunk -> Init")
	} else if err != nil {
		t.Error(errors.Wrap(err, "Unmarshal init Chunk failed"))
	}
}
func TestInitMarshalUnmarshal(t *testing.T) {
	p := &Packet{}
	p.DestinationPort = 1
	p.SourcePort = 1
	p.VerificationTag = 123

	initAck := &InitAck{}

	initAck.initialTSN = 123
	initAck.numOutboundStreams = 1
	initAck.numInboundStreams = 1
	initAck.initiateTag = 123
	initAck.advertisedReceiverWindowCredit = 1024
	initAck.params = []Param{NewRandomStateCookie()}

	p.Chunks = []Chunk{initAck}
	rawPkt, err := p.Marshal()
	if err != nil {
		t.Error(errors.Wrap(err, "Failed to marshal packet"))
	}

	pkt := &Packet{}
	err = pkt.Unmarshal(rawPkt)
	if err != nil {
		t.Error(errors.Wrap(err, "Unmarshal failed, has chunk"))
	}

	i, ok := pkt.Chunks[0].(*InitAck)
	if !ok {
		t.Error("Failed to cast Chunk -> InitAck")
	}

	if err != nil {
		t.Error(errors.Wrap(err, "Unmarshal init ack Chunk failed"))
	} else if i.initiateTag != 123 {
		t.Error(errors.Errorf("Unmarshal passed for SCTP packet, but got incorrect initiate tag exp: %d act: %d", 123, i.initiateTag))
	} else if i.advertisedReceiverWindowCredit != 1024 {
		t.Error(errors.Errorf("Unmarshal passed for SCTP packet, but got incorrect advertisedReceiverWindowCredit exp: %d act: %d", 1024, i.advertisedReceiverWindowCredit))
	} else if i.numOutboundStreams != 1 {
		t.Error(errors.Errorf("Unmarshal passed for SCTP packet, but got incorrect numOutboundStreams tag exp: %d act: %d", 1, i.numOutboundStreams))
	} else if i.numInboundStreams != 1 {
		t.Error(errors.Errorf("Unmarshal passed for SCTP packet, but got incorrect numInboundStreams exp: %d act: %d", 1, i.numInboundStreams))
	} else if i.initialTSN != 123 {
		t.Error(errors.Errorf("Unmarshal passed for SCTP packet, but got incorrect initialTSN exp: %d act: %d", 123, i.initialTSN))
	}
}
