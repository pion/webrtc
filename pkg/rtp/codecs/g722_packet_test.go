package codecs

import (
	"bytes"
	"crypto/rand"
	"math"
	"testing"
)

func TestG722Payloader(t *testing.T) {
	p := G722Payloader{}

	const (
		testlen = 10000
		testmtu = 1500
	)

	//generate random 8-bit g722 samples
	samples := make([]byte, testlen)
	_, err := rand.Read(samples)

	if err != nil {
		t.Fatal("RNG Error: ", err)
	}

	//make a copy, for payloader input
	samplesIn := make([]byte, testlen)
	copy(samplesIn, samples)

	//split our samples into payloads
	payloads := p.Payload(testmtu, samplesIn)

	outcnt := int(math.Ceil(float64(testlen) / testmtu))
	if len(payloads) != outcnt {
		t.Fatalf("Generated %d payloads instead of %d", len(payloads), outcnt)
	}

	if !bytes.Equal(samplesIn, samples) {
		t.Fatal("Modified input samples")
	}

	samplesOut := bytes.Join(payloads, []byte{})

	if !bytes.Equal(samplesIn, samplesOut) {
		t.Fatal("Output samples don't match")
	}
}
