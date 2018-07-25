package datachannel

import "fmt"

// PayloadType are the different types of data that can be
// represented in a DataChannel message
type PayloadType int

// PayloadType enums
const (
	PayloadTypeString = iota + 1
	PayloadTypeBinary
)

func (p PayloadType) String() string {
	switch p {
	case PayloadTypeString:
		return "Payload Type String"
	case PayloadTypeBinary:
		return "Payload Type Binary"
	default:
		return fmt.Sprintf("Invalid PayloadType (%d)", p)
	}
}

// Payload is the body of a DataChannel message
type Payload interface {
	PayloadType() PayloadType
}

// PayloadString is a string DataChannel message
type PayloadString struct {
	Data []byte
}

//PayloadType returns the type of payload
func (p PayloadString) PayloadType() PayloadType {
	return PayloadTypeString
}

// PayloadBinary is a binary DataChannel message
type PayloadBinary struct {
	Data []byte
}

//PayloadType returns the type of payload
func (p PayloadBinary) PayloadType() PayloadType {
	return PayloadTypeBinary
}
