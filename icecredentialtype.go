package webrtc

import (
	"encoding/json"
	"fmt"
)

// ICECredentialType indicates the type of credentials used to connect to
// an ICE server.
type ICECredentialType int

const (
	// ICECredentialTypePassword describes username and password based
	// credentials as described in https://tools.ietf.org/html/rfc5389.
	ICECredentialTypePassword ICECredentialType = iota + 1

	// ICECredentialTypeOauth describes token based credential as described
	// in https://tools.ietf.org/html/rfc7635.
	ICECredentialTypeOauth
)

// This is done this way because of a linter.
const (
	iceCredentialTypePasswordStr = "password"
	iceCredentialTypeOauthStr    = "oauth"
)

func newICECredentialType(raw string) ICECredentialType {
	switch raw {
	case iceCredentialTypePasswordStr:
		return ICECredentialTypePassword
	case iceCredentialTypeOauthStr:
		return ICECredentialTypeOauth
	default:
		return ICECredentialType(Unknown)
	}
}

func (t ICECredentialType) String() string {
	switch t {
	case Unknown:
		return ""
	case ICECredentialTypePassword:
		return iceCredentialTypePasswordStr
	case ICECredentialTypeOauth:
		return iceCredentialTypeOauthStr
	default:
		return ErrUnknownType.Error()
	}
}

// UnmarshalJSON parses the JSON-encoded data and stores the result
func (t *ICECredentialType) UnmarshalJSON(b []byte) error {
	var val string
	var tmp ICECredentialType
	if err := json.Unmarshal(b, &val); err != nil {
		return err
	}

	tmp = newICECredentialType(val)

	if (tmp == ICECredentialType(Unknown)) && (val != "") {
		return fmt.Errorf("%w: (%s)", errInvalidICECredentialTypeString, val)
	}

	*t = tmp
	return nil
}

// MarshalJSON returns the JSON encoding
func (t ICECredentialType) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.String())
}
