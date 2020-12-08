// +build !js

package webrtc

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pion/randutil"
	"github.com/pion/rtcp"
	"github.com/pion/rtp"
	"github.com/pion/transport/test"
	"github.com/pion/webrtc/v3/pkg/media"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	errIncomingTrackIDInvalid    = errors.New("incoming Track ID is invalid")
	errIncomingTrackLabelInvalid = errors.New("incoming Track Label is invalid")
	errNoTransceiverwithMid      = errors.New("no transceiver with mid")
)

func offerMediaHasDirection(offer SessionDescription, kind RTPCodecType, direction RTPTransceiverDirection) bool {
	for _, media := range offer.parsed.MediaDescriptions {
		if media.MediaName.Media == kind.String() {
			_, exists := media.Attribute(direction.String())
			return exists
		}
	}
	return false
}

/*
Integration test for bi-directional peers

This asserts we can send RTP and RTCP both ways, and blocks until
each side gets something (and asserts payload contents)
*/
// nolint: gocyclo
func TestPeerConnection_Media_Sample(t *testing.T) {
	const (
		expectedTrackID  = "video"
		expectedStreamID = "pion"
	)

	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	pcOffer, pcAnswer, err := newPair()
	if err != nil {
		t.Fatal(err)
	}

	awaitRTPRecv := make(chan bool)
	awaitRTPRecvClosed := make(chan bool)
	awaitRTPSend := make(chan bool)

	awaitRTCPSenderRecv := make(chan bool)
	awaitRTCPSenderSend := make(chan error)

	awaitRTCPReceiverRecv := make(chan error)
	awaitRTCPReceiverSend := make(chan error)

	trackMetadataValid := make(chan error)

	pcAnswer.OnTrack(func(track *TrackRemote, receiver *RTPReceiver) {
		if track.ID() != expectedTrackID {
			trackMetadataValid <- fmt.Errorf("%w: expected(%s) actual(%s)", errIncomingTrackIDInvalid, expectedTrackID, track.ID())
			return
		}

		if track.StreamID() != expectedStreamID {
			trackMetadataValid <- fmt.Errorf("%w: expected(%s) actual(%s)", errIncomingTrackLabelInvalid, expectedStreamID, track.StreamID())
			return
		}
		close(trackMetadataValid)

		go func() {
			for {
				time.Sleep(time.Millisecond * 100)
				if routineErr := pcAnswer.WriteRTCP([]rtcp.Packet{&rtcp.RapidResynchronizationRequest{SenderSSRC: uint32(track.SSRC()), MediaSSRC: uint32(track.SSRC())}}); routineErr != nil {
					awaitRTCPReceiverSend <- routineErr
					return
				}

				select {
				case <-awaitRTCPSenderRecv:
					close(awaitRTCPReceiverSend)
					return
				default:
				}
			}
		}()

		go func() {
			_, routineErr := receiver.Read(make([]byte, 1400))
			if routineErr != nil {
				awaitRTCPReceiverRecv <- routineErr
			} else {
				close(awaitRTCPReceiverRecv)
			}
		}()

		haveClosedAwaitRTPRecv := false
		for {
			p, routineErr := track.ReadRTP()
			if routineErr != nil {
				close(awaitRTPRecvClosed)
				return
			} else if bytes.Equal(p.Payload, []byte{0x10, 0x00}) && !haveClosedAwaitRTPRecv {
				haveClosedAwaitRTPRecv = true
				close(awaitRTPRecv)
			}
		}
	})

	vp8Track, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: "video/vp8"}, expectedTrackID, expectedStreamID)
	if err != nil {
		t.Fatal(err)
	}
	sender, err := pcOffer.AddTrack(vp8Track)
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		for {
			time.Sleep(time.Millisecond * 100)
			if routineErr := vp8Track.WriteSample(media.Sample{Data: []byte{0x00}, Duration: time.Second}); routineErr != nil {
				fmt.Println(routineErr)
			}

			select {
			case <-awaitRTPRecv:
				close(awaitRTPSend)
				return
			default:
			}
		}
	}()

	go func() {
		for {
			time.Sleep(time.Millisecond * 100)
			if routineErr := pcOffer.WriteRTCP([]rtcp.Packet{&rtcp.PictureLossIndication{SenderSSRC: uint32(sender.ssrc), MediaSSRC: uint32(sender.ssrc)}}); routineErr != nil {
				awaitRTCPSenderSend <- routineErr
			}

			select {
			case <-awaitRTCPReceiverRecv:
				close(awaitRTCPSenderSend)
				return
			default:
			}
		}
	}()

	go func() {
		if _, routineErr := sender.Read(make([]byte, 1400)); routineErr == nil {
			close(awaitRTCPSenderRecv)
		}
	}()

	assert.NoError(t, signalPair(pcOffer, pcAnswer))

	err, ok := <-trackMetadataValid
	if ok {
		t.Fatal(err)
	}

	<-awaitRTPRecv
	<-awaitRTPSend

	<-awaitRTCPSenderRecv
	if err, ok = <-awaitRTCPSenderSend; ok {
		t.Fatal(err)
	}

	<-awaitRTCPReceiverRecv
	if err, ok = <-awaitRTCPReceiverSend; ok {
		t.Fatal(err)
	}

	assert.NoError(t, pcOffer.Close())
	assert.NoError(t, pcAnswer.Close())
	<-awaitRTPRecvClosed
}

