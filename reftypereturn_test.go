package webrtc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBool(t *testing.T) {
	var actual *bool
	assert.Nil(t, actual)

	actual = RefBool(true)
	assert.NotNil(t, actual)
	assert.Equal(t, true, *actual)
}

func TestUint(t *testing.T) {
	var actual *uint
	assert.Nil(t, actual)

	actual = RefUint(1)
	assert.NotNil(t, actual)
	assert.Equal(t, uint(1), *actual)
}

func TestUint8(t *testing.T) {
	var actual *uint8
	assert.Nil(t, actual)

	actual = RefUint8(1)
	assert.NotNil(t, actual)
	assert.Equal(t, uint8(1), *actual)
}

func TestUint16(t *testing.T) {
	var actual *uint16
	assert.Nil(t, actual)

	actual = RefUint16(1)
	assert.NotNil(t, actual)
	assert.Equal(t, uint16(1), *actual)
}

func TestUint32(t *testing.T) {
	var actual *uint32
	assert.Nil(t, actual)

	actual = RefUint32(1)
	assert.NotNil(t, actual)
	assert.Equal(t, uint32(1), *actual)
}

func TestUint64(t *testing.T) {
	var actual *uint64
	assert.Nil(t, actual)

	actual = RefUint64(1)
	assert.NotNil(t, actual)
	assert.Equal(t, uint64(1), *actual)
}

func TestInt(t *testing.T) {
	var actual *int
	assert.Nil(t, actual)

	actual = RefInt(1)
	assert.NotNil(t, actual)
	assert.Equal(t, int(1), *actual)
}

func TestInt8(t *testing.T) {
	var actual *int8
	assert.Nil(t, actual)

	actual = RefInt8(1)
	assert.NotNil(t, actual)
	assert.Equal(t, int8(1), *actual)
}

func TestInt16(t *testing.T) {
	var actual *int16
	assert.Nil(t, actual)

	actual = RefInt16(1)
	assert.NotNil(t, actual)
	assert.Equal(t, int16(1), *actual)
}

func TestInt32(t *testing.T) {
	var actual *int32
	assert.Nil(t, actual)

	actual = RefInt32(1)
	assert.NotNil(t, actual)
	assert.Equal(t, int32(1), *actual)
}

func TestInt64(t *testing.T) {
	var actual *int64
	assert.Nil(t, actual)

	actual = RefInt64(1)
	assert.NotNil(t, actual)
	assert.Equal(t, int64(1), *actual)
}

func TestFloat32(t *testing.T) {
	var actual *float32
	assert.Nil(t, actual)

	actual = RefFloat32(1)
	assert.NotNil(t, actual)
	assert.Equal(t, float32(1), *actual)
}

func TestFloat64(t *testing.T) {
	var actual *float64
	assert.Nil(t, actual)

	actual = RefFloat64(1)
	assert.NotNil(t, actual)
	assert.Equal(t, float64(1), *actual)
}

func TestComplex64(t *testing.T) {
	var actual *complex64
	assert.Nil(t, actual)

	actual = RefComplex64(1)
	assert.NotNil(t, actual)
	assert.Equal(t, complex64(1), *actual)
}

func TestComplex128(t *testing.T) {
	var actual *complex128
	assert.Nil(t, actual)

	actual = RefComplex128(1)
	assert.NotNil(t, actual)
	assert.Equal(t, complex128(1), *actual)
}

func TestByte(t *testing.T) {
	var actual *byte
	assert.Nil(t, actual)

	actual = RefByte(1)
	assert.NotNil(t, actual)
	assert.Equal(t, byte(1), *actual)
}

func TestRune(t *testing.T) {
	var actual *rune
	assert.Nil(t, actual)

	actual = RefRune(1)
	assert.NotNil(t, actual)
	assert.Equal(t, rune(1), *actual)
}

func TestString(t *testing.T) {
	var actual *string
	assert.Nil(t, actual)

	actual = RefString("unittest")
	assert.NotNil(t, actual)
	assert.Equal(t, "unittest", *actual)
}
