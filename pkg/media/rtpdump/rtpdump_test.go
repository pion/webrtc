// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package rtpdump

import (
	"errors"
	"net"
	"reflect"
	"testing"
	"time"
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
		if err != nil {
			t.Fatal(err)
		}

		var hdr Header
		if err := hdr.Unmarshal(d); err != nil {
			t.Fatal(err)
		}

		if got, want := hdr, test.Header; !reflect.DeepEqual(got, want) {
			t.Fatalf("Unmarshal(%v.Marshal()) = %v, want identical", got, want)
		}
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
		if got, want := err, test.WantErr; !errors.Is(got, want) {
			t.Fatalf("Marshal(%q) err=%v, want %v", test.Name, got, want)
		}

		if got, want := data, test.Want; !reflect.DeepEqual(got, want) {
			t.Fatalf("Marshal(%q) = %v, want %v", test.Name, got, want)
		}
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
		d, err := test.Packet.Marshal()
		if err != nil {
			t.Fatal(err)
		}

		var pkt Packet
		if err := pkt.Unmarshal(d); err != nil {
			t.Fatal(err)
		}

		if got, want := pkt, test.Packet; !reflect.DeepEqual(got, want) {
			t.Fatalf("Unmarshal(%v.Marshal()) = %v, want identical", got, want)
		}
	}
}