/*
PeerConnection should be able to be torn down at anytime
This test adds an input track and asserts

* OnTrack doesn't fire since no video packets will arrive
* No goroutine leaks
* No deadlocks on shutdown
*/
func TestPeerConnection_Media_Shutdown(t *testing.T) {
	iceCompleteAnswer := make(chan struct{})
	iceCompleteOffer := make(chan struct{})

	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	pcOffer, pcAnswer, err := newPair()
	if err != nil {
		t.Fatal(err)
	}

	_, err = pcOffer.AddTransceiverFromKind(RTPCodecTypeVideo, RtpTransceiverInit{Direction: RTPTransceiverDirectionRecvonly})
	if err != nil {
		t.Fatal(err)
	}

	_, err = pcAnswer.AddTransceiverFromKind(RTPCodecTypeAudio, RtpTransceiverInit{Direction: RTPTransceiverDirectionRecvonly})
	if err != nil {
		t.Fatal(err)
	}

	opusTrack, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: "audio/opus"}, "audio", "pion1")
	if err != nil {
		t.Fatal(err)
	}

	vp8Track, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: "video/vp8"}, "video", "pion2")
	if err != nil {
		t.Fatal(err)
	}

	if _, err = pcOffer.AddTrack(opusTrack); err != nil {
		t.Fatal(err)
	} else if _, err = pcAnswer.AddTrack(vp8Track); err != nil {
		t.Fatal(err)
	}

	var onTrackFiredLock sync.Mutex
	onTrackFired := false

	pcAnswer.OnTrack(func(track *TrackRemote, receiver *RTPReceiver) {
		onTrackFiredLock.Lock()
		defer onTrackFiredLock.Unlock()
		onTrackFired = true
	})

	pcAnswer.OnICEConnectionStateChange(func(iceState ICEConnectionState) {
		if iceState == ICEConnectionStateConnected {
			close(iceCompleteAnswer)
		}
	})
	pcOffer.OnICEConnectionStateChange(func(iceState ICEConnectionState) {
		if iceState == ICEConnectionStateConnected {
			close(iceCompleteOffer)
		}
	})

	err = signalPair(pcOffer, pcAnswer)
	if err != nil {
		t.Fatal(err)
	}
	<-iceCompleteAnswer
	<-iceCompleteOffer

	// Each PeerConnection should have one sender, one receiver and one transceiver
	for _, pc := range []*PeerConnection{pcOffer, pcAnswer} {
		senders := pc.GetSenders()
		if len(senders) != 1 {
			t.Errorf("Each PeerConnection should have one RTPSender, we have %d", len(senders))
		}

		receivers := pc.GetReceivers()
		if len(receivers) != 2 {
			t.Errorf("Each PeerConnection should have two RTPReceivers, we have %d", len(receivers))
		}

		transceivers := pc.GetTransceivers()
		if len(transceivers) != 2 {
			t.Errorf("Each PeerConnection should have two RTPTransceivers, we have %d", len(transceivers))
		}
	}

	assert.NoError(t, pcOffer.Close())
	assert.NoError(t, pcAnswer.Close())

	onTrackFiredLock.Lock()
	if onTrackFired {
		t.Fatalf("PeerConnection OnTrack fired even though we got no packets")
	}
	onTrackFiredLock.Unlock()
}

