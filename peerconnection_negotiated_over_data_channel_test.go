package webrtc

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pion/sctp"
	"github.com/pion/webrtc/v3/pkg/media"
	"github.com/stretchr/testify/assert"
)

// run `go test -v -race -run TestRenegotation -count 100 --failfast` to repro
func TestRenegotationOverDataChannel(t *testing.T) {
	var (
		clientNegChannelOpened <-chan struct{}
		clientNegChannelClosed <-chan struct{}
		serverNegChannelOpened <-chan struct{}
		serverNegChannelClosed <-chan struct{}
	)

	client, server, err := newPair()
	assert.NoError(t, err)
	defer func() {
		assert.NoError(t, client.Close())
		<-clientNegChannelClosed
	}()

	defer func() {
		assert.NoError(t, server.Close())
		<-serverNegChannelClosed
	}()

	// Add a renegotation channel. Set these channels up before signaling/answering.
	clientNegChannelOpened, clientNegChannelClosed, err = ConfigureForRenegotiation(client)
	assert.NoError(t, err)

	serverNegChannelOpened, serverNegChannelClosed, err = ConfigureForRenegotiation(server)
	assert.NoError(t, err)

	// Run signaling/answering such that the client + server can connect to each other.
	signalPair2(t, client, server)

	// Wait for the negotiation channels (aka data channels) to be ready.
	<-clientNegChannelOpened
	<-serverNegChannelOpened

	// This test observes a successful renegotiation by having a server create a video track, and
	// communicating this to the client via the `negotiation` DataChannel. And then sending data
	// over the video track. Install the `OnTrack` callback before kicking off the renegotiation via
	// the (server) `AddTrack` call.
	onTrack := atomic.Bool{}
	client.OnTrack(func(_ *TrackRemote, _ *RTPReceiver) {
		onTrack.Store(true)
	})

	trackLocal, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: "video/H264"},
		"video", "main+camera")
	assert.NoError(t, err)

	// `AddTrack` triggers the `PeerConnection.OnNegotiationNeeded` callback. Which will
	// asynchronously start our custom renegotiation code that sends offer/answer messages over the
	// `negotiation` DataChannel.
	_, err = server.AddTrack(trackLocal)
	assert.NoError(t, err)

	// Send data over the track until we observe the `OnTrack` callback is invoked within 10 seconds.
	for start := time.Now(); time.Since(start) < 10*time.Second; {
		err = trackLocal.WriteSample(media.Sample{Data: []byte{0, 0, 0, 0, 0}, Timestamp: time.Now(), Duration: time.Millisecond})
		assert.NoError(t, err)
		if onTrack.Load() == true {
			break
		}
		time.Sleep(time.Millisecond)
	}
	assert.True(t, onTrack.Load())
}

func signalPair2(t *testing.T, left, right *PeerConnection) {
	t.Helper()

	leftOffer, err := left.CreateOffer(nil)
	assert.NoError(t, err)
	err = left.SetLocalDescription(leftOffer)
	assert.NoError(t, err)
	<-GatheringCompletePromise(left)

	leftOffer.SDP = left.LocalDescription().SDP
	assert.NoError(t, right.SetRemoteDescription(leftOffer))

	rightAnswer, err := right.CreateAnswer(nil)
	assert.NoError(t, err)
	assert.NoError(t, right.SetLocalDescription(rightAnswer))
	<-GatheringCompletePromise(right)

	assert.NoError(t, left.SetRemoteDescription(rightAnswer))
}

// isUserInitiatedAbortChunkErr returns true if the error is an abort chunk
// error that the user initiated through Close. Certain browsers (Safari,
// Chrome and potentially others) close RTCPeerConnections with this type of
// abort chunk that is not indicative of an actual state of error.
func isUserInitiatedAbortChunkErr(err error) bool {
	return err != nil && errors.Is(err, sctp.ErrChunk) &&
		strings.Contains(err.Error(), "User Initiated Abort: Close called")
}

func initialDataChannelOnError(pc io.Closer) func(err error) {
	return func(err error) {
		if errors.Is(err, sctp.ErrResetPacketInStateNotExist) ||
			isUserInitiatedAbortChunkErr(err) {
			log.Printf("initialDataChannelOnError ignoreing err: %s", err)
			return
		}
		log.Printf("premature data channel error before WebRTC channel association %s", err)
		if err := pc.Close(); err != nil {
			log.Printf("pc.Close returned err: %s", err)
		}
	}
}

// EncodeSDP encodes the given SDP in base64.
func EncodeSDP(sdp *SessionDescription) (string, error) {
	b, err := json.Marshal(sdp)
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(b), nil
}

// DecodeSDP decodes the input from base64 into the given SDP.
func DecodeSDP(in string, sdp *SessionDescription) error {
	b, err := base64.StdEncoding.DecodeString(in)
	if err != nil {
		return err
	}

	err = json.Unmarshal(b, sdp)
	if err != nil {
		return err
	}
	return err
}

