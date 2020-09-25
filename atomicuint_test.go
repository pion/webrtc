package webrtc

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestNewAtomicUint32(t *testing.T) {

	for index := 0; index < 100; index++ {
		value := NewAtomicUint32()
		assert.Equal(t, value.value(), uint32(0))

		value.increment()
		assert.Equal(t, value.value(), uint32(1))

		value.add(uint32(4))
		assert.Equal(t, value.value(), uint32(5))

		value.increment()
		assert.Equal(t, value.value(), uint32(6))
	}
}