/*
Integration test for behavior around media and disconnected peers

* Sending RTP and RTCP to a disconnected Peer shouldn't return an error
*/
func TestPeerConnection_Media_Disconnected(t *testing.T) {
	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	s := SettingEngine{}
	s.SetICETimeouts(1*time.Second, 5*time.Second, 250*time.Millisecond)

	pcOffer, pcAnswer, err := newPair()
	if err != nil {
		t.Fatal(err)
	}

	vp8Track, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: "video/vp8"}, "video", "pion2")
	if err != nil {
		t.Fatal(err)
	}

	vp8Sender, err := pcOffer.AddTrack(vp8Track)
	if err != nil {
		t.Fatal(err)
	}

	haveDisconnected := make(chan error)
	pcOffer.OnICEConnectionStateChange(func(iceState ICEConnectionState) {
		if iceState == ICEConnectionStateDisconnected {
			close(haveDisconnected)
		} else if iceState == ICEConnectionStateConnected {
			// Assert that DTLS is done by pull remote certificate, don't tear down the PC early
			for {
				if len(vp8Sender.Transport().GetRemoteCertificate()) != 0 {
					pcAnswer.sctpTransport.lock.RLock()
					haveAssociation := pcAnswer.sctpTransport.association != nil
					pcAnswer.sctpTransport.lock.RUnlock()

					if haveAssociation {
						break
					}
				}

				time.Sleep(time.Second)
			}

			if pcCloseErr := pcAnswer.Close(); pcCloseErr != nil {
				haveDisconnected <- pcCloseErr
			}
		}
	})

	err = signalPair(pcOffer, pcAnswer)
	if err != nil {
		t.Fatal(err)
	}

	err, ok := <-haveDisconnected
	if ok {
		t.Fatal(err)
	}
	for i := 0; i <= 5; i++ {
		if rtpErr := vp8Track.WriteSample(media.Sample{Data: []byte{0x00}, Duration: time.Second}); rtpErr != nil {
			t.Fatal(rtpErr)
		} else if rtcpErr := pcOffer.WriteRTCP([]rtcp.Packet{&rtcp.PictureLossIndication{MediaSSRC: 0}}); rtcpErr != nil {
			t.Fatal(rtcpErr)
		}
	}

	assert.NoError(t, pcOffer.Close())
}

