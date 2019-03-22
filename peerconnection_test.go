package webrtc

import (
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/pions/webrtc/pkg/rtcerr"
	"github.com/stretchr/testify/assert"
)

// newPair creates two new peer connections (an offerer and an answerer)
// *without* using an api (i.e. using the default settings).
func newPair() (pcOffer *PeerConnection, pcAnswer *PeerConnection, err error) {
	pca, err := NewPeerConnection(Configuration{})
	if err != nil {
		return nil, nil, err
	}

	pcb, err := NewPeerConnection(Configuration{})
	if err != nil {
		return nil, nil, err
	}

	return pca, pcb, nil
}

func signalPair(pcOffer *PeerConnection, pcAnswer *PeerConnection) error {
	offerChan := make(chan SessionDescription)
	pcOffer.OnICECandidate(func(candidate *ICECandidate) {
		if candidate == nil {
			offerChan <- *pcOffer.PendingLocalDescription()
		}
	})

	// Note(albrow): We need to create a data channel in order to trigger ICE
	// candidate gathering in the background for the JavaScript/Wasm bindings. If
	// we don't do this, the complete offer including ICE candidates will never be
	// generated.
	if _, err := pcOffer.CreateDataChannel("initial_data_channel", nil); err != nil {
		return err
	}

	offer, err := pcOffer.CreateOffer(nil)
	if err != nil {
		return err
	}
	if err := pcOffer.SetLocalDescription(offer); err != nil {
		return err
	}

	timeout := time.After(3 * time.Second)
	select {
	case <-timeout:
		return fmt.Errorf("timed out waiting to receive offer")
	case offer := <-offerChan:
		if err := pcAnswer.SetRemoteDescription(offer); err != nil {
			return err
		}

		answer, err := pcAnswer.CreateAnswer(nil)
		if err != nil {
			return err
		}

		if err = pcAnswer.SetLocalDescription(answer); err != nil {
			return err
		}

		err = pcOffer.SetRemoteDescription(answer)
		if err != nil {
			return err
		}
		return nil
	}
}

func TestNew(t *testing.T) {
	pc, err := NewPeerConnection(Configuration{
		ICEServers: []ICEServer{
			{
				URLs: []string{
					"stun:stun.l.google.com:19302",
				},
				Username: "unittest",
			},
		},
		ICETransportPolicy:   ICETransportPolicyRelay,
		BundlePolicy:         BundlePolicyMaxCompat,
		RTCPMuxPolicy:        RTCPMuxPolicyNegotiate,
		PeerIdentity:         "unittest",
		ICECandidatePoolSize: 5,
	})
	assert.NoError(t, err)
	assert.NotNil(t, pc)
}

