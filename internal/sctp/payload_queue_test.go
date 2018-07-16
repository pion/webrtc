package sctp

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func makePayload(tsn uint32) *PayloadData {
	return &PayloadData{TSN: tsn}
}

func TestPayloadQueue_GetGapAckBlocks(t *testing.T) {
	pq := &PayloadQueue{}
	pq.Push(makePayload(1), 0)
	pq.Push(makePayload(2), 0)
	pq.Push(makePayload(3), 0)
	pq.Push(makePayload(4), 0)
	pq.Push(makePayload(5), 0)
	pq.Push(makePayload(6), 0)

	gab1 := []*GapAckBlock{&GapAckBlock{1, 6}}
	gab2 := pq.GetGapAckBlocks(0)
	assert.NotNil(t, gab2)
	assert.Len(t, gab2, 1)

	assert.Equal(t, gab1[0].start, gab2[0].start)
	assert.Equal(t, gab1[0].end, gab2[0].end)

	pq.Push(makePayload(8), 0)
	pq.Push(makePayload(9), 0)

	gab1 = []*GapAckBlock{&GapAckBlock{1, 6}, &GapAckBlock{8, 9}}
	gab2 = pq.GetGapAckBlocks(0)
	assert.NotNil(t, gab2)
	assert.Len(t, gab2, 2)

	assert.Equal(t, gab1[0].start, gab2[0].start)
	assert.Equal(t, gab1[0].end, gab2[0].end)
	assert.Equal(t, gab1[1].start, gab2[1].start)
	assert.Equal(t, gab1[1].end, gab2[1].end)
}
