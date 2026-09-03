// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package webrtc

import (
	"encoding/json"
	"syscall/js"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestValueToICECandidate(t *testing.T) {
	testCases := []struct {
		jsonCandidate string
		expect        ICECandidate
	}{
		{
			// Firefox-style ICECandidateInit:
			`{"candidate":"1966762133 1 udp 2122260222 192.168.20.128 47298 typ srflx raddr 203.0.113.1 rport 5000"}`,
			ICECandidate{
				Foundation:     "1966762133",
				Priority:       2122260222,
				Address:        "192.168.20.128",
				Protocol:       ICEProtocolUDP,
				Port:           47298,
				Typ:            ICECandidateTypeSrflx,
				Component:      1,
				RelatedAddress: "203.0.113.1",
				RelatedPort:    5000,
			},
		}, {
			// Chrome/Webkit-style ICECandidate:
			`{"foundation":"1966762134", "component":"rtp", "protocol":"udp", "priority":2122260223, "address":"192.168.20.129", "port":47299, "type":"host", "relatedAddress":null}`,
			ICECandidate{
				Foundation:     "1966762134",
				Priority:       2122260223,
				Address:        "192.168.20.129",
				Protocol:       ICEProtocolUDP,
				Port:           47299,
				Typ:            ICECandidateTypeHost,
				Component:      1,
				RelatedAddress: "<null>",
				RelatedPort:    0,
			},
		}, {
			// Both are present; the SDP candidate string is the browser's
			// authoritative candidate payload.
			`{"candidate":"1966762133 1 udp 2122260222 192.168.20.128 47298 typ srflx raddr 203.0.113.1 rport 5000", "foundation":"1966762134", "component":"rtp", "protocol":"udp", "priority":2122260223, "address":"192.168.20.129", "port":47299, "type":"host", "relatedAddress":null, "sdpMid":"0", "sdpMLineIndex":2}`,
			ICECandidate{
				Foundation:     "1966762133",
				Priority:       2122260222,
				Address:        "192.168.20.128",
				Protocol:       ICEProtocolUDP,
				Port:           47298,
				Typ:            ICECandidateTypeSrflx,
				Component:      1,
				RelatedAddress: "203.0.113.1",
				RelatedPort:    5000,
				SDPMid:         "0",
				SDPMLineIndex:  2,
			},
		},
	}

	for i, testCase := range testCases {
		v := map[string]any{}
		err := json.Unmarshal([]byte(testCase.jsonCandidate), &v)
		if err != nil {
			t.Errorf("Case %d: bad test, got error: %v", i, err)
		}
		val := *valueToICECandidate(js.ValueOf(v))
		val.statsID = ""
		assert.Equal(t, testCase.expect, val)
	}
}

func TestValueToICECandidateFromNativeBrowserCandidate(t *testing.T) {
	constructor := js.Global().Get("RTCIceCandidate")
	if constructor.IsUndefined() {
		t.Skip("RTCIceCandidate constructor is not available")
	}

	native := constructor.New(js.ValueOf(map[string]any{
		"candidate":        "candidate:1 1 udp 2130706431 192.0.2.1 3478 typ host",
		"sdpMid":           "audio",
		"sdpMLineIndex":    2,
		"usernameFragment": "browser-ufrag",
	}))
	candidate := valueToICECandidate(native)
	if assert.NotNil(t, candidate) {
		assert.Equal(t, native.Get("candidate").String(), candidate.ToJSON().Candidate)
		assert.Equal(t, "audio", candidate.SDPMid)
		assert.Equal(t, uint16(2), candidate.SDPMLineIndex)
		assert.Equal(t, "browser-ufrag", candidate.UsernameFragment())
		assert.Equal(t, "browser-ufrag", *candidate.ToJSON().UsernameFragment)
	}
}

func TestOnICECandidatePreservesBrowserEventOrder(t *testing.T) {
	closeFunc := js.FuncOf(func(this js.Value, args []js.Value) any {
		return nil
	})
	defer closeFunc.Release()

	pc := &PeerConnection{underlying: js.ValueOf(map[string]any{"close": closeFunc})}
	events := make(chan string, 3)
	releaseCandidate := make(chan struct{})
	pc.OnICECandidateEvent(func(event ICECandidateEvent) {
		if event.Candidate == nil {
			events <- "gathering-complete:" + event.UsernameFragment
			return
		}

		events <- "candidate-start:" + event.UsernameFragment
		<-releaseCandidate
		events <- "candidate-complete"
	})
	defer func() {
		assert.NoError(t, pc.Close())
	}()

	handler := pc.underlying.Get("onicecandidate")
	handler.Invoke(js.ValueOf(map[string]any{
		"candidate": map[string]any{
			"candidate":        "candidate:1 1 udp 2130706431 192.0.2.1 3478 typ host",
			"usernameFragment": "gathering-1",
		},
	}))
	assert.Equal(t, "candidate-start:gathering-1", <-events)

	// The browser delivers the end marker after its candidates. The adapter
	// must not begin that callback while the preceding candidate is in flight.
	handler.Invoke(js.ValueOf(map[string]any{"candidate": nil}))
	select {
	case event := <-events:
		t.Fatalf("callback order violated: got %q before candidate completion", event)
	case <-time.After(50 * time.Millisecond):
	}

	close(releaseCandidate)
	assert.Equal(t, "candidate-complete", <-events)
	assert.Equal(t, "gathering-complete:gathering-1", <-events)
}

func TestOnICECandidateReplacementDrainsAcceptedEvents(t *testing.T) {
	closeFunc := js.FuncOf(func(this js.Value, args []js.Value) any { return nil })
	defer closeFunc.Release()

	pc := &PeerConnection{underlying: js.ValueOf(map[string]any{"close": closeFunc})}
	events := make(chan string, 6)
	firstStarted := make(chan struct{})
	releaseFirst := make(chan struct{})
	oldCalls := 0
	pc.OnICECandidateEvent(func(event ICECandidateEvent) {
		oldCalls++
		if oldCalls == 1 {
			events <- "old-start:" + event.UsernameFragment
			close(firstStarted)
			<-releaseFirst
			events <- "old-complete:" + event.UsernameFragment
			return
		}
		if event.Candidate == nil {
			events <- "old-eoc:" + event.UsernameFragment
			return
		}
		events <- "old-late:" + event.UsernameFragment
	})
	defer func() { assert.NoError(t, pc.Close()) }()

	oldCandidate := js.ValueOf(map[string]any{"candidate": map[string]any{
		"candidate": "candidate:1 1 udp 2130706431 192.0.2.1 3478 typ host", "usernameFragment": "old",
	}})
	oldHandler := pc.underlying.Get("onicecandidate")
	oldHandler.Invoke(oldCandidate)
	<-firstStarted
	oldHandler.Invoke(oldCandidate)
	oldHandler.Invoke(js.ValueOf(map[string]any{"candidate": nil}))

	pc.OnICECandidateEvent(func(event ICECandidateEvent) {
		if event.Candidate == nil {
			events <- "new-eoc:" + event.UsernameFragment
			return
		}
		events <- "new:" + event.UsernameFragment
	})
	newHandler := pc.underlying.Get("onicecandidate")
	newHandler.Invoke(js.ValueOf(map[string]any{"candidate": map[string]any{
		"candidate": "candidate:2 1 udp 2130706431 192.0.2.2 3478 typ host", "usernameFragment": "new",
	}}))
	newHandler.Invoke(js.ValueOf(map[string]any{"candidate": nil}))
	close(releaseFirst)

	assert.Equal(t, "old-start:old", <-events)
	assert.Equal(t, "old-complete:old", <-events)
	assert.Equal(t, "old-late:old", <-events)
	assert.Equal(t, "old-eoc:old", <-events)
	assert.Equal(t, "new:new", <-events)
	assert.Equal(t, "new-eoc:new", <-events)
}

func TestCloseDropsQueuedICECandidateCallbacks(t *testing.T) {
	closeFunc := js.FuncOf(func(this js.Value, args []js.Value) any {
		return nil
	})
	defer closeFunc.Release()

	pc := &PeerConnection{underlying: js.ValueOf(map[string]any{"close": closeFunc})}
	firstStarted := make(chan struct{})
	releaseFirst := make(chan struct{})
	unexpected := make(chan struct{}, 1)
	calls := 0
	pc.OnICECandidate(func(candidate *ICECandidate) {
		calls++
		if calls == 1 {
			close(firstStarted)
			<-releaseFirst
			return
		}
		unexpected <- struct{}{}
	})

	handler := pc.underlying.Get("onicecandidate")
	event := js.ValueOf(map[string]any{
		"candidate": map[string]any{
			"candidate": "candidate:1 1 udp 2130706431 192.0.2.1 3478 typ host",
		},
	})
	handler.Invoke(event)
	<-firstStarted
	handler.Invoke(event)
	assert.NoError(t, pc.Close())
	close(releaseFirst)

	select {
	case <-unexpected:
		t.Fatal("queued candidate callback ran after Close")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestOnICECandidateZeroCandidateRestartUsesLocalDescriptionGeneration(t *testing.T) {
	closeFunc := js.FuncOf(func(this js.Value, args []js.Value) any { return nil })
	defer closeFunc.Release()

	underlying := js.ValueOf(map[string]any{
		"close": closeFunc,
		"localDescription": map[string]any{
			"sdp": "v=0\r\na=ice-ufrag:first-generation\r\n",
		},
	})
	pc := &PeerConnection{underlying: underlying}
	defer func() { assert.NoError(t, pc.Close()) }()

	events := make(chan string, 2)
	pc.OnICECandidateEvent(func(event ICECandidateEvent) { events <- event.UsernameFragment })
	pc.underlying.Get("onicecandidate").Invoke(js.ValueOf(map[string]any{"candidate": nil}))
	assert.Equal(t, "first-generation", <-events)

	underlying.Set("localDescription", map[string]any{
		"sdp": "v=0\r\na=ice-ufrag:second-generation\r\n",
	})
	pc.OnICECandidateEvent(func(event ICECandidateEvent) { events <- event.UsernameFragment })
	pc.underlying.Get("onicecandidate").Invoke(js.ValueOf(map[string]any{"candidate": nil}))
	assert.Equal(t, "second-generation", <-events)
}

func TestOnICECandidateEmptyIdentityDoesNotInheritPriorGeneration(t *testing.T) {
	closeFunc := js.FuncOf(func(this js.Value, args []js.Value) any { return nil })
	defer closeFunc.Release()

	underlying := js.ValueOf(map[string]any{
		"close": closeFunc,
		"localDescription": map[string]any{
			"sdp": "v=0\r\na=ice-ufrag:known-generation\r\n",
		},
	})
	pc := &PeerConnection{underlying: underlying}
	defer func() { assert.NoError(t, pc.Close()) }()

	events := make(chan ICECandidateEvent, 4)
	pc.OnICECandidateEvent(func(event ICECandidateEvent) { events <- event })
	handler := pc.underlying.Get("onicecandidate")
	handler.Invoke(js.ValueOf(map[string]any{"candidate": map[string]any{
		"candidate":        "candidate:1 1 udp 2130706431 192.0.2.1 3478 typ host",
		"usernameFragment": "known-generation",
	}}))
	handler.Invoke(js.ValueOf(map[string]any{"candidate": nil}))
	assert.Equal(t, "known-generation", (<-events).UsernameFragment)
	assert.Equal(t, "known-generation", (<-events).UsernameFragment)

	underlying.Set("localDescription", js.Null())
	handler.Invoke(js.ValueOf(map[string]any{"candidate": map[string]any{
		"candidate":        "candidate:2 1 udp 2130706431 192.0.2.2 3478 typ host",
		"usernameFragment": "",
	}}))
	handler.Invoke(js.ValueOf(map[string]any{"candidate": nil}))

	candidateEvent := <-events
	if assert.NotNil(t, candidateEvent.Candidate) {
		assert.Empty(t, candidateEvent.Candidate.UsernameFragment())
	}
	assert.Empty(t, candidateEvent.UsernameFragment)
	endEvent := <-events
	assert.Nil(t, endEvent.Candidate)
	assert.Empty(t, endEvent.UsernameFragment)
}

func TestOnICECandidateUsesSelectedMediaUsernameFragment(t *testing.T) {
	closeFunc := js.FuncOf(func(this js.Value, args []js.Value) any { return nil })
	defer closeFunc.Release()

	pc := &PeerConnection{underlying: js.ValueOf(map[string]any{
		"close": closeFunc,
		"localDescription": map[string]any{"sdp": "v=0\r\n" +
			"a=ice-ufrag:session-generation\r\n" +
			"m=audio 9 UDP/TLS/RTP/SAVPF 111\r\n" +
			"a=mid:audio\r\n" +
			"a=ice-ufrag:audio-generation\r\n" +
			"m=video 9 UDP/TLS/RTP/SAVPF 96\r\n" +
			"a=mid:video\r\n" +
			"a=ice-ufrag:video-generation\r\n"},
	})}
	defer func() { assert.NoError(t, pc.Close()) }()

	events := make(chan ICECandidateEvent, 2)
	pc.OnICECandidateEvent(func(event ICECandidateEvent) { events <- event })
	handler := pc.underlying.Get("onicecandidate")
	handler.Invoke(js.ValueOf(map[string]any{"candidate": map[string]any{
		"candidate":     "candidate:1 1 udp 2130706431 192.0.2.1 3478 typ host",
		"sdpMid":        "video",
		"sdpMLineIndex": 1,
	}}))
	handler.Invoke(js.ValueOf(map[string]any{"candidate": nil}))

	candidateEvent := <-events
	if assert.NotNil(t, candidateEvent.Candidate) {
		assert.Equal(t, "video-generation", candidateEvent.Candidate.UsernameFragment())
		assert.Equal(t, "video-generation", *candidateEvent.Candidate.ToJSON().UsernameFragment)
	}
	assert.Equal(t, "video-generation", candidateEvent.UsernameFragment)

	endEvent := <-events
	assert.Nil(t, endEvent.Candidate)
	assert.Empty(t, endEvent.UsernameFragment, "global end marker cannot select one of multiple ICE transports")
}

func TestLocalICEUsernameFragmentSelection(t *testing.T) {
	const distinctMedia = "v=0\r\n" +
		"a=ice-ufrag:session-generation\r\n" +
		"m=audio 9 UDP/TLS/RTP/SAVPF 111\r\n" +
		"a=mid:audio\r\n" +
		"a=ice-ufrag:audio-generation\r\n" +
		"m=video 9 UDP/TLS/RTP/SAVPF 96\r\n" +
		"a=mid:video\r\n" +
		"a=ice-ufrag:video-generation\r\n"

	testCases := []struct {
		name      string
		sdp       string
		candidate map[string]any
		expect    string
	}{
		{
			name: "session credential inherited",
			sdp: "v=0\r\na=ice-ufrag:session-generation\r\n" +
				"m=audio 9 UDP/TLS/RTP/SAVPF 111\r\na=mid:audio\r\n",
			candidate: map[string]any{"sdpMid": "audio", "sdpMLineIndex": 0},
			expect:    "session-generation",
		},
		{
			name:      "media index selects override",
			sdp:       distinctMedia,
			candidate: map[string]any{"sdpMLineIndex": 1},
			expect:    "video-generation",
		},
		{
			name:      "selectors absent in multi transport description",
			sdp:       distinctMedia,
			candidate: map[string]any{},
		},
		{
			name: "candidate mid absent from description",
			sdp: "v=0\r\nm=audio 9 UDP/TLS/RTP/SAVPF 111\r\n" +
				"a=ice-ufrag:audio-generation\r\n",
			candidate: map[string]any{"sdpMid": "audio", "sdpMLineIndex": 0},
		},
		{
			name: "malformed mid is rejected",
			sdp: "v=0\r\nm=audio 9 UDP/TLS/RTP/SAVPF 111\r\n" +
				"a=mid:bad mid\r\na=ice-ufrag:audio-generation\r\n",
			candidate: map[string]any{"sdpMid": "bad mid", "sdpMLineIndex": 0},
		},
		{
			name: "duplicate mid is ambiguous",
			sdp: "v=0\r\nm=audio 9 UDP/TLS/RTP/SAVPF 111\r\n" +
				"a=mid:duplicate\r\na=ice-ufrag:first\r\n" +
				"m=video 9 UDP/TLS/RTP/SAVPF 96\r\n" +
				"a=mid:duplicate\r\na=ice-ufrag:second\r\n",
			candidate: map[string]any{"sdpMid": "duplicate", "sdpMLineIndex": 1},
		},
		{
			name:      "mid and index disagree",
			sdp:       distinctMedia,
			candidate: map[string]any{"sdpMid": "audio", "sdpMLineIndex": 1},
		},
		{
			name:      "malformed SDP",
			sdp:       "v=0\r\nnot-an-sdp-line\r\na=ice-ufrag:first\r\n",
			candidate: map[string]any{},
		},
		{
			name: "duplicate media credential",
			sdp: "v=0\r\nm=audio 9 UDP/TLS/RTP/SAVPF 111\r\na=mid:audio\r\n" +
				"a=ice-ufrag:first\r\na=ice-ufrag:second\r\n",
			candidate: map[string]any{"sdpMid": "audio", "sdpMLineIndex": 0},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			pc := &PeerConnection{underlying: js.ValueOf(map[string]any{
				"localDescription": map[string]any{"sdp": testCase.sdp},
			})}
			assert.Equal(t, testCase.expect, pc.localICEUsernameFragment(js.ValueOf(testCase.candidate)))
		})
	}
}

func TestOnICECandidateReentrantReplacementRetainsAcceptedCallback(t *testing.T) {
	closeFunc := js.FuncOf(func(this js.Value, args []js.Value) any { return nil })
	defer closeFunc.Release()

	pc := &PeerConnection{underlying: js.ValueOf(map[string]any{"close": closeFunc})}
	defer func() { assert.NoError(t, pc.Close()) }()

	events := make(chan string, 4)
	calls := 0
	pc.OnICECandidateEvent(func(event ICECandidateEvent) {
		calls++
		events <- "old"
		if calls == 1 {
			pc.OnICECandidateEvent(func(ICECandidateEvent) { events <- "new" })
		}
	})
	candidate := js.ValueOf(map[string]any{"candidate": map[string]any{
		"candidate": "candidate:1 1 udp 2130706431 192.0.2.1 3478 typ host",
	}})
	pc.underlying.Get("onicecandidate").Invoke(candidate)
	assert.Equal(t, "old", <-events)

	pc.underlying.Get("onicecandidate").Invoke(candidate)
	assert.Equal(t, "new", <-events)
}

func TestOnICECandidateReentrantCloseDropsQueuedCallbacks(t *testing.T) {
	closeFunc := js.FuncOf(func(this js.Value, args []js.Value) any { return nil })
	defer closeFunc.Release()

	dispatcher := &iceCandidateDispatcher{running: true}
	pc := &PeerConnection{
		underlying:             js.ValueOf(map[string]any{"close": closeFunc}),
		iceCandidateDispatcher: dispatcher,
	}
	closed := make(chan struct{})
	unexpected := make(chan struct{}, 1)
	dispatcher.events = []queuedICECandidateEvent{
		{callback: func(ICECandidateEvent) {
			assert.NoError(t, pc.Close())
			close(closed)
		}},
		{callback: func(ICECandidateEvent) { unexpected <- struct{}{} }},
	}
	go dispatcher.run()
	<-closed
	select {
	case <-unexpected:
		t.Fatal("queued callback ran after reentrant Close")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestOnICECandidateNilHandlers(t *testing.T) {
	closeFunc := js.FuncOf(func(this js.Value, args []js.Value) any { return nil })
	defer closeFunc.Release()

	pc := &PeerConnection{underlying: js.ValueOf(map[string]any{"close": closeFunc})}
	defer func() { assert.NoError(t, pc.Close()) }()
	pc.OnICECandidate(nil)
	assert.Equal(t, js.TypeUndefined, pc.underlying.Get("onicecandidate").Type())
	pc.OnICECandidateEvent(nil)
	assert.Equal(t, js.TypeUndefined, pc.underlying.Get("onicecandidate").Type())
}

func TestValueToICEServer(t *testing.T) {
	testCases := []ICEServer{
		{
			URLs:           []string{"turn:192.158.29.39?transport=udp"},
			Username:       "unittest",
			Credential:     "placeholder",
			CredentialType: ICECredentialTypePassword,
		},
		{
			URLs:           []string{"turn:[2001:db8:1234:5678::1]?transport=udp"},
			Username:       "unittest",
			Credential:     "placeholder",
			CredentialType: ICECredentialTypePassword,
		},
		{
			URLs:     []string{"turn:192.158.29.39?transport=udp"},
			Username: "unittest",
			Credential: OAuthCredential{
				MACKey:      "WmtzanB3ZW9peFhtdm42NzUzNG0=",
				AccessToken: "AAwg3kPHWPfvk9bDFL936wYvkoctMADzQ5VhNDgeMR3+ZlZ35byg972fW8QjpEl7bx91YLBPFsIhsxloWcXPhA==",
			},
			CredentialType: ICECredentialTypeOauth,
		},
	}

	for _, testCase := range testCases {
		v := iceServerToValue(testCase)
		s := valueToICEServer(v)
		assert.Equal(t, testCase, s)
	}
}

func TestPeerConnectionCanTrickleICECandidatesJS(t *testing.T) {
	pc := &PeerConnection{
		underlying: js.ValueOf(map[string]any{
			"canTrickleIceCandidates": true,
		}),
	}
	assert.Equal(t, ICETrickleCapabilitySupported, pc.CanTrickleICECandidates())

	pc.underlying = js.ValueOf(map[string]any{
		"canTrickleIceCandidates": false,
	})
	assert.Equal(t, ICETrickleCapabilityUnsupported, pc.CanTrickleICECandidates())

	pc.underlying = js.ValueOf(map[string]any{})
	assert.Equal(t, ICETrickleCapabilityUnknown, pc.CanTrickleICECandidates())
}

func TestDTLSTransportGetRemoteCertificateMock(t *testing.T) {
	expected := []byte{0x01, 0x02, 0x03, 0x04}

	u8 := js.Global().Get("Uint8Array").New(len(expected))
	if n := js.CopyBytesToJS(u8, expected); n != len(expected) {
		t.Fatalf("copied %d bytes to Uint8Array; expected %d", n, len(expected))
	}
	certBuffer := u8.Get("buffer")

	getRemoteCertificates := js.FuncOf(func(this js.Value, args []js.Value) any {
		return js.ValueOf([]any{certBuffer})
	})
	defer getRemoteCertificates.Release()

	mockTransport := js.Global().Get("Object").New()
	mockTransport.Set("getRemoteCertificates", getRemoteCertificates)

	dtlsTransport := &DTLSTransport{underlying: mockTransport}
	assert.Equal(t, expected, dtlsTransport.GetRemoteCertificate())
}