// If a SessionDescription has a single media section and no SSRC
// assume that it is meant to handle all RTP packets
func TestUndeclaredSSRC(t *testing.T) {
	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	pcOffer, pcAnswer, err := newPair()
	assert.NoError(t, err)

	_, err = pcAnswer.AddTransceiverFromKind(RTPCodecTypeVideo)
	assert.NoError(t, err)

	_, err = pcOffer.CreateDataChannel("test-channel", nil)
	assert.NoError(t, err)

	vp8Writer, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: "video/vp8"}, "video", "pion2")
	assert.NoError(t, err)

	_, err = pcOffer.AddTrack(vp8Writer)
	assert.NoError(t, err)

	onTrackFired := make(chan *TrackRemote)
	pcAnswer.OnTrack(func(t *TrackRemote, r *RTPReceiver) {
		close(onTrackFired)
	})

	offer, err := pcOffer.CreateOffer(nil)
	assert.NoError(t, err)

	offerGatheringComplete := GatheringCompletePromise(pcOffer)
	assert.NoError(t, pcOffer.SetLocalDescription(offer))

	<-offerGatheringComplete
	offer = *pcOffer.LocalDescription()

	// Filter SSRC lines, and remove SCTP
	filteredSDP := ""
	scanner := bufio.NewScanner(strings.NewReader(offer.SDP))
	inApplicationMedia := false
	for scanner.Scan() {
		l := scanner.Text()
		if strings.HasPrefix(l, "m=application") {
			inApplicationMedia = !inApplicationMedia
		} else if strings.HasPrefix(l, "a=ssrc") {
			continue
		}

		if inApplicationMedia {
			continue
		}

		filteredSDP += l + "\n"
	}

	offer.SDP = filteredSDP

	assert.NoError(t, pcAnswer.SetRemoteDescription(offer))

	answer, err := pcAnswer.CreateAnswer(nil)
	assert.NoError(t, err)

	answerGatheringComplete := GatheringCompletePromise(pcAnswer)
	assert.NoError(t, pcAnswer.SetLocalDescription(answer))
	<-answerGatheringComplete

	assert.NoError(t, pcOffer.SetRemoteDescription(*pcAnswer.LocalDescription()))

	go func() {
		for {
			assert.NoError(t, vp8Writer.WriteSample(media.Sample{Data: []byte{0x00}, Duration: time.Second}))
			time.Sleep(time.Millisecond * 25)

			select {
			case <-onTrackFired:
				return
			default:
				continue
			}
		}
	}()

	<-onTrackFired
	assert.NoError(t, pcOffer.Close())
	assert.NoError(t, pcAnswer.Close())
}

func TestAddTransceiverFromTrackSendOnly(t *testing.T) {
	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	pc, err := NewPeerConnection(Configuration{})
	if err != nil {
		t.Error(err.Error())
	}

	track, err := NewTrackLocalStaticSample(
		RTPCodecCapability{MimeType: "audio/Opus"},
		"track-id",
		"stream-id",
	)
	if err != nil {
		t.Error(err.Error())
	}

	transceiver, err := pc.AddTransceiverFromTrack(track, RtpTransceiverInit{
		Direction: RTPTransceiverDirectionSendonly,
	})
	if err != nil {
		t.Error(err.Error())
	}

	if transceiver.Receiver() != nil {
		t.Errorf("Transceiver shouldn't have a receiver")
	}

	if transceiver.Sender() == nil {
		t.Errorf("Transceiver should have a sender")
	}

	if len(pc.GetTransceivers()) != 1 {
		t.Errorf("PeerConnection should have one transceiver but has %d", len(pc.GetTransceivers()))
	}

	if len(pc.GetSenders()) != 1 {
		t.Errorf("PeerConnection should have one sender but has %d", len(pc.GetSenders()))
	}

	offer, err := pc.CreateOffer(nil)
	if err != nil {
		t.Error(err.Error())
	}

	if !offerMediaHasDirection(offer, RTPCodecTypeAudio, RTPTransceiverDirectionSendonly) {
		t.Errorf("Direction on SDP is not %s", RTPTransceiverDirectionSendonly)
	}

	assert.NoError(t, pc.Close())
}

func TestAddTransceiverFromTrackSendRecv(t *testing.T) {
	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	pc, err := NewPeerConnection(Configuration{})
	if err != nil {
		t.Error(err.Error())
	}

	track, err := NewTrackLocalStaticSample(
		RTPCodecCapability{MimeType: "audio/Opus"},
		"track-id",
		"stream-id",
	)
	if err != nil {
		t.Error(err.Error())
	}

	transceiver, err := pc.AddTransceiverFromTrack(track, RtpTransceiverInit{
		Direction: RTPTransceiverDirectionSendrecv,
	})
	if err != nil {
		t.Error(err.Error())
	}

	if transceiver.Receiver() == nil {
		t.Errorf("Transceiver should have a receiver")
	}

	if transceiver.Sender() == nil {
		t.Errorf("Transceiver should have a sender")
	}

	if len(pc.GetTransceivers()) != 1 {
		t.Errorf("PeerConnection should have one transceiver but has %d", len(pc.GetTransceivers()))
	}

	offer, err := pc.CreateOffer(nil)
	if err != nil {
		t.Error(err.Error())
	}

	if !offerMediaHasDirection(offer, RTPCodecTypeAudio, RTPTransceiverDirectionSendrecv) {
		t.Errorf("Direction on SDP is not %s", RTPTransceiverDirectionSendrecv)
	}
	assert.NoError(t, pc.Close())
}

