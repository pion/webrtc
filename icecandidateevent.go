// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package webrtc

import "sync"

// ICECandidateEvent describes one local ICE gathering event.
//
// Handlers receive accepted events in FIFO order. Replacing a handler or
// setting it to nil affects only future event acceptance; already accepted
// events retain the callback that was active when they were accepted. Closing
// the ICEGatherer or PeerConnection discards accepted events still in the queue.
type ICECandidateEvent struct {
	// Candidate is the gathered candidate, or nil at the end of a gathering.
	Candidate *ICECandidate
	// UsernameFragment identifies the gathering that produced Candidate. For
	// an end-of-gathering event, it identifies the exact common gathering of
	// every current media section, or the common identity of all preceding
	// candidates when no local description is available. It is empty when the
	// browser or ICE agent did not provide one exact identity.
	UsernameFragment string
}

type queuedICECandidateEvent struct {
	event    ICECandidateEvent
	callback func(ICECandidateEvent)
}

// iceCandidateDispatcher serializes accepted events without holding its lock
// across callbacks. Each queued event retains the callback active at acceptance.
type iceCandidateDispatcher struct {
	mtx sync.Mutex

	events  []queuedICECandidateEvent
	running bool
	stopped bool
}

func (d *iceCandidateDispatcher) enqueue(event ICECandidateEvent, callback func(ICECandidateEvent)) {
	if d.accept([]queuedICECandidateEvent{{event: event, callback: callback}}) {
		go d.run()
	}
}

func (d *iceCandidateDispatcher) accept(events []queuedICECandidateEvent) bool {
	d.mtx.Lock()
	defer d.mtx.Unlock()
	if d.stopped {
		return false
	}

	d.events = append(d.events, events...)
	if d.running || len(events) == 0 {
		return false
	}
	d.running = true

	return true
}

func (d *iceCandidateDispatcher) run() {
	for {
		d.mtx.Lock()
		if d.stopped || len(d.events) == 0 {
			d.running = false
			d.mtx.Unlock()
			return
		}

		event := d.events[0]
		d.events[0] = queuedICECandidateEvent{}
		d.events = d.events[1:]
		d.mtx.Unlock()
		if event.callback != nil {
			event.callback(event.event)
		}
	}
}

func (d *iceCandidateDispatcher) stop() {
	d.mtx.Lock()
	d.stopped = true
	d.events = nil
	d.mtx.Unlock()
}
