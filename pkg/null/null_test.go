package null

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewBool(t *testing.T) {
	value := bool(true)
	nullable := NewBool(value)

	assert.Equal(t,
		true,
		nullable.Valid,
		"valid: Bool",
	)

	assert.Equal(t,
		value,
		nullable.Bool,
		"value: Bool",
	)
}

func TestNewByte(t *testing.T) {
	value := byte('a')
	nullable := NewByte(value)

	assert.Equal(t,
		true,
		nullable.Valid,
		"valid: Byte",
	)

	assert.Equal(t,
		value,
		nullable.Byte,
		"value: Byte",
	)
}

func TestNewComplex128(t *testing.T) {
	value := complex128(-5 + 12i)
	nullable := NewComplex128(value)

	assert.Equal(t,
		true,
		nullable.Valid,
		"valid: Complex128",
	)

	assert.Equal(t,
		value,
		nullable.Complex128,
		"value: Complex128",
	)
}

func TestNewComplex64(t *testing.T) {
	value := complex64(-5 + 12i)
	nullable := NewComplex64(value)

	assert.Equal(t,
		true,
		nullable.Valid,
		"valid: Complex64",
	)

	assert.Equal(t,
		value,
		nullable.Complex64,
		"value: Complex64",
	)
}

func TestNewFloat32(t *testing.T) {
	value := float32(0.5)
	nullable := NewFloat32(value)

	assert.Equal(t,
		true,
		nullable.Valid,
		"valid: Float32",
	)

	assert.Equal(t,
		value,
		nullable.Float32,
		"value: Float32",
	)
}

func TestNewFloat64(t *testing.T) {
	value := float64(0.5)
	nullable := NewFloat64(value)

	assert.Equal(t,
		true,
		nullable.Valid,
		"valid: Float64",
	)

	assert.Equal(t,
		value,
		nullable.Float64,
		"value: Float64",
	)
}

func TestNewInt(t *testing.T) {
	value := int(1)
	nullable := NewInt(value)

	assert.Equal(t,
		true,
		nullable.Valid,
		"valid: Int",
	)

	assert.Equal(t,
		value,
		nullable.Int,
		"value: Int",
	)
}

func TestNewInt16(t *testing.T) {
	value := int16(1)
	nullable := NewInt16(value)

	assert.Equal(t,
		true,
		nullable.Valid,
		"valid: Int16",
	)

	assert.Equal(t,
		value,
		nullable.Int16,
		"value: Int16",
	)
}

func TestNewInt32(t *testing.T) {
	value := int32(1)
	nullable := NewInt32(value)

	assert.Equal(t,
		true,
		nullable.Valid,
		"valid: Int32",
	)

	assert.Equal(t,
		value,
		nullable.Int32,
		"value: Int32",
	)
}

func TestNewInt64(t *testing.T) {
	value := int64(1)
	nullable := NewInt64(value)

	assert.Equal(t,
		true,
		nullable.Valid,
		"valid: Int64",
	)

	assert.Equal(t,
		value,
		nullable.Int64,
		"value: Int64",
	)
}

func TestNewInt8(t *testing.T) {
	value := int8(1)
	nullable := NewInt8(value)

	assert.Equal(t,
		true,
		nullable.Valid,
		"valid: Int8",
	)

	assert.Equal(t,
		value,
		nullable.Int8,
		"value: Int8",
	)
}

func TestNewRune(t *testing.T) {
	value := rune('p')
	nullable := NewRune(value)

	assert.Equal(t,
		true,
		nullable.Valid,
		"valid: Rune",
	)

	assert.Equal(t,
		value,
		nullable.Rune,
		"value: Rune",
	)
}

func TestNewString(t *testing.T) {
	value := string("pions")
	nullable := NewString(value)

	assert.Equal(t,
		true,
		nullable.Valid,
		"valid: String",
	)

	assert.Equal(t,
		value,
		nullable.String,
		"value: String",
	)
}

func TestNewUint(t *testing.T) {
	value := uint(1)
	nullable := NewUint(value)

	assert.Equal(t,
		true,
		nullable.Valid,
		"valid: Uint",
	)

	assert.Equal(t,
		value,
		nullable.Uint,
		"value: Uint",
	)
}

func TestNewUint16(t *testing.T) {
	value := uint16(1)
	nullable := NewUint16(value)

	assert.Equal(t,
		true,
		nullable.Valid,
		"valid: Uint16",
	)

	assert.Equal(t,
		value,
		nullable.Uint16,
		"value: Uint16",
	)
}

func TestNewUint32(t *testing.T) {
	value := uint32(1)
	nullable := NewUint32(value)

	assert.Equal(t,
		true,
		nullable.Valid,
		"valid: Uint32",
	)

	assert.Equal(t,
		value,
		nullable.Uint32,
		"value: Uint32",
	)
}

func TestNewUint64(t *testing.T) {
	value := uint64(1)
	nullable := NewUint64(value)

	assert.Equal(t,
		true,
		nullable.Valid,
		"valid: Uint64",
	)

	assert.Equal(t,
		value,
		nullable.Uint64,
		"value: Uint64",
	)
}

func TestNewUint8(t *testing.T) {
	value := uint8(1)
	nullable := NewUint8(value)

	assert.Equal(t,
		true,
		nullable.Valid,
		"valid: Uint8",
	)

	assert.Equal(t,
		value,
		nullable.Uint8,
		"value: Uint8",
	)
}