// nolint: dupl
func TestAddTransceiver(t *testing.T) {
	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	pc, err := NewPeerConnection(Configuration{})
	if err != nil {
		t.Error(err.Error())
	}

	transceiver, err := pc.AddTransceiverFromKind(RTPCodecTypeVideo, RtpTransceiverInit{
		Direction: RTPTransceiverDirectionSendrecv,
	})
	if err != nil {
		t.Error(err.Error())
	}

	if transceiver.Receiver() == nil {
		t.Errorf("Transceiver should have a receiver")
	}

	if transceiver.Sender() == nil {
		t.Errorf("Transceiver should have a sender")
	}

	offer, err := pc.CreateOffer(nil)
	if err != nil {
		t.Error(err.Error())
	}

	if !offerMediaHasDirection(offer, RTPCodecTypeVideo, RTPTransceiverDirectionSendrecv) {
		t.Errorf("Direction on SDP is not %s", RTPTransceiverDirectionSendrecv)
	}
	assert.NoError(t, pc.Close())
}

func TestAddTransceiverAddTrack_Reuse(t *testing.T) {
	pc, err := NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	tr, err := pc.AddTransceiverFromKind(
		RTPCodecTypeVideo,
		RtpTransceiverInit{Direction: RTPTransceiverDirectionRecvonly},
	)
	assert.NoError(t, err)

	assert.Equal(t, []*RTPTransceiver{tr}, pc.GetTransceivers())

	addTrack := func() (TrackLocal, *RTPSender) {
		track, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: "video/vp8"}, "foo", "bar")
		assert.NoError(t, err)

		sender, err := pc.AddTrack(track)
		assert.NoError(t, err)

		return track, sender
	}

	track1, sender1 := addTrack()
	assert.Equal(t, 1, len(pc.GetTransceivers()))
	assert.Equal(t, sender1, tr.Sender())
	assert.Equal(t, track1, tr.Sender().track)
	require.NoError(t, pc.RemoveTrack(sender1))

	track2, _ := addTrack()
	assert.Equal(t, 1, len(pc.GetTransceivers()))
	assert.Equal(t, track2, tr.Sender().track)

	addTrack()
	assert.Equal(t, 2, len(pc.GetTransceivers()))

	assert.NoError(t, pc.Close())
}

func TestAddTransceiverAddTrack_NewRTPSender_Error(t *testing.T) {
	pc, err := NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	_, err = pc.AddTransceiverFromKind(
		RTPCodecTypeVideo,
		RtpTransceiverInit{Direction: RTPTransceiverDirectionRecvonly},
	)
	assert.NoError(t, err)

	dtlsTransport := pc.dtlsTransport
	pc.dtlsTransport = nil

	track, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: "video/vp8"}, "foo", "bar")
	assert.NoError(t, err)

	_, err = pc.AddTrack(track)
	assert.Error(t, err, "DTLSTransport must not be nil")

	assert.Equal(t, 1, len(pc.GetTransceivers()))

	pc.dtlsTransport = dtlsTransport
	assert.NoError(t, pc.Close())
}

func TestRtpSenderReceiver_ReadClose_Error(t *testing.T) {
	pc, err := NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	tr, err := pc.AddTransceiverFromKind(
		RTPCodecTypeVideo,
		RtpTransceiverInit{Direction: RTPTransceiverDirectionSendrecv},
	)
	assert.NoError(t, err)

	sender, receiver := tr.Sender(), tr.Receiver()
	assert.NoError(t, sender.Stop())
	_, err = sender.Read(make([]byte, 0, 1400))
	assert.Error(t, err, io.ErrClosedPipe)

	assert.NoError(t, receiver.Stop())
	_, err = receiver.Read(make([]byte, 0, 1400))
	assert.Error(t, err, io.ErrClosedPipe)

	assert.NoError(t, pc.Close())
}

