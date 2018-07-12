package sctp

import (
	"encoding/binary"
	"fmt"

	"github.com/pkg/errors"
)

// ErrorCauseCode is a cause code that appears in either a ERROR or ABORT chunk
type ErrorCauseCode uint16

// ErrorCause interface
type ErrorCause interface {
	Unmarshal([]byte) error
	Marshal() ([]byte, error)
	Length() uint16

	errorCauseCode() ErrorCauseCode
}

// BuildErrorCause delegates the building of a error cause from raw bytes to the correct structure
func BuildErrorCause(raw []byte) (ErrorCause, error) {
	var e ErrorCause

	c := ErrorCauseCode(binary.BigEndian.Uint16(raw[0:]))
	switch c {
	case InvalidMandatoryParameter:
		e = &ErrorCauseInvalidMandatoryParameter{}
	case UnrecognizedChunkType:
		e = &ErrorCauseUnrecognizedChunkType{}
	default:
		return nil, errors.Errorf("BuildErrorCause does not handle %s", c.String())
	}

	if err := e.Unmarshal(raw); err != nil {
		return nil, err
	}
	return e, nil
}

// ErrorCause Codes
const (
	InvalidStreamIdentifier                ErrorCauseCode = 1
	MissingMandatoryParameter              ErrorCauseCode = 2
	StaleCookieError                       ErrorCauseCode = 3
	OutOfResource                          ErrorCauseCode = 4
	UnresolvableAddress                    ErrorCauseCode = 5
	UnrecognizedChunkType                  ErrorCauseCode = 6
	InvalidMandatoryParameter              ErrorCauseCode = 7
	UnrecognizedParameters                 ErrorCauseCode = 8
	NoUserData                             ErrorCauseCode = 9
	CookieReceivedWhileShuttingDown        ErrorCauseCode = 10
	RestartOfAnAssociationWithNewAddresses ErrorCauseCode = 11
	UserInitiatedAbort                     ErrorCauseCode = 12
	ProtocolViolation                      ErrorCauseCode = 13
)

func (e ErrorCauseCode) String() string {
	switch e {
	case InvalidStreamIdentifier:
		return "Invalid Stream Identifier"
	case MissingMandatoryParameter:
		return "Missing Mandatory Parameter"
	case StaleCookieError:
		return "Stale Cookie Error"
	case OutOfResource:
		return "Out Of Resource"
	case UnresolvableAddress:
		return "Unresolvable Address"
	case UnrecognizedChunkType:
		return "Unrecognized Chunk Type"
	case InvalidMandatoryParameter:
		return "Invalid Mandatory Parameter"
	case UnrecognizedParameters:
		return "Unrecognized Parameters"
	case NoUserData:
		return "No User Data"
	case CookieReceivedWhileShuttingDown:
		return "Cookie Received While Shutting Down"
	case RestartOfAnAssociationWithNewAddresses:
		return "Restart Of An Association With New Addresses"
	case UserInitiatedAbort:
		return "User Initiated Abort"
	case ProtocolViolation:
		return "Protocol Violation"
	default:
		return fmt.Sprintf("Unknown CauseCode: %d", e)
	}
}
