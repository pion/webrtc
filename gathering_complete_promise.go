// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package webrtc

import (
	"context"
)

// GatheringCompletePromise is a Pion specific helper function that returns a channel that is closed
// when gathering is complete.
// This function may be helpful in cases where you are unable to trickle your ICE Candidates.
//
// It is better to not use this function, and instead trickle candidates.
// If you use this function you will see longer connection startup times.
// When the call is connected you will see no impact however.
//
// A promise created after CreateOffer(ICERestart) waits for the restart's gathering cycle,
// which only begins on SetLocalDescription. If that offer is never applied, whether abandoned
// or superseded by a later offer, the returned channel stays open, so wait on it with a
// timeout or a context rather than blocking indefinitely.
func GatheringCompletePromise(pc *PeerConnection) (gatherComplete <-chan struct{}) {
	gatheringComplete, done := context.WithCancel(context.Background())

	// It's possible to miss the GatherComplete event since setGatherCompleteHandler is an atomic operation and the
	// promise might have been created after the gathering is finished. Therefore, we need to check if the ICE gathering
	// state has changed to complete so that we don't block the caller forever.
	pc.setGatherCompleteHandler(func() { done() })
	// While a restart is pending, the current Complete belongs to the previous cycle, so let the
	// handler resolve once SetLocalDescription starts the new one instead of resolving now.
	if !pc.hasPendingICERestart() && pc.ICEGatheringState() == ICEGatheringStateComplete {
		done()
	}

	return gatheringComplete.Done()
}
