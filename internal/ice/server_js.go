// +build js,wasm

package ice

import (
	"errors"

	"github.com/pion/ice"
)

// ICEServer describes a single STUN and TURN server that can be used by
// the ICEAgent to establish a connection with a peer.
type ICEServer struct {
	URLs     []string
	Username string
	// Note: Credential and CredentialType are not supported.
	// Credential     interface{}
	// CredentialType ICECredentialType
}

func (s ICEServer) parseURL(i int) (*ice.URL, error) {
	return ice.ParseURL(s.URLs[i])
}

// Validate checks if the ICEServer struct is valid
func (s ICEServer) Validate() error {
	_, err := s.urls()
	return err
}

func (s ICEServer) urls() ([]*ice.URL, error) {
	urls := []*ice.URL{}

	for i := range s.URLs {
		url, err := s.parseURL(i)
		if err != nil {
			return nil, err
		}

		if url.Scheme == ice.SchemeTypeTURN || url.Scheme == ice.SchemeTypeTURNS {
			// // https://www.w3.org/TR/webrtc/#set-the-configuration (step #11.3.2)
			// if s.Username == "" || s.Credential == nil {
			// 	return nil, &rtcerr.InvalidAccessError{Err: ErrNoTurnCredencials}
			// }

			// switch s.CredentialType {
			// case ICECredentialTypePassword:
			// 	// https://www.w3.org/TR/webrtc/#set-the-configuration (step #11.3.3)
			// 	if _, ok := s.Credential.(string); !ok {
			// 		return nil, &rtcerr.InvalidAccessError{Err: ErrTurnCredencials}
			// 	}

			// case ICECredentialTypeOauth:
			// 	// https://www.w3.org/TR/webrtc/#set-the-configuration (step #11.3.4)
			// 	if _, ok := s.Credential.(OAuthCredential); !ok {
			// 		return nil, &rtcerr.InvalidAccessError{Err: ErrTurnCredencials}
			// 	}

			// default:
			// 	return nil, &rtcerr.InvalidAccessError{Err: ErrTurnCredencials}
			// }
			return nil, errors.New("TURN is not currently supported in the JavaScript/Wasm bindings")
		}

		urls = append(urls, url)
	}

	return urls, nil
}
