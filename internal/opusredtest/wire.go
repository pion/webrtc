//go:build opusred && !js

// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

// Package opusredtest supplies independent evidence for Opus RED qualification.
package opusredtest

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/pion/rtp"
)

// PacketRecord identifies media without retaining caller-owned buffers.
type PacketRecord struct {
	Sequence      uint16 `json:"sequence"`
	Timestamp     uint32 `json:"timestamp"`
	SSRC          uint32 `json:"ssrc"`
	PayloadType   uint8  `json:"payloadType"`
	PayloadLength int    `json:"payloadLength"`
	PayloadSHA256 string `json:"payloadSHA256"`
	RTPSize       int    `json:"rtpSize"`
}

// BlockRecord records an independently decoded redundant block.
type BlockRecord struct {
	PayloadType     uint8  `json:"payloadType"`
	TimestampOffset uint16 `json:"timestampOffset"`
	PayloadLength   int    `json:"payloadLength"`
	PayloadSHA256   string `json:"payloadSHA256"`
}

// WireRecord records one physical RTP packet, rather than reconstructed media.
type WireRecord struct {
	PacketRecord
	Primary PacketRecord  `json:"primary"`
	Blocks  []BlockRecord `json:"blocks"`
}

func payloadHash(payload []byte) string {
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:])
}

// NewPacketRecord snapshots identifying fields and a payload digest.
func NewPacketRecord(header *rtp.Header, payload []byte) PacketRecord {
	record := PacketRecord{PayloadLength: len(payload), PayloadSHA256: payloadHash(payload)}
	if header != nil {
		record.Sequence = header.SequenceNumber
		record.Timestamp = header.Timestamp
		record.SSRC = header.SSRC
		record.PayloadType = header.PayloadType
		record.RTPSize = header.MarshalSize() + len(payload) + int(header.PaddingSize)
	}
	return record
}

// ParseWire independently decodes RFC2198 without using the implementation under test.
func ParseWire(header *rtp.Header, payload []byte, mapping Mapping) (WireRecord, error) {
	if header == nil {
		return WireRecord{}, fmt.Errorf("missing RTP header")
	}
	record := WireRecord{PacketRecord: NewPacketRecord(header, payload), Blocks: []BlockRecord{}}
	if !mapping.HasRED || header.PayloadType != mapping.REDPT {
		record.Primary = record.PacketRecord
		return record, nil
	}
	type descriptor struct {
		pt     uint8
		offset uint16
		length int
	}
	descriptors := make([]descriptor, 0, 2)
	cursor := 0
	var primaryPT uint8
	for {
		if cursor >= len(payload) {
			return WireRecord{}, fmt.Errorf("RED missing primary header")
		}
		value := payload[cursor]
		if value&128 == 0 {
			primaryPT = value
			cursor++
			break
		}
		if len(descriptors) >= 32 {
			return WireRecord{}, fmt.Errorf("RED exceeds 32 redundant blocks")
		}
		if len(payload)-cursor < 4 {
			return WireRecord{}, fmt.Errorf("RED truncated block header")
		}
		descriptors = append(descriptors, descriptor{
			pt:     value & 127,
			offset: uint16(payload[cursor+1])*64 + uint16(payload[cursor+2]/4),
			length: int(payload[cursor+2]%4)*256 + int(payload[cursor+3]),
		})
		cursor += 4
	}
	for _, descriptor := range descriptors {
		if descriptor.length > len(payload)-cursor {
			return WireRecord{}, fmt.Errorf("RED block exceeds packet payload")
		}
		block := payload[cursor : cursor+descriptor.length]
		record.Blocks = append(record.Blocks, BlockRecord{
			PayloadType: descriptor.pt, TimestampOffset: descriptor.offset,
			PayloadLength: len(block), PayloadSHA256: payloadHash(block),
		})
		cursor += descriptor.length
	}
	if primaryPT != mapping.OpusPT {
		return WireRecord{}, fmt.Errorf("RED primary PT %d differs from Opus PT %d", primaryPT, mapping.OpusPT)
	}
	if cursor == len(payload) {
		return WireRecord{}, fmt.Errorf("RED primary payload is empty")
	}
	primaryHeader := header.Clone()
	primaryHeader.PayloadType = primaryPT
	record.Primary = NewPacketRecord(&primaryHeader, payload[cursor:])
	return record, nil
}
