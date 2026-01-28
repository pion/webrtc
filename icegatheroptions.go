// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package webrtc

// ICEGatherOptions provides options relating to the gathering of ICE candidates.
type ICEGatherOptions struct {
	ICEServers           []ICEServer
	ICEGatherPolicy      ICETransportPolicy
	ICECandidatePoolSize uint8
}
