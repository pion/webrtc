// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build js && wasm
// +build js,wasm

package webrtc

// Configuration defines a set of parameters to configure how the
// peer-to-peer communication via PeerConnection is established or
// re-established.
type Configuration struct {
	// ICEServers defines a slice describing servers available to be used by
	// ICE, such as STUN and TURN servers.
	ICEServers []ICEServer `json:"iceServers,omitempty"`

	// ICETransportPolicy indicates which candidates the ICEAgent is allowed
	// to use.
	ICETransportPolicy ICETransportPolicy `json:"iceTransportPolicy,omitempty"`

	// BundlePolicy indicates which media-bundling policy to use when gathering
	// ICE candidates.
	BundlePolicy BundlePolicy `json:"bundlePolicy,omitempty"`

	// RTCPMuxPolicy indicates which rtcp-mux policy to use when gathering ICE
	// candidates.
	RTCPMuxPolicy RTCPMuxPolicy `json:"rtcpMuxPolicy,omitempty"`

	// PeerIdentity sets the target peer identity for the PeerConnection.
	// The PeerConnection will not establish a connection to a remote peer
	// unless it can be successfully authenticated with the provided name.
	PeerIdentity string `json:"peerIdentity,omitempty"`

	// Certificates are not supported in the JavaScript/Wasm bindings.
	// Certificates []Certificate

	// ICECandidatePoolSize describes the size of the prefetched ICE pool.
	ICECandidatePoolSize uint8 `json:"iceCandidatePoolSize,omitempty"`

	// AlwaysNegotiateDataChannels specifies whether the application prefers
	// to always negotiate data channels in the initial SDP offer.
	AlwaysNegotiateDataChannels bool `json:"alwaysNegotiateDataChannels,omitempty"`

	// RTPHeaderEncryptionPolicy affects whether RTP header extension encryption
	// (RFC 9335 Cryptex) is negotiated.
	RTPHeaderEncryptionPolicy RTPHeaderEncryptionPolicy `json:"rtpHeaderEncryptionPolicy,omitempty"`

	Certificates []Certificate `json:"certificates,omitempty"`
}