func TestPeerConnection_SetConfiguration(t *testing.T) {
	// Note: These tests don't include ICEServer.Credential,
	// ICEServer.CredentialType, or Certificates because those are not supported
	// in the WASM bindings.

	for _, test := range []struct {
		name    string
		init    func() (*PeerConnection, error)
		config  Configuration
		wantErr error
	}{
		{
			name: "valid",
			init: func() (*PeerConnection, error) {
				pc, err := NewPeerConnection(Configuration{
					ICECandidatePoolSize: 5,
				})
				if err != nil {
					return pc, err
				}

				err = pc.SetConfiguration(Configuration{
					ICEServers: []ICEServer{
						{
							URLs: []string{
								"stun:stun.l.google.com:19302",
							},
							Username: "unittest",
						},
					},
					ICETransportPolicy:   ICETransportPolicyAll,
					BundlePolicy:         BundlePolicyBalanced,
					RTCPMuxPolicy:        RTCPMuxPolicyRequire,
					ICECandidatePoolSize: 5,
				})
				if err != nil {
					return pc, err
				}

				return pc, nil
			},
			config:  Configuration{},
			wantErr: nil,
		},
		{
			name: "closed connection",
			init: func() (*PeerConnection, error) {
				pc, err := NewPeerConnection(Configuration{})
				assert.Nil(t, err)

				err = pc.Close()
				assert.Nil(t, err)
				return pc, err
			},
			config:  Configuration{},
			wantErr: &rtcerr.InvalidStateError{Err: ErrConnectionClosed},
		},
		{
			name: "update PeerIdentity",
			init: func() (*PeerConnection, error) {
				return NewPeerConnection(Configuration{})
			},
			config: Configuration{
				PeerIdentity: "unittest",
			},
			wantErr: &rtcerr.InvalidModificationError{Err: ErrModifyingPeerIdentity},
		},
		{
			name: "update BundlePolicy",
			init: func() (*PeerConnection, error) {
				return NewPeerConnection(Configuration{})
			},
			config: Configuration{
				BundlePolicy: BundlePolicyMaxCompat,
			},
			wantErr: &rtcerr.InvalidModificationError{Err: ErrModifyingBundlePolicy},
		},
		{
			name: "update RTCPMuxPolicy",
			init: func() (*PeerConnection, error) {
				return NewPeerConnection(Configuration{})
			},
			config: Configuration{
				RTCPMuxPolicy: RTCPMuxPolicyNegotiate,
			},
			wantErr: &rtcerr.InvalidModificationError{Err: ErrModifyingRTCPMuxPolicy},
		},
		{
			name: "update ICECandidatePoolSize",
			init: func() (*PeerConnection, error) {
				pc, err := NewPeerConnection(Configuration{
					ICECandidatePoolSize: 0,
				})
				if err != nil {
					return pc, err
				}
				offer, err := pc.CreateOffer(nil)
				if err != nil {
					return pc, err
				}
				err = pc.SetLocalDescription(offer)
				if err != nil {
					return pc, err
				}
				return pc, nil
			},
			config: Configuration{
				ICECandidatePoolSize: 1,
			},
			wantErr: &rtcerr.InvalidModificationError{Err: ErrModifyingICECandidatePoolSize},
		},
	} {
		pc, err := test.init()
		if err != nil {
			t.Errorf("SetConfiguration %q: init failed: %v", test.name, err)
		}

		err = pc.SetConfiguration(test.config)
		if got, want := err, test.wantErr; !reflect.DeepEqual(got, want) {
			t.Errorf("SetConfiguration %q: err = %v, want %v", test.name, got, want)
		}
	}
}

func TestPeerConnection_GetConfiguration(t *testing.T) {
	pc, err := NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	expected := Configuration{
		ICEServers:           []ICEServer{},
		ICETransportPolicy:   ICETransportPolicyAll,
		BundlePolicy:         BundlePolicyBalanced,
		RTCPMuxPolicy:        RTCPMuxPolicyRequire,
		ICECandidatePoolSize: 0,
	}
	actual := pc.GetConfiguration()
	assert.True(t, &expected != &actual)
	assert.Equal(t, expected.ICEServers, actual.ICEServers)
	assert.Equal(t, expected.ICETransportPolicy, actual.ICETransportPolicy)
	assert.Equal(t, expected.BundlePolicy, actual.BundlePolicy)
	assert.Equal(t, expected.RTCPMuxPolicy, actual.RTCPMuxPolicy)
	// TODO(albrow): Uncomment this after #513 is fixed.
	// See: https://github.com/pions/webrtc/issues/513.
	// assert.Equal(t, len(expected.Certificates), len(actual.Certificates))
	assert.Equal(t, expected.ICECandidatePoolSize, actual.ICECandidatePoolSize)
}

const minimalOffer = `v=0
o=- 4596489990601351948 2 IN IP4 127.0.0.1
s=-
t=0 0
a=msid-semantic: WMS
m=application 47299 DTLS/SCTP 5000
c=IN IP4 192.168.20.129
a=candidate:1966762134 1 udp 2122260223 192.168.20.129 47299 typ host generation 0
a=candidate:211962667 1 udp 2122194687 10.0.3.1 40864 typ host generation 0
a=candidate:1002017894 1 tcp 1518280447 192.168.20.129 0 typ host tcptype active generation 0
a=candidate:1109506011 1 tcp 1518214911 10.0.3.1 0 typ host tcptype active generation 0
a=ice-ufrag:1/MvHwjAyVf27aLu
a=ice-pwd:3dBU7cFOBl120v33cynDvN1E
a=ice-options:google-ice
a=fingerprint:sha-256 75:74:5A:A6:A4:E5:52:F4:A7:67:4C:01:C7:EE:91:3F:21:3D:A2:E3:53:7B:6F:30:86:F2:30:AA:65:FB:04:24
a=setup:actpass
a=mid:data
a=sctpmap:5000 webrtc-datachannel 1024
`

