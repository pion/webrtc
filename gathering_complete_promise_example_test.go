// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package webrtc

import (
	"fmt"
	"strings"
)

// ExampleGatheringCompletePromise demonstrates how to implement
// non-trickle ICE in Pion, an older form of ICE that does not require an
// asynchronous side channel between peers: negotiation is just a single
// offer-answer exchange.  It works by explicitly waiting for all local
// ICE candidates to have been gathered before sending an offer to the peer.
func ExampleGatheringCompletePromise() {
	// create a peer connection
	pc, err := NewPeerConnection(Configuration{})
	if err != nil {
		panic(err)
	}
	defer func() {
		closeErr := pc.Close()
		if closeErr != nil {
			panic(closeErr)
		}
	}()

	// add at least one transceiver to the peer connection, or nothing
	// interesting will happen.  This could use pc.AddTrack instead.
	_, err = pc.AddTransceiverFromKind(RTPCodecTypeVideo)
	if err != nil {
		panic(err)
	}

	// create a first offer that does not contain any local candidates
	offer, err := pc.CreateOffer(nil)
	if err != nil {
		panic(err)
	}

	// gatherComplete is a channel that will be closed when
	// the gathering of local candidates is complete.
	gatherComplete := GatheringCompletePromise(pc)

	// apply the offer
	err = pc.SetLocalDescription(offer)
	if err != nil {
		panic(err)
	}

	// wait for gathering of local candidates to complete
	<-gatherComplete

	// compute the local offer again
	offer2 := pc.LocalDescription()

	// this second offer contains all candidates, and may be sent to
	// the peer with no need for further communication.  In this
	// example, we simply check that it contains at least one
	// candidate.
	hasCandidate := strings.Contains(offer2.SDP, "\na=candidate:")
	if hasCandidate {
		fmt.Println("Ok!")
	}
	// Output: Ok!
}