// ConfigureForRenegotiation sets up PeerConnection callbacks for updating local descriptions and
// sending offers when a negotiation is needed (e.g: adding a video track). As well as listening for
// offers/answers to update remote descriptions (e.g: when the peer adds a video track).
//
// If successful, two Go channels are returned. The first Go channel will close when the negotiation
// DataChannel is open and available for renegotiation. The second Go channel will close when the
// negotiation DataChannel is closed. PeerConnection.Close does not wait on DataChannel's to finish
// their work. Thus waiting on this can be helpful to guarantee background goroutines have exitted.
func ConfigureForRenegotiation(peerConn *PeerConnection) (<-chan struct{}, <-chan struct{}, error) {
	var negMu sync.Mutex
	negotiated := true
	// Packets over this channel must be processed in order (à la TCP).
	ordered := true
	negotiationChannelID := uint16(1)
	negotiationChannel, err := peerConn.CreateDataChannel("negotiation", &DataChannelInit{
		ID:         &negotiationChannelID,
		Negotiated: &negotiated,
		Ordered:    &ordered,
	})
	if err != nil {
		return nil, nil, err
	}

	negotiationChannel.OnError(initialDataChannelOnError(peerConn))

	// The pion webrtc library may invoke `OnNegotiationNeeded` prior to the connection being
	// established. We drop those requests on the floor. The original connection is established with
	// our signaling and answering machinery.
	//
	// Additionally, just because a PeerConnection has moved into the `connected` state, that does
	// not imply the pre-negotiated `negotiation` DataChannel is available for use. We return this
	// `negOpened` channel to let tests create a happens-before relationship. Such that these tests
	// can know when a PeerConnection method that invokes `OnNegotiationNeeded` can utilize this
	// negotiation channel.
	negOpened := make(chan struct{})
	negotiationChannel.OnOpen(func() {
		close(negOpened)
	})

	negClosed := make(chan struct{})
	negotiationChannel.OnClose(func() {
		close(negClosed)
	})

	// OnNegotiationNeeded is webrtc callback for when a PeerConnection is mutated in a way such
	// that its local description should change. Such as when a video track is added that should be
	// streamed to the peer.
	peerConn.OnNegotiationNeeded(func() {
		select {
		case <-negOpened:
		default:
			// Negotiation cannot occur over the negotiation channel until after the channel is in
			// operation.
			return
		}

		negMu.Lock()
		defer negMu.Unlock()
		// Creating an offer will generate the desired local description that includes the
		// modifications responsible for entering the callback. Such as adding a video track.
		offer, err := peerConn.CreateOffer(nil)
		if err != nil {
			log.Printf("renegotiation: error creating offer err: %s", err)
			return
		}

		// It's not clear to me why an offer is created from a `PeerConnection` just to call
		// `PeerConnection.SetLocalDescription`. And then when encoding the `Description` ("SDP")
		// for sending to the peer, we must call `PeerConnection.LocalDescription` rather than using
		// the `offer`. But it's easy to see that the `offer` and `peerConn.LocalDescription()` are
		// different (e.g: the latter includes ICE candidates), so it must be done this way.
		if err = peerConn.SetLocalDescription(offer); err != nil {
			log.Printf("renegotiation: error setting local description err: %s", err)
			return
		}

		// Encode and send the new local description to the peer over the `negotiation` channel. The
		// peer will respond over the negotiation channel with an answer. That answer will be used to
		// update the remote description.
		encodedSDP, err := EncodeSDP(peerConn.LocalDescription())
		if err != nil {
			log.Printf("renegotiation: error encoding SDP err: %s", err)
			return
		}
		if err := negotiationChannel.SendText(encodedSDP); err != nil {
			log.Printf("renegotiation: error sending SDP err: %s", err)
			return
		}
	})

	negotiationChannel.OnMessage(func(msg DataChannelMessage) {
		negMu.Lock()
		defer negMu.Unlock()

		description := SessionDescription{}
		if err := DecodeSDP(string(msg.Data), &description); err != nil {
			log.Printf("renegotiation: error decoding SDP err: %s ", err)
			return
		}

		// A new description was received over the negotiation channel. Use that to update the remote
		// description.
		if err := peerConn.SetRemoteDescription(description); err != nil {
			log.Printf("renegotiation: error setting remote description err: %s", err)
			return
		}

		// If the message was an offer, generate an answer, set it as the local description and
		// respond. Such that the peer can update its remote description.
		//
		// If the incoming message was an answer, the peers are now in sync and no further messages
		// are required.
		if description.Type != SDPTypeOffer {
			return
		}

		// Dan: It's unclear to me how error handling should happen here. Receiving an offer implies
		// the peer's local description is not in sync with our remote description for that
		// peer. I'm unsure of the long-term consequence of a pair of PeerConnections being in this
		// inconsistent state.
		answer, err := peerConn.CreateAnswer(nil)
		if err != nil {
			log.Printf("renegotiation: error creating answer err: %s", err)
			return
		}
		if err = peerConn.SetLocalDescription(answer); err != nil {
			log.Printf("renegotiation: error setting local description err: %s", err)
			return
		}

		encodedSDP, err := EncodeSDP(peerConn.LocalDescription())
		if err != nil {
			log.Printf("renegotiation: error encoding SDP err: %s", err)
			return
		}
		if err := negotiationChannel.SendText(encodedSDP); err != nil {
			log.Printf("renegotiation: error sending SDP err: %s", err)
			return
		}
	})

	return negOpened, negClosed, nil
}