func TestSetRemoteDescription(t *testing.T) {
	testCases := []struct {
		desc SessionDescription
	}{
		{SessionDescription{Type: SDPTypeOffer, SDP: minimalOffer}},
	}

	for i, testCase := range testCases {
		peerConn, err := NewPeerConnection(Configuration{})
		if err != nil {
			t.Errorf("Case %d: got error: %v", i, err)
		}
		err = peerConn.SetRemoteDescription(testCase.desc)
		if err != nil {
			t.Errorf("Case %d: got error: %v", i, err)
		}
	}
}

func TestCreateOfferAnswer(t *testing.T) {
	offerPeerConn, err := NewPeerConnection(Configuration{})
	if err != nil {
		t.Errorf("New PeerConnection: got error: %v", err)
	}
	offer, err := offerPeerConn.CreateOffer(nil)
	if err != nil {
		t.Errorf("Create Offer: got error: %v", err)
	}
	if err = offerPeerConn.SetLocalDescription(offer); err != nil {
		t.Errorf("SetLocalDescription: got error: %v", err)
	}
	answerPeerConn, err := NewPeerConnection(Configuration{})
	if err != nil {
		t.Errorf("New PeerConnection: got error: %v", err)
	}
	err = answerPeerConn.SetRemoteDescription(offer)
	if err != nil {
		t.Errorf("SetRemoteDescription: got error: %v", err)
	}
	answer, err := answerPeerConn.CreateAnswer(nil)
	if err != nil {
		t.Errorf("Create Answer: got error: %v", err)
	}
	if err = answerPeerConn.SetLocalDescription(answer); err != nil {
		t.Errorf("SetLocalDescription: got error: %v", err)
	}
	err = offerPeerConn.SetRemoteDescription(answer)
	if err != nil {
		t.Errorf("SetRemoteDescription (Originator): got error: %v", err)
	}
}

func TestPeerConnection_EventHandlers(t *testing.T) {
	pcOffer, err := NewPeerConnection(Configuration{})
	assert.NoError(t, err)
	pcAnswer, err := NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	// wasCalled is a list of event handlers that were called.
	wasCalled := []string{}
	wasCalledMut := &sync.Mutex{}
	// wg is used to wait for all event handlers to be called.
	wg := &sync.WaitGroup{}
	wg.Add(4)

	// Each sync.Once is used to ensure that we call wg.Done once for each event
	// handler and don't add multiple entries to wasCalled. The event handlers can
	// be called more than once in some cases.
	onceOffererOnICEConnectionStateChange := &sync.Once{}
	onceOffererOnSignalingStateChange := &sync.Once{}
	onceAnswererOnICEConnectionStateChange := &sync.Once{}
	onceAnswererOnSignalingStateChange := &sync.Once{}

	// Register all the event handlers.
	pcOffer.OnICEConnectionStateChange(func(ICEConnectionState) {
		onceOffererOnICEConnectionStateChange.Do(func() {
			wasCalledMut.Lock()
			defer wasCalledMut.Unlock()
			wasCalled = append(wasCalled, "offerer OnICEConnectionStateChange")
			wg.Done()
		})
	})
	pcOffer.OnSignalingStateChange(func(SignalingState) {
		onceOffererOnSignalingStateChange.Do(func() {
			wasCalledMut.Lock()
			defer wasCalledMut.Unlock()
			wasCalled = append(wasCalled, "offerer OnSignalingStateChange")
			wg.Done()
		})
	})
	pcAnswer.OnICEConnectionStateChange(func(ICEConnectionState) {
		onceAnswererOnICEConnectionStateChange.Do(func() {
			wasCalledMut.Lock()
			defer wasCalledMut.Unlock()
			wasCalled = append(wasCalled, "answerer OnICEConnectionStateChange")
			wg.Done()
		})
	})
	pcAnswer.OnSignalingStateChange(func(SignalingState) {
		onceAnswererOnSignalingStateChange.Do(func() {
			wasCalledMut.Lock()
			defer wasCalledMut.Unlock()
			wasCalled = append(wasCalled, "answerer OnSignalingStateChange")
			wg.Done()
		})
	})

	// Use signalPair to establish a connection between pcOffer and pcAnswer. This
	// process should trigger the above event handlers.
	assert.NoError(t, signalPair(pcOffer, pcAnswer))

	// Wait for all of the event handlers to be triggered.
	done := make(chan struct{})
	go func() {
		wg.Wait()
		done <- struct{}{}
	}()
	timeout := time.After(5 * time.Second)
	select {
	case <-done:
		break
	case <-timeout:
		t.Fatalf("timed out waiting for one or more events handlers to be called (these *were* called: %+v)", wasCalled)
	}
}
