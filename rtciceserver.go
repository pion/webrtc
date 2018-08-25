package webrtc

import (
	"github.com/pions/webrtc/pkg/ice"
)

// RTCIceServer describes a single STUN and TURN server that can be used by
// the IceAgent to establish a connection with a peer.
type RTCIceServer struct {
	URLs           []string
	Username       string
	Credential     interface{}
	CredentialType RTCIceCredentialType
}

func (s RTCIceServer) parseURL(i int) (*ice.URL, error) {
	return nil, nil
}

func (s RTCIceServer) validate() error {
	for i := range s.URLs {
		url, err := s.parseURL(i)
		if err != nil {
			return err // TODO Need proper error
		}

		if url.Type == ice.ServerTypeTURN {
			// https://www.w3.org/TR/webrtc/#set-the-configuration (step #11.3.2)
			if s.Username == "" || s.Credential == nil {
				return &InvalidAccessError{Err: ErrNoTurnCredencials}
			}

			switch s.CredentialType {
			case RTCIceCredentialTypePassword:
				// https://www.w3.org/TR/webrtc/#set-the-configuration (step #11.3.3)
				if _, ok := s.Credential.(string); !ok {
					return &InvalidAccessError{Err: ErrTurnCredencials}
				}
			case RTCIceCredentialTypeOauth:
				// https://www.w3.org/TR/webrtc/#set-the-configuration (step #11.3.4)
				if _, ok := s.Credential.(RTCOAuthCredential); !ok {
					return &InvalidAccessError{Err: ErrTurnCredencials}
				}

			default:
				return &InvalidAccessError{Err: ErrTurnCredencials}
			}
		}
	}
	return nil
}
