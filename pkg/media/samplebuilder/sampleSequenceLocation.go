// Package samplebuilder provides functionality to reconstruct media frames from RTP packets.
package samplebuilder

import "math"

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

func minUint32(x, y uint32) uint32 {
	if x < y {
		return x
	}
	return y
}

// Distance between two seqnums
func seqnumDistance32(x, y uint32) uint32 {
	diff := int32(x - y)
	if diff < 0 {
		return uint32(-diff)
	}

	return uint32(diff)
}

func (l sampleSequenceLocation) compare(pos uint16) int {
	if l.empty() {
		return slCompareVoid
	}

	head32 := uint32(l.head)
	count32 := uint32(l.count())
	tail32 := head32 + count32

	// pos32 is possibly two values, the normal value or a wrap
	// around the start value, figure out which it is...

	pos32Normal := uint32(pos)
	pos32Wrap := uint32(pos) + math.MaxUint16 + 1

	distNormal := minUint32(seqnumDistance32(head32, pos32Normal), seqnumDistance32(tail32, pos32Normal))
	distWrap := minUint32(seqnumDistance32(head32, pos32Wrap), seqnumDistance32(tail32, pos32Wrap))

	pos32 := pos32Normal
	if distWrap < distNormal {
		pos32 = pos32Wrap
	}

	if pos32 < head32 {
		return slCompareBefore
	}

	if pos32 >= tail32 {
		return slCompareAfter
	}

	return slCompareInside
}
