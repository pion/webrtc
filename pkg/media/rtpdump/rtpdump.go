// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

// Package rtpdump implements the RTPDump file format documented at
// https://www.cs.columbia.edu/irt/software/rtptools/
package rtpdump

import (
	"encoding/binary"
	"errors"
	"net"
	"time"
)

const (
	pktHeaderLen = 8
	headerLen    = 16
	preambleLen  = 36
)

var errMalformed = errors.New("malformed rtpdump")

// Header is the binary header at the top of the RTPDump file. It contains
// information about the source and start time of the packet stream included
// in the file.
type Header struct {
	// start of recording (GMT)
	Start time.Time
	// network source (multicast address)
	Source net.IP
	// UDP port
	Port uint16
}

// Marshal encodes the Header as binary.
func (h Header) Marshal() ([]byte, error) {
	data := make([]byte, headerLen)

	startNano := h.Start.UnixNano()
	startSec := uint32(startNano / int64(time.Second)) //nolint:gosec // G115
	startUsec := uint32(                               //nolint:gosec // G115
		(startNano % int64(time.Second)) / int64(time.Microsecond),
	)
	binary.BigEndian.PutUint32(data[0:], startSec)
	binary.BigEndian.PutUint32(data[4:], startUsec)

	source := h.Source.To4()
	copy(data[8:], source)

	binary.BigEndian.PutUint16(data[12:], h.Port)

	return data, nil
}

// Unmarshal decodes the Header from binary.
func (h *Header) Unmarshal(data []byte) error {
	if len(data) < headerLen {
		return errMalformed
	}

	// time as a `struct timeval`
	startSec := binary.BigEndian.Uint32(data[0:])
	startUsec := binary.BigEndian.Uint32(data[4:])
	h.Start = time.Unix(int64(startSec), int64(startUsec)*1e3).UTC()

	// ipv4 address
	h.Source = net.IPv4(data[8], data[9], data[10], data[11])

	h.Port = binary.BigEndian.Uint16(data[12:])

	// 2 bytes of padding (ignored)

	return nil
}

// Packet contains an RTP or RTCP packet along a time offset when it was logged
// (relative to the Start of the recording in Header). The Payload may contain
// truncated packets to support logging just the headers of RTP/RTCP packets.
type Packet struct {
	// Offset is the time since the start of recording in milliseconds
	Offset time.Duration
	// IsRTCP is true if the payload is RTCP, false if the payload is RTP
	IsRTCP bool
	// Payload is the binary RTP or RTCP payload. The contents may not parse
	// as a valid packet if the contents have been truncated.
	Payload []byte
}

// Marshal encodes the Packet as binary.
func (p Packet) Marshal() ([]byte, error) {
	packetLength := len(p.Payload)
	if p.IsRTCP {
		packetLength = 0
	}

	hdr := packetHeader{
		Length:       uint16(len(p.Payload)) + 8, //nolint:gosec // G115
		PacketLength: uint16(packetLength),       //nolint:gosec // G115
		Offset:       p.offsetMs(),
	}
	hdrData, err := hdr.Marshal()
	if err != nil {
		return nil, err
	}

	return append(hdrData, p.Payload...), nil
}

// Unmarshal decodes the Packet from binary.
func (p *Packet) Unmarshal(data []byte) error {
	var hdr packetHeader
	if err := hdr.Unmarshal(data); err != nil {
		return err
	}

	p.Offset = hdr.offset()
	p.IsRTCP = hdr.Length != 0 && hdr.PacketLength == 0

	if hdr.Length < 8 {
		return errMalformed
	}
	if len(data) < int(hdr.Length) {
		return errMalformed
	}
	p.Payload = data[8:hdr.Length]

	return nil
}

func (p *Packet) offsetMs() uint32 {
	return uint32(p.Offset / time.Millisecond) //nolint:gosec // G115
}

type packetHeader struct {
	// length of packet, including this header (may be smaller than
	// plen if not whole packet recorded)
	Length uint16
	// Actual header+payload length for RTP, 0 for RTCP
	PacketLength uint16
	// milliseconds since the start of recording
	Offset uint32
}

func (p packetHeader) Marshal() ([]byte, error) {
	d := make([]byte, pktHeaderLen)

	binary.BigEndian.PutUint16(d[0:], p.Length)
	binary.BigEndian.PutUint16(d[2:], p.PacketLength)
	binary.BigEndian.PutUint32(d[4:], p.Offset)

	return d, nil
}

func (p *packetHeader) Unmarshal(d []byte) error {
	if len(d) < pktHeaderLen {
		return errMalformed
	}

	p.Length = binary.BigEndian.Uint16(d[0:])
	p.PacketLength = binary.BigEndian.Uint16(d[2:])
	p.Offset = binary.BigEndian.Uint32(d[4:])

	return nil
}

func (p packetHeader) offset() time.Duration {
	return time.Duration(p.Offset) * time.Millisecond
}
