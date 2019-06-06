package ice

// CredentialType indicates the type of credentials used to connect to
// an ICE server.
type CredentialType int

const (
	// CredentialTypePassword describes username and pasword based
	// credentials as described in https://tools.ietf.org/html/rfc5389.
	CredentialTypePassword CredentialType = iota

	// CredentialTypeOauth describes token based credential as described
	// in https://tools.ietf.org/html/rfc7635.
	CredentialTypeOauth
)

// This is done this way because of a linter.
const (
	credentialTypePasswordStr = "password"
	credentialTypeOauthStr    = "oauth"
)

func newCredentialType(raw string) CredentialType {
	switch raw {
	case credentialTypePasswordStr:
		return CredentialTypePassword
	case credentialTypeOauthStr:
		return CredentialTypeOauth
	default:
		return CredentialType(Unknown)
	}
}

func (t CredentialType) String() string {
	switch t {
	case CredentialTypePassword:
		return credentialTypePasswordStr
	case CredentialTypeOauth:
		return credentialTypeOauthStr
	default:
		return ErrUnknownType.Error()
	}
}
