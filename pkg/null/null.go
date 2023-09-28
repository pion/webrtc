// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

// Package null is used to represent values where the 0 value is significant
// This pattern is common in ECMAScript, this allows us to maintain a matching API
package null

// Bool is used to represent a bool that may be null
type Bool struct {
	Valid bool
	Bool  bool
}

// NewBool turns a bool into a valid null.Bool
func NewBool(value bool) Bool {
	return Bool{Valid: true, Bool: value}
}

// Byte is used to represent a byte that may be null
type Byte struct {
	Valid bool
	Byte  byte
}

// NewByte turns a byte into a valid null.Byte
func NewByte(value byte) Byte {
	return Byte{Valid: true, Byte: value}
}

// Complex128 is used to represent a complex128 that may be null
type Complex128 struct {
	Valid      bool
	Complex128 complex128
}

// NewComplex128 turns a complex128 into a valid null.Complex128
func NewComplex128(value complex128) Complex128 {
	return Complex128{Valid: true, Complex128: value}
}

// Complex64 is used to represent a complex64 that may be null
type Complex64 struct {
	Valid     bool
	Complex64 complex64
}

// NewComplex64 turns a complex64 into a valid null.Complex64
func NewComplex64(value complex64) Complex64 {
	return Complex64{Valid: true, Complex64: value}
}

// Float32 is used to represent a float32 that may be null
type Float32 struct {
	Valid   bool
	Float32 float32
}

// NewFloat32 turns a float32 into a valid null.Float32
func NewFloat32(value float32) Float32 {
	return Float32{Valid: true, Float32: value}
}

// Float64 is used to represent a float64 that may be null
type Float64 struct {
	Valid   bool
	Float64 float64
}

// NewFloat64 turns a float64 into a valid null.Float64
func NewFloat64(value float64) Float64 {
	return Float64{Valid: true, Float64: value}
}

// Int is used to represent a int that may be null
type Int struct {
	Valid bool
	Int   int
}

// NewInt turns a int into a valid null.Int
func NewInt(value int) Int {
	return Int{Valid: true, Int: value}
}

// Int16 is used to represent a int16 that may be null
type Int16 struct {
	Valid bool
	Int16 int16
}

// NewInt16 turns a int16 into a valid null.Int16
func NewInt16(value int16) Int16 {
	return Int16{Valid: true, Int16: value}
}

// Int32 is used to represent a int32 that may be null
type Int32 struct {
	Valid bool
	Int32 int32
}

// NewInt32 turns a int32 into a valid null.Int32
func NewInt32(value int32) Int32 {
	return Int32{Valid: true, Int32: value}
}

// Int64 is used to represent a int64 that may be null
type Int64 struct {
	Valid bool
	Int64 int64
}

// NewInt64 turns a int64 into a valid null.Int64
func NewInt64(value int64) Int64 {
	return Int64{Valid: true, Int64: value}
}

// Int8 is used to represent a int8 that may be null
type Int8 struct {
	Valid bool
	Int8  int8
}

// NewInt8 turns a int8 into a valid null.Int8
func NewInt8(value int8) Int8 {
	return Int8{Valid: true, Int8: value}
}

// Rune is used to represent a rune that may be null
type Rune struct {
	Valid bool
	Rune  rune
}

// NewRune turns a rune into a valid null.Rune
func NewRune(value rune) Rune {
	return Rune{Valid: true, Rune: value}
}

// String is used to represent a string that may be null
type String struct {
	Valid  bool
	String string
}

// NewString turns a string into a valid null.String
func NewString(value string) String {
	return String{Valid: true, String: value}
}

// Uint is used to represent a uint that may be null
type Uint struct {
	Valid bool
	Uint  uint
}

// NewUint turns a uint into a valid null.Uint
func NewUint(value uint) Uint {
	return Uint{Valid: true, Uint: value}
}

// Uint16 is used to represent a uint16 that may be null
type Uint16 struct {
	Valid  bool
	Uint16 uint16
}

// NewUint16 turns a uint16 into a valid null.Uint16
func NewUint16(value uint16) Uint16 {
	return Uint16{Valid: true, Uint16: value}
}

// Uint32 is used to represent a uint32 that may be null
type Uint32 struct {
	Valid  bool
	Uint32 uint32
}

// NewUint32 turns a uint32 into a valid null.Uint32
func NewUint32(value uint32) Uint32 {
	return Uint32{Valid: true, Uint32: value}
}

// Uint64 is used to represent a uint64 that may be null
type Uint64 struct {
	Valid  bool
	Uint64 uint64
}

// NewUint64 turns a uint64 into a valid null.Uint64
func NewUint64(value uint64) Uint64 {
	return Uint64{Valid: true, Uint64: value}
}

// Uint8 is used to represent a uint8 that may be null
type Uint8 struct {
	Valid bool
	Uint8 uint8
}

// NewUint8 turns a uint8 into a valid null.Uint8
func NewUint8(value uint8) Uint8 {
	return Uint8{Valid: true, Uint8: value}
}
