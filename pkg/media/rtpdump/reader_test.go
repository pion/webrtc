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

func TestReader(t *testing.T) {
	validPreamble := []byte("#!rtpplay1.0 224.2.0.1/3456\n")

	for _, test := range []struct {
		Name        string
		Data        []byte
		WantHeader  Header
		WantPackets []Packet
		WantErr     error
	}{
		{
			Name:    "empty",
			Data:    nil,
			WantErr: errMalformed,
		},
		{
			Name: "hashbang missing ip/port",
			Data: append(
				[]byte("#!rtpplay1.0 \n"),
				0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00,
			),
			WantErr: errMalformed,
		},
		{
			Name: "hashbang missing port",
			Data: append(
				[]byte("#!rtpplay1.0 0.0.0.0\n"),
				0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00,
			),
			WantErr: errMalformed,
		},
		{
			Name: "valid empty file",
			Data: append(
				validPreamble,
				0x00, 0x00, 0x00, 0x01,
				0x00, 0x00, 0x00, 0x00,
				0x01, 0x01, 0x01, 0x01,
				0x22, 0xB8, 0x00, 0x00,
			),
			WantHeader: Header{
				Start:  time.Unix(1, 0).UTC(),
				Source: net.IPv4(1, 1, 1, 1),
				Port:   8888,
			},
		},
		{
			Name: "malformed packet header",
			Data: append(
				validPreamble,
				// header
				0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00,
				// packet header
				0x00,
			),
			WantHeader: Header{
				Start:  time.Unix(0, 0).UTC(),
				Source: net.IPv4(0, 0, 0, 0),
				Port:   0,
			},
			WantErr: errMalformed,
		},
		{
			Name: "short packet payload",
			Data: append(
				validPreamble,
				// header
				0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00,
				// packet header len=1048575
				0xFF, 0xFF, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00,
				// packet payload
				0x00,
			),
			WantHeader: Header{
				Start:  time.Unix(0, 0).UTC(),
				Source: net.IPv4(0, 0, 0, 0),
				Port:   0,
			},
			WantErr: errMalformed,
		},
		{
			Name: "empty packet payload",
			Data: append(
				validPreamble,
				// header
				0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00,
				// packet header len=0
				0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00,
			),
			WantHeader: Header{
				Start:  time.Unix(0, 0).UTC(),
				Source: net.IPv4(0, 0, 0, 0),
				Port:   0,
			},
			WantErr: errMalformed,
		},
		{
			Name: "valid rtcp packet",
			Data: append(
				validPreamble,
				// header
				0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00,
				// packet header len=20, pLen=0, off=1
				0x00, 0x14, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x01,
				// packet payload (BYE)
				0x81, 0xcb, 0x00, 0x0c,
				0x90, 0x2f, 0x9e, 0x2e,
				0x03, 0x46, 0x4f, 0x4f,
			),
			WantHeader: Header{
				Start:  time.Unix(0, 0).UTC(),
				Source: net.IPv4(0, 0, 0, 0),
				Port:   0,
			},
			WantPackets: []Packet{
				{
					Offset: time.Millisecond,
					IsRTCP: true,
					Payload: []byte{
						0x81, 0xcb, 0x00, 0x0c,
						0x90, 0x2f, 0x9e, 0x2e,
						0x03, 0x46, 0x4f, 0x4f,
					},
				},
			},
			WantErr: nil,
		},
		{
			Name: "truncated rtcp packet",
			Data: append(
				validPreamble,
				// header
				0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00,
				// packet header len=9, pLen=0, off=1
				0x00, 0x09, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x01,
				// invalid payload
				0x81,
			),
			WantHeader: Header{
				Start:  time.Unix(0, 0).UTC(),
				Source: net.IPv4(0, 0, 0, 0),
				Port:   0,
			},
			WantPackets: []Packet{
				{
					Offset:  time.Millisecond,
					IsRTCP:  true,
					Payload: []byte{0x81},
				},
			},
		},
		{
			Name: "two valid packets",
			Data: append(
				validPreamble,
				// header
				0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00,
				// packet header len=20, pLen=0, off=1
				0x00, 0x14, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x01,
				// packet payload (BYE)
				0x81, 0xcb, 0x00, 0x0c,
				0x90, 0x2f, 0x9e, 0x2e,
				0x03, 0x46, 0x4f, 0x4f,
				// packet header len=33, pLen=0, off=2
				0x00, 0x21, 0x00, 0x19,
				0x00, 0x00, 0x00, 0x02,
				// packet payload (RTP)
				0x90, 0x60, 0x69, 0x8f,
				0xd9, 0xc2, 0x93, 0xda,
				0x1c, 0x64, 0x27, 0x82,
				0x00, 0x01, 0x00, 0x01,
				0xFF, 0xFF, 0xFF, 0xFF,
				0x98, 0x36, 0xbe, 0x88,
				0x9e,
			),
			WantHeader: Header{
				Start:  time.Unix(0, 0).UTC(),
				Source: net.IPv4(0, 0, 0, 0),
				Port:   0,
			},
			WantPackets: []Packet{
				{
					Offset: time.Millisecond,
					IsRTCP: true,
					Payload: []byte{
						0x81, 0xcb, 0x00, 0x0c,
						0x90, 0x2f, 0x9e, 0x2e,
						0x03, 0x46, 0x4f, 0x4f,
					},
				},
				{
					Offset: 2 * time.Millisecond,
					IsRTCP: false,
					Payload: []byte{
						0x90, 0x60, 0x69, 0x8f,
						0xd9, 0xc2, 0x93, 0xda,
						0x1c, 0x64, 0x27, 0x82,
						0x00, 0x01, 0x00, 0x01,
						0xFF, 0xFF, 0xFF, 0xFF,
						0x98, 0x36, 0xbe, 0x88,
						0x9e,
					},
				},
			},
			WantErr: nil,
		},
	} {
		r, hdr, err := NewReader(bytes.NewReader(test.Data))
		if err != nil {
			if got, want := err, test.WantErr; !errors.Is(got, want) {
				t.Fatalf("NewReader(%s) err=%v want %v", test.Name, got, want)
			}
			continue
		}

		if got, want := hdr, test.WantHeader; !reflect.DeepEqual(got, want) {
			t.Fatalf("%q Header = %#v, want %#v", test.Name, got, want)
		}

		var nextErr error
		var packets []Packet
		for {
			pkt, err := r.Next()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				nextErr = err
				break
			}

			packets = append(packets, pkt)
		}

		if got, want := nextErr, test.WantErr; !errors.Is(got, want) {
			t.Fatalf("%s err=%v want %v", test.Name, got, want)
		}
		if got, want := packets, test.WantPackets; !reflect.DeepEqual(got, want) {
			t.Fatalf("%q packets=%#v, want %#v", test.Name, got, want)
		}
	}
}
