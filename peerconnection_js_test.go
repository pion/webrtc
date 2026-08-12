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
		"candidate":     "candidate:1 1 udp 2130706431 192.0.2.1 3478 typ host",
		"sdpMid":        "audio",
		"sdpMLineIndex": 2,
	}))
	candidate := valueToICECandidate(native)
	if assert.NotNil(t, candidate) {
		assert.Equal(t, native.Get("candidate").String(), candidate.ToJSON().Candidate)
		assert.Equal(t, "audio", candidate.SDPMid)
		assert.Equal(t, uint16(2), candidate.SDPMLineIndex)
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
	pc.OnICECandidate(func(candidate *ICECandidate) {
		if candidate == nil {
			events <- "gathering-complete"
			return
		}

		events <- "candidate-start"
		<-releaseCandidate
		events <- "candidate-complete"
	})
	defer func() {
		assert.NoError(t, pc.Close())
	}()

	handler := pc.underlying.Get("onicecandidate")
	handler.Invoke(js.ValueOf(map[string]any{
		"candidate": map[string]any{
			"candidate": "candidate:1 1 udp 2130706431 192.0.2.1 3478 typ host",
		},
	}))
	assert.Equal(t, "candidate-start", <-events)

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
	assert.Equal(t, "gathering-complete", <-events)
}

func TestOnICECandidateReplacementDrainsAcceptedEvents(t *testing.T) {
	closeFunc := js.FuncOf(func(this js.Value, args []js.Value) any {
		return nil
	})
	defer closeFunc.Release()

	pc := &PeerConnection{underlying: js.ValueOf(map[string]any{"close": closeFunc})}
	events := make(chan string, 5)
	firstStarted := make(chan struct{})
	allowReplacement := make(chan struct{})
	replaced := make(chan struct{})
	oldCalls := 0
	pc.OnICECandidate(func(candidate *ICECandidate) {
		oldCalls++
		if oldCalls != 1 {
			events <- "old-candidate-2"
			return
		}

		events <- "old-candidate-1-start"
		close(firstStarted)
		<-allowReplacement
		pc.OnICECandidate(func(candidate *ICECandidate) {
			if candidate == nil {
				events <- "new-gathering-complete"
				return
			}
			events <- "new-candidate"
		})
		close(replaced)
		events <- "old-candidate-1-complete"
	})
	defer func() {
		assert.NoError(t, pc.Close())
	}()

	candidateEvent := js.ValueOf(map[string]any{
		"candidate": map[string]any{
			"candidate": "candidate:1 1 udp 2130706431 192.0.2.1 3478 typ host",
		},
	})
	oldHandler := pc.underlying.Get("onicecandidate")
	oldHandler.Invoke(candidateEvent)
	<-firstStarted
	assert.Equal(t, "old-candidate-1-start", <-events)

	oldHandler.Invoke(candidateEvent)
	close(allowReplacement)
	<-replaced
	newHandler := pc.underlying.Get("onicecandidate")
	newHandler.Invoke(candidateEvent)
	newHandler.Invoke(js.ValueOf(map[string]any{"candidate": nil}))

	assert.Equal(t, "old-candidate-1-complete", <-events)
	assert.Equal(t, "old-candidate-2", <-events)
	assert.Equal(t, "new-candidate", <-events)
	assert.Equal(t, "new-gathering-complete", <-events)
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
