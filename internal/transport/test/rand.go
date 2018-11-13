package test

import (
	crand "crypto/rand"
	"fmt"
	mrand "math/rand"
)

var randomness []byte

func init() {
	// read 1MB of randomness
	randomness = make([]byte, 1<<20)
	if _, err := crand.Read(randomness); err != nil {
		fmt.Println("Failed to initiate randomness:", err)
	}
}

func randBuf(size int) ([]byte, error) {
	n := len(randomness) - size
	if size < 1 {
		return nil, fmt.Errorf("requested too large buffer (%d). max is %d", size, len(randomness))
	}

	start := mrand.Intn(n)
	return randomness[start : start+size], nil
}