// nolint: dupl
func TestAddTransceiverFromKind(t *testing.T) {
	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	pc, err := NewPeerConnection(Configuration{})
	if err != nil {
		t.Error(err.Error())
	}

	transceiver, err := pc.AddTransceiverFromKind(RTPCodecTypeVideo, RtpTransceiverInit{
		Direction: RTPTransceiverDirectionRecvonly,
	})
	if err != nil {
		t.Error(err.Error())
	}

	if transceiver.Receiver() == nil {
		t.Errorf("Transceiver should have a receiver")
	}

	if transceiver.Sender() != nil {
		t.Errorf("Transceiver shouldn't have a sender")
	}

	offer, err := pc.CreateOffer(nil)
	if err != nil {
		t.Error(err.Error())
	}

	if !offerMediaHasDirection(offer, RTPCodecTypeVideo, RTPTransceiverDirectionRecvonly) {
		t.Errorf("Direction on SDP is not %s", RTPTransceiverDirectionRecvonly)
	}
	assert.NoError(t, pc.Close())
}

func TestAddTransceiverFromKindFailsSendOnly(t *testing.T) {
	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	pc, err := NewPeerConnection(Configuration{})
	if err != nil {
		t.Error(err.Error())
	}

	transceiver, err := pc.AddTransceiverFromKind(RTPCodecTypeVideo, RtpTransceiverInit{
		Direction: RTPTransceiverDirectionSendonly,
	})

	if transceiver != nil {
		t.Error("AddTransceiverFromKind shouldn't succeed with Direction RTPTransceiverDirectionSendonly")
	}

	assert.NotNil(t, err)
	assert.NoError(t, pc.Close())
}

func TestAddTransceiverFromTrackFailsRecvOnly(t *testing.T) {
	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	pc, err := NewPeerConnection(Configuration{})
	if err != nil {
		t.Error(err.Error())
	}

	track, err := NewTrackLocalStaticSample(
		RTPCodecCapability{MimeType: "video/h264", SDPFmtpLine: "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42001f"},
		"track-id",
		"track-label",
	)
	if err != nil {
		t.Error(err.Error())
	}

	transceiver, err := pc.AddTransceiverFromTrack(track, RtpTransceiverInit{
		Direction: RTPTransceiverDirectionRecvonly,
	})

	if transceiver != nil {
		t.Error("AddTransceiverFromTrack shouldn't succeed with Direction RTPTransceiverDirectionRecvonly")
	}

	assert.NotNil(t, err)
	assert.NoError(t, pc.Close())
}

func TestPlanBMediaExchange(t *testing.T) {
	runTest := func(trackCount int, t *testing.T) {
		addSingleTrack := func(p *PeerConnection) *TrackLocalStaticSample {
			track, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: "video/vp8"}, fmt.Sprintf("video-%d", randutil.NewMathRandomGenerator().Uint32()), fmt.Sprintf("video-%d", randutil.NewMathRandomGenerator().Uint32()))
			assert.NoError(t, err)

			_, err = p.AddTrack(track)
			assert.NoError(t, err)

			return track
		}

		pcOffer, err := NewPeerConnection(Configuration{SDPSemantics: SDPSemanticsPlanB})
		assert.NoError(t, err)

		pcAnswer, err := NewPeerConnection(Configuration{SDPSemantics: SDPSemanticsPlanB})
		assert.NoError(t, err)

		var onTrackWaitGroup sync.WaitGroup
		onTrackWaitGroup.Add(trackCount)
		pcAnswer.OnTrack(func(track *TrackRemote, r *RTPReceiver) {
			onTrackWaitGroup.Done()
		})

		done := make(chan struct{})
		go func() {
			onTrackWaitGroup.Wait()
			close(done)
		}()

		_, err = pcAnswer.AddTransceiverFromKind(RTPCodecTypeVideo)
		assert.NoError(t, err)

		outboundTracks := []*TrackLocalStaticSample{}
		for i := 0; i < trackCount; i++ {
			outboundTracks = append(outboundTracks, addSingleTrack(pcOffer))
		}

		assert.NoError(t, signalPair(pcOffer, pcAnswer))

		func() {
			for {
				select {
				case <-time.After(20 * time.Millisecond):
					for _, track := range outboundTracks {
						assert.NoError(t, track.WriteSample(media.Sample{Data: []byte{0x00}, Duration: time.Second}))
					}
				case <-done:
					return
				}
			}
		}()

		assert.NoError(t, pcOffer.Close())
		assert.NoError(t, pcAnswer.Close())
	}

	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	t.Run("Single Track", func(t *testing.T) {
		runTest(1, t)
	})
	t.Run("Multi Track", func(t *testing.T) {
		runTest(2, t)
	})
}

