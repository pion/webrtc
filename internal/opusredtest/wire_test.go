//go:build opusred && !js

// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package opusredtest

import (
	"testing"

	"github.com/pion/rtp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testMapping() Mapping {
	return Mapping{OpusPT: 111, REDPT: 63, ClockRate: 48000, Channels: 2, REDFMTP: "111/111", HasRED: true}
}

func TestParseWireIndependentGoldenVectors(t *testing.T) {
	header := &rtp.Header{Version: 2, PayloadType: 63, SequenceNumber: 7, Timestamp: 2880, SSRC: 42}
	tests := []struct {
		name    string
		raw     []byte
		offsets []uint16
		blocks  [][]byte
		primary []byte
	}{
		{name: "primary only", raw: []byte{0x6f, 0x30}, primary: []byte{0x30}},
		{name: "one block", raw: []byte{0xef, 0x0f, 0x00, 0x02, 0x6f, 0x10, 0x11, 0x30}, offsets: []uint16{960}, blocks: [][]byte{{0x10, 0x11}}, primary: []byte{0x30}},
		{name: "two blocks", raw: []byte{0xef, 0x1e, 0x00, 0x02, 0xef, 0x0f, 0x00, 0x01, 0x6f, 0x10, 0x11, 0x20, 0x30}, offsets: []uint16{1920, 960}, blocks: [][]byte{{0x10, 0x11}, {0x20}}, primary: []byte{0x30}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			record, err := ParseWire(header, test.raw, testMapping())
			require.NoError(t, err)
			assert.Equal(t, uint16(7), record.Sequence)
			assert.Equal(t, uint32(2880), record.Timestamp)
			assert.Equal(t, uint32(42), record.SSRC)
			assert.Equal(t, uint8(111), record.Primary.PayloadType)
			assert.Equal(t, payloadHash(test.primary), record.Primary.PayloadSHA256)
			require.Len(t, record.Blocks, len(test.blocks))
			for index, block := range test.blocks {
				assert.Equal(t, uint8(111), record.Blocks[index].PayloadType)
				assert.Equal(t, test.offsets[index], record.Blocks[index].TimestampOffset)
				assert.Equal(t, len(block), record.Blocks[index].PayloadLength)
				assert.Equal(t, payloadHash(block), record.Blocks[index].PayloadSHA256)
			}
		})
	}
}

func TestParseWireRejectsInvalidRED(t *testing.T) {
	header := &rtp.Header{Version: 2, PayloadType: 63}
	for _, raw := range [][]byte{nil, {0xef}, {0xef, 0, 0}, {0xef, 0, 0, 1}, {0xef, 0, 0, 3, 0x6f, 1}, {0x6e, 1}, {0x6f}} {
		_, err := ParseWire(header, raw, testMapping())
		require.Error(t, err)
	}
	_, err := ParseWire(nil, []byte{0x6f, 1}, testMapping())
	require.Error(t, err)
}

func TestParseWirePlainAndRecordOwnership(t *testing.T) {
	header := &rtp.Header{Version: 2, Padding: true, PaddingSize: 4, PayloadType: 111, SSRC: 42, CSRC: []uint32{23}}
	require.NoError(t, header.SetExtension(1, []byte{0x11}))
	payload := []byte{0xf8, 0xff, 0xfe}
	record, err := ParseWire(header, payload, testMapping())
	require.NoError(t, err)
	assert.Equal(t, record.PacketRecord, record.Primary)
	assert.Empty(t, record.Blocks)
	assert.Equal(t, header.MarshalSize()+len(payload)+4, record.RTPSize)
	digest := record.PayloadSHA256
	payload[0] = 0
	header.SSRC = 99
	assert.Equal(t, uint32(42), record.SSRC)
	assert.Equal(t, digest, record.PayloadSHA256)
	assert.NotEqual(t, payloadHash(payload), record.PayloadSHA256)
}
