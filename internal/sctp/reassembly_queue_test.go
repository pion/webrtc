package sctp

import (
	"testing"

	"gotest.tools/assert"
)

func TestReassemblyQueue_push(t *testing.T) {
	r := &reassemblyQueue{}

	r.push(&chunkPayloadData{beginingFragment: true, tsn: 1, streamSequenceNumber: 0, userData: []byte{0}})
	r.push(&chunkPayloadData{tsn: 2, streamSequenceNumber: 0, userData: []byte{1}})
	r.push(&chunkPayloadData{tsn: 3, streamSequenceNumber: 0, userData: []byte{2}})
	r.push(&chunkPayloadData{endingFragment: true, tsn: 4, streamSequenceNumber: 0, userData: []byte{3}})

	b, ok := r.pop()
	if ok {
		assert.DeepEqual(t, b, []byte{0, 1, 2, 3})
	} else {
		t.Error("Unable to assemble message")
	}

	r.push(&chunkPayloadData{beginingFragment: true, tsn: 1, streamSequenceNumber: 1, userData: []byte{0}})
	r.push(&chunkPayloadData{tsn: 2, streamSequenceNumber: 1, userData: []byte{1}})

	r.push(&chunkPayloadData{unordered: true, beginingFragment: true, tsn: 1, streamSequenceNumber: 1, userData: []byte{0}})
	r.push(&chunkPayloadData{unordered: true, endingFragment: true, tsn: 2, streamSequenceNumber: 1, userData: []byte{1}})

	r.push(&chunkPayloadData{tsn: 3, streamSequenceNumber: 1, userData: []byte{2}})
	r.push(&chunkPayloadData{endingFragment: true, tsn: 4, streamSequenceNumber: 1, userData: []byte{3}})

	b, ok = r.pop()
	if ok {
		assert.DeepEqual(t, b, []byte{0, 1})
	} else {
		t.Error("Unable to assemble unordered message")
	}

	b, ok = r.pop()
	if ok {
		assert.DeepEqual(t, b, []byte{0, 1, 2, 3})
	} else {
		t.Error("Unable to assemble message after unordered message")
	}

}

func TestReassemblyQueue_clear(t *testing.T) {
	r := &reassemblyQueue{}

	r.push(&chunkPayloadData{beginingFragment: true, tsn: 1, streamSequenceNumber: 0, userData: []byte{0}})
	r.push(&chunkPayloadData{tsn: 2, streamSequenceNumber: 0, userData: []byte{1}})
	r.push(&chunkPayloadData{tsn: 3, streamSequenceNumber: 0, userData: []byte{2}})
	r.push(&chunkPayloadData{endingFragment: true, tsn: 4, streamSequenceNumber: 0, userData: []byte{3}})

}
