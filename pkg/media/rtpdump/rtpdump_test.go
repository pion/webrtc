// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package rtpdump

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestHeaderRoundTrip(t *testing.T) {
	for _, test := range []struct {
		Header Header
	}{
		{
			Header: Header{
				Start:  time.Unix(0, 0).UTC(),
				Source: net.IPv4(0, 0, 0, 0),
				Port:   0,
			},
		},
		{
			Header: Header{
				Start:  time.Date(2019, 3, 25, 1, 1, 1, 0, time.UTC),
				Source: net.IPv4(1, 2, 3, 4),
				Port:   8080,
			},
		},
	} {
		d, err := test.Header.Marshal()
		assert.NoError(t, err)

		var hdr Header
		assert.NoError(t, hdr.Unmarshal(d))
		assert.Equal(t, test.Header, hdr)
	}
}

func TestMarshalHeader(t *testing.T) {
	for _, test := range []struct {
		Name    string
		Header  Header
		Want    []byte
		WantErr error
	}{
		{
			Name: "nil source",
			Header: Header{
				Start:  time.Unix(0, 0).UTC(),
				Source: nil,
				Port:   0,
			},
			Want: []byte{
				0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00,
			},
		},
	} {
		data, err := test.Header.Marshal()
		assert.ErrorIs(t, err, test.WantErr)
		assert.Equal(t, test.Want, data)
	}
}

func TestPacketRoundTrip(t *testing.T) {
	for _, test := range []struct {
		Packet Packet
	}{
		{
			Packet: Packet{
				Offset:  0,
				IsRTCP:  false,
				Payload: []byte{0},
			},
		},
		{
			Packet: Packet{
				Offset:  0,
				IsRTCP:  true,
				Payload: []byte{0},
			},
		},
		{
			Packet: Packet{
				Offset:  123 * time.Millisecond,
				IsRTCP:  false,
				Payload: []byte{1, 2, 3, 4},
			},
		},
	} {
		packet, err := test.Packet.Marshal()
		assert.NoError(t, err)

		var pkt Packet
		assert.NoError(t, pkt.Unmarshal(packet))

		assert.Equal(t, test.Packet, pkt)
	}
}
