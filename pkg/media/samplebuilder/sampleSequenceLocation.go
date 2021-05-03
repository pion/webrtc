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

/*
func (l sampleSequenceLocation) adjacent(after sampleSequenceLocation) bool {
	if !l.hasData() {
		return false
	}
	if !after.hasData() {
		return false
	}
	return l.tail == after.head
}

func (l sampleSequenceLocation) reset() {
	l.head, l.tail = 0, 0
}

func (l sampleSequenceLocation) flush() {
	l.tail = l.head
}
*/

const (
	slCompareVoid = iota
	slCompareBefore
	slCompareInside
	slCompareAfter
)

/*
func minUint16(x, y uint16) uint16 {
	if x < y {
		return x
	}
	return y
}
*/

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

/*
func (l sampleSequenceLocation) calculateOverlap(other sampleSequenceLocation) sampleSequenceLocation {
	if l.empty() {
		return sampleSequenceLocation{}
	}
	if other.empty() {
		return sampleSequenceLocation{}
	}

	lHead32 := uint32(l.head)
	rHead32 := uint32(other.head)

	lCount32 := uint32(l.head) + uint32(l.count())
	rCount32 := uint32(other.head) + uint32(other.count())

	// make the lHead always be first in the overlap
	if lHead32 > rHead32 {
		lHead32, rHead32 = rHead32, lHead32
		lCount32, rCount32 = rCount32, lCount32
	}

	lTail32 := lHead32 + lCount32
	rTail32 := rHead32 + rCount32

	// if the right starts after the left there is no overlap
	if rHead32 >= lTail32 {
		return sampleSequenceLocation{}
	}

	// calculate an alternative possilbe tail (depending which tail ends first)
	newPossibleTail := rHead32 + (lCount32 - (rHead32 - lHead32))

	return sampleSequenceLocation{head: uint16(rHead32), tail: uint16(minUint32(newPossibleTail, rTail32))}
}

*/
