// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package rtpdump

import (
	"bytes"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestWriter(t *testing.T) {
	buf := bytes.NewBuffer(nil)

	writer, err := NewWriter(buf, Header{
		Start:  time.Unix(9, 0),
		Source: net.IPv4(2, 2, 2, 2),
		Port:   2222,
	})
	assert.NoError(t, err)

	assert.NoError(t, writer.WritePacket(Packet{
		Offset:  time.Millisecond,
		IsRTCP:  false,
		Payload: []byte{9},
	}))

	expected := append(
		[]byte("#!rtpplay1.0 2.2.2.2/2222\n"),
		// header
		0x00, 0x00, 0x00, 0x09,
		0x00, 0x00, 0x00, 0x00,
		0x02, 0x02, 0x02, 0x02,
		0x08, 0xae, 0x00, 0x00,
		// packet header
		0x00, 0x09, 0x00, 0x01,
		0x00, 0x00, 0x00, 0x01,
		0x09,
	)

	assert.Equal(t, expected, buf.Bytes())
}

func TestRoundTrip(t *testing.T) {
	buf := bytes.NewBuffer(nil)

	packets := []Packet{
		{
			Offset:  time.Millisecond,
			IsRTCP:  false,
			Payload: []byte{9},
		},
		{
			Offset:  999 * time.Millisecond,
			IsRTCP:  true,
			Payload: []byte{9},
		},
	}
	hdr := Header{
		Start:  time.Unix(9, 0).UTC(),
		Source: net.IPv4(2, 2, 2, 2),
		Port:   2222,
	}

	writer, err := NewWriter(buf, hdr)
	assert.NoError(t, err)

	for _, pkt := range packets {
		assert.NoError(t, writer.WritePacket(pkt))
	}

	reader, hdr2, err := NewReader(buf)
	assert.NoError(t, err)

	assert.Equal(t, hdr, hdr2, "round trip: header")

	var packets2 []Packet
	for {
		pkt, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		assert.NoError(t, err)
		packets2 = append(packets2, pkt)
	}

	assert.Equal(t, packets, packets2, "round trip: packets")
}
