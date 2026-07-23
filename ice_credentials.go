// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package webrtc

import "github.com/pion/randutil"

// ufrag/pwd length and charset for a restart offer's credentials; any values
// clearing the ICE credential minimums (RFC 8445 section 5.3) are valid.
const (
	iceCredentialUfragLength = 16
	iceCredentialPwdLength   = 32
	iceCredentialRunesAlpha  = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
)

// iceCredentials is a ufrag/pwd pair not tied to any live ICEAgent, used to stage
// an ICE restart's new credentials in an offer before they are applied.
type iceCredentials struct {
	ufrag string
	pwd   string
}

func generateICECredentials() (iceCredentials, error) {
	ufrag, err := randutil.GenerateCryptoRandomString(iceCredentialUfragLength, iceCredentialRunesAlpha)
	if err != nil {
		return iceCredentials{}, err
	}
	pwd, err := randutil.GenerateCryptoRandomString(iceCredentialPwdLength, iceCredentialRunesAlpha)
	if err != nil {
		return iceCredentials{}, err
	}

	return iceCredentials{ufrag: ufrag, pwd: pwd}, nil
}