// TestPeerConnection_Start_Only_Negotiated_Senders tests that only
// the current negotiated transceivers senders provided in an
// offer/answer are started
func TestPeerConnection_Start_Only_Negotiated_Senders(t *testing.T) {
	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	pcOffer, err := NewPeerConnection(Configuration{})
	assert.NoError(t, err)
	defer func() { assert.NoError(t, pcOffer.Close()) }()

	pcAnswer, err := NewPeerConnection(Configuration{})
	assert.NoError(t, err)
	defer func() { assert.NoError(t, pcAnswer.Close()) }()

	track1, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: "video/vp8"}, "video", "pion1")
	require.NoError(t, err)

	sender1, err := pcOffer.AddTrack(track1)
	require.NoError(t, err)

	offer, err := pcOffer.CreateOffer(nil)
	assert.NoError(t, err)

	offerGatheringComplete := GatheringCompletePromise(pcOffer)
	assert.NoError(t, pcOffer.SetLocalDescription(offer))
	<-offerGatheringComplete
	assert.NoError(t, pcAnswer.SetRemoteDescription(*pcOffer.LocalDescription()))
	answer, err := pcAnswer.CreateAnswer(nil)
	assert.NoError(t, err)
	answerGatheringComplete := GatheringCompletePromise(pcAnswer)
	assert.NoError(t, pcAnswer.SetLocalDescription(answer))
	<-answerGatheringComplete

	// Add a new track between providing the offer and applying the answer

	track2, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: "video/vp8"}, "video", "pion2")
	require.NoError(t, err)

	sender2, err := pcOffer.AddTrack(track2)
	require.NoError(t, err)

	// apply answer so we'll test generateMatchedSDP
	assert.NoError(t, pcOffer.SetRemoteDescription(*pcAnswer.LocalDescription()))

	// Wait for senders to be started by startTransports spawned goroutine
	pcOffer.ops.Done()

	// sender1 should be started but sender2 should not be started
	assert.True(t, sender1.hasSent(), "sender1 is not started but should be started")
	assert.False(t, sender2.hasSent(), "sender2 is started but should not be started")
}

