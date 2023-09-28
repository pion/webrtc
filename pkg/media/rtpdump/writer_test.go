// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package rtpdump

import (
	"bytes"
	"errors"
	"io"
	"net"
	"reflect"
	"testing"
	"time"
)

func TestWriter(t *testing.T) {
	buf := bytes.NewBuffer(nil)

	writer, err := NewWriter(buf, Header{
		Start:  time.Unix(9, 0),
		Source: net.IPv4(2, 2, 2, 2),
		Port:   2222,
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := writer.WritePacket(Packet{
		Offset:  time.Millisecond,
		IsRTCP:  false,
		Payload: []byte{9},
	}); err != nil {
		t.Fatal(err)
	}

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

	if got, want := buf.Bytes(), expected; !reflect.DeepEqual(got, want) {
		t.Fatalf("wrote %v, want %v", got, want)
	}
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
	if err != nil {
		t.Fatal(err)
	}

	for _, pkt := range packets {
		if err = writer.WritePacket(pkt); err != nil {
			t.Fatal(err)
		}
	}

	reader, hdr2, err := NewReader(buf)
	if err != nil {
		t.Fatal(err)
	}

	if got, want := hdr2, hdr; !reflect.DeepEqual(got, want) {
		t.Fatalf("round trip: header=%v, want %v", got, want)
	}

	var packets2 []Packet
	for {
		pkt, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}

		packets2 = append(packets2, pkt)
	}

	if got, want := packets2, packets; !reflect.DeepEqual(got, want) {
		t.Fatalf("round trip: packets=%v, want %v", got, want)
	}
}
