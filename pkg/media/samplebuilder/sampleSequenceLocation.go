// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

// Package samplebuilder provides functionality to reconstruct media frames from RTP packets.
package samplebuilder

type sampleSequenceLocation struct {
	// head is the first packet in a sequence
	head uint16
	// tail is always set to one after the final sequence number,
	// so if head == tail then the sequence is empty
	tail uint16
}

func (l sampleSequenceLocation) empty() bool {
	return l.head == l.tail
}

func (l sampleSequenceLocation) hasData() bool {
	return l.head != l.tail
}

func (l sampleSequenceLocation) count() uint16 {
	return seqnumDistance(l.head, l.tail)
}

const (
	slCompareVoid = iota
	slCompareBefore
	slCompareInside
	slCompareAfter
)

func (l sampleSequenceLocation) compare(pos uint16) int {
	if l.head == l.tail {
		return slCompareVoid
	}

	if l.head < l.tail {
		if l.head <= pos && pos < l.tail {
			return slCompareInside
		}
	} else {
		if l.head <= pos || pos < l.tail {
			return slCompareInside
		}
	}

	if l.head-pos <= pos-l.tail {
		return slCompareBefore
	}
	return slCompareAfter
}
