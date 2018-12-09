package sctp

import (
	"testing"

	"gotest.tools/assert"
)

func TestReassemblyQueue_push(t *testing.T) {
	r := &reassemblyQueue{}

	r.push(&chunkPayloadData{payloadType: PayloadTypeWebRTCBinary, beginingFragment: true, tsn: 1, streamSequenceNumber: 0, userData: []byte{0}})
	r.push(&chunkPayloadData{payloadType: PayloadTypeWebRTCBinary, tsn: 2, streamSequenceNumber: 0, userData: []byte{1}})
	r.push(&chunkPayloadData{payloadType: PayloadTypeWebRTCBinary, tsn: 3, streamSequenceNumber: 0, userData: []byte{2}})
	r.push(&chunkPayloadData{payloadType: PayloadTypeWebRTCBinary, endingFragment: true, tsn: 4, streamSequenceNumber: 0, userData: []byte{3}})

	b, ppi, ok := r.pop()
	if ok {
		assert.Equal(t, ppi, PayloadTypeWebRTCBinary)
		assert.DeepEqual(t, b, []byte{0, 1, 2, 3})
	} else {
		t.Error("Unable to assemble message")
	}

	r.push(&chunkPayloadData{payloadType: PayloadTypeWebRTCBinary, beginingFragment: true, tsn: 1, streamSequenceNumber: 1, userData: []byte{0}})
	r.push(&chunkPayloadData{payloadType: PayloadTypeWebRTCBinary, tsn: 2, streamSequenceNumber: 1, userData: []byte{1}})

	r.push(&chunkPayloadData{payloadType: PayloadTypeWebRTCBinary, unordered: true, beginingFragment: true, tsn: 1, streamSequenceNumber: 1, userData: []byte{0}})
	r.push(&chunkPayloadData{payloadType: PayloadTypeWebRTCBinary, unordered: true, endingFragment: true, tsn: 2, streamSequenceNumber: 1, userData: []byte{1}})

	r.push(&chunkPayloadData{payloadType: PayloadTypeWebRTCBinary, tsn: 3, streamSequenceNumber: 1, userData: []byte{2}})
	r.push(&chunkPayloadData{payloadType: PayloadTypeWebRTCBinary, endingFragment: true, tsn: 4, streamSequenceNumber: 1, userData: []byte{3}})

	b, ppi, ok = r.pop()
	if ok {
		assert.Equal(t, ppi, PayloadTypeWebRTCBinary)
		assert.DeepEqual(t, b, []byte{0, 1})
	} else {
		t.Error("Unable to assemble unordered message")
	}

	b, ppi, ok = r.pop()
	if ok {
		assert.Equal(t, ppi, PayloadTypeWebRTCBinary)
		assert.DeepEqual(t, b, []byte{0, 1, 2, 3})
	} else {
		t.Error("Unable to assemble message after unordered message")
	}
}

func TestReassemblyQueue_clear(t *testing.T) {
	r := &reassemblyQueue{}

	r.push(&chunkPayloadData{payloadType: PayloadTypeWebRTCBinary, beginingFragment: true, tsn: 1, streamSequenceNumber: 0, userData: []byte{0}})
	r.push(&chunkPayloadData{payloadType: PayloadTypeWebRTCBinary, tsn: 2, streamSequenceNumber: 0, userData: []byte{1}})
	r.push(&chunkPayloadData{payloadType: PayloadTypeWebRTCBinary, tsn: 3, streamSequenceNumber: 0, userData: []byte{2}})
	r.push(&chunkPayloadData{payloadType: PayloadTypeWebRTCBinary, endingFragment: true, tsn: 4, streamSequenceNumber: 0, userData: []byte{3}})
}