// TestPeerConnection_Start_Right_Receiver tests that the right
// receiver (the receiver which transceiver has the same media section as the track)
// is started for the specified track
func TestPeerConnection_Start_Right_Receiver(t *testing.T) {
	isTransceiverReceiverStarted := func(pc *PeerConnection, mid string) (bool, error) {
		for _, transceiver := range pc.GetTransceivers() {
			if transceiver.Mid() != mid {
				continue
			}
			return transceiver.Receiver() != nil && transceiver.Receiver().haveReceived(), nil
		}
		return false, fmt.Errorf("%w: %q", errNoTransceiverwithMid, mid)
	}

	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	pcOffer, pcAnswer, err := newPair()
	require.NoError(t, err)

	_, err = pcAnswer.AddTransceiverFromKind(RTPCodecTypeVideo, RtpTransceiverInit{Direction: RTPTransceiverDirectionRecvonly})
	assert.NoError(t, err)

	track1, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: "video/vp8"}, "video", "pion1")
	require.NoError(t, err)

	sender1, err := pcOffer.AddTrack(track1)
	require.NoError(t, err)

	assert.NoError(t, signalPair(pcOffer, pcAnswer))

	pcOffer.ops.Done()
	pcAnswer.ops.Done()

	// transceiver with mid 0 should be started
	started, err := isTransceiverReceiverStarted(pcAnswer, "0")
	assert.NoError(t, err)
	assert.True(t, started, "transceiver with mid 0 should be started")

	// Remove track
	assert.NoError(t, pcOffer.RemoveTrack(sender1))

	assert.NoError(t, signalPair(pcOffer, pcAnswer))

	pcOffer.ops.Done()
	pcAnswer.ops.Done()

	// transceiver with mid 0 should not be started
	started, err = isTransceiverReceiverStarted(pcAnswer, "0")
	assert.NoError(t, err)
	assert.False(t, started, "transceiver with mid 0 should not be started")

	// Add a new transceiver (we're not using AddTrack since it'll reuse the transceiver with mid 0)
	_, err = pcOffer.AddTransceiverFromTrack(track1)
	assert.NoError(t, err)

	_, err = pcAnswer.AddTransceiverFromKind(RTPCodecTypeVideo, RtpTransceiverInit{Direction: RTPTransceiverDirectionRecvonly})
	assert.NoError(t, err)

	assert.NoError(t, signalPair(pcOffer, pcAnswer))

	pcOffer.ops.Done()
	pcAnswer.ops.Done()

	// transceiver with mid 0 should not be started
	started, err = isTransceiverReceiverStarted(pcAnswer, "0")
	assert.NoError(t, err)
	assert.False(t, started, "transceiver with mid 0 should not be started")
	// transceiver with mid 2 should be started
	started, err = isTransceiverReceiverStarted(pcAnswer, "2")
	assert.NoError(t, err)
	assert.True(t, started, "transceiver with mid 2 should be started")

	assert.NoError(t, pcOffer.Close())
	assert.NoError(t, pcAnswer.Close())
}

// Assert that failed Simulcast probing doesn't cause
// the handleUndeclaredSSRC to be leaked
func TestPeerConnection_Simulcast_Probe(t *testing.T) {
	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	track, err := NewTrackLocalStaticRTP(RTPCodecCapability{MimeType: "video/vp8"}, "video", "pion")
	assert.NoError(t, err)

	offerer, answerer, err := newPair()
	assert.NoError(t, err)

	_, err = offerer.AddTrack(track)
	assert.NoError(t, err)

	ticker := time.NewTicker(time.Millisecond * 20)
	testFinished := make(chan struct{})
	seenFiveStreams, seenFiveStreamsCancel := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-testFinished:
				return
			case <-ticker.C:
				answerer.dtlsTransport.lock.Lock()
				if len(answerer.dtlsTransport.simulcastStreams) >= 5 {
					seenFiveStreamsCancel()
				}
				answerer.dtlsTransport.lock.Unlock()

				track.mu.Lock()
				if len(track.bindings) == 1 {
					_, err = track.bindings[0].writeStream.WriteRTP(&rtp.Header{
						Version: 2,
						SSRC:    randutil.NewMathRandomGenerator().Uint32(),
					}, []byte{0, 1, 2, 3, 4, 5})
					assert.NoError(t, err)
				}
				track.mu.Unlock()
			}
		}
	}()

	assert.NoError(t, signalPair(offerer, answerer))

	peerConnectionConnected := sync.WaitGroup{}
	peerConnectionConnected.Add(2)

	connectionStateHandler := func(connectionState PeerConnectionState) {
		if connectionState == PeerConnectionStateConnected {
			peerConnectionConnected.Done()
		}
	}

	offerer.OnConnectionStateChange(connectionStateHandler)
	answerer.OnConnectionStateChange(connectionStateHandler)
	peerConnectionConnected.Wait()

	<-seenFiveStreams.Done()

	assert.NoError(t, answerer.Close())
	assert.NoError(t, offerer.Close())
	close(testFinished)
}
