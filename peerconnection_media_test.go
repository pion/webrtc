// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js
// +build !js

package webrtc

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pion/logging"
	"github.com/pion/randutil"
	"github.com/pion/rtcp"
	"github.com/pion/rtp"
	"github.com/pion/sdp/v3"
	"github.com/pion/transport/v2/test"
	"github.com/pion/webrtc/v3/internal/util"
	"github.com/pion/webrtc/v3/pkg/media"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	errIncomingTrackIDInvalid    = errors.New("incoming Track ID is invalid")
	errIncomingTrackLabelInvalid = errors.New("incoming Track Label is invalid")
	errNoTransceiverwithMid      = errors.New("no transceiver with mid")
)

func registerSimulcastHeaderExtensions(m *MediaEngine, codecType RTPCodecType) {
	for _, extension := range []string{
		sdp.SDESMidURI,
		sdp.SDESRTPStreamIDURI,
		sdesRepairRTPStreamIDURI,
	} {
		if err := m.RegisterHeaderExtension(RTPHeaderExtensionCapability{URI: extension}, codecType); err != nil {
			panic(err)
		}
	}
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
			_, _, routineErr := receiver.Read(make([]byte, 1400))
			if routineErr != nil {
				awaitRTCPReceiverRecv <- routineErr
			} else {
				close(awaitRTCPReceiverRecv)
			}
		}()

		haveClosedAwaitRTPRecv := false
		for {
			p, _, routineErr := track.ReadRTP()
			if routineErr != nil {
				close(awaitRTPRecvClosed)
				return
			} else if bytes.Equal(p.Payload, []byte{0x10, 0x00}) && !haveClosedAwaitRTPRecv {
				haveClosedAwaitRTPRecv = true
				close(awaitRTPRecv)
			}
		}
	})

	vp8Track, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: MimeTypeVP8}, expectedTrackID, expectedStreamID)
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
			if pcOffer.ICEConnectionState() != ICEConnectionStateConnected {
				continue
			}
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
			if routineErr := pcOffer.WriteRTCP([]rtcp.Packet{&rtcp.PictureLossIndication{SenderSSRC: uint32(sender.trackEncodings[0].ssrc), MediaSSRC: uint32(sender.trackEncodings[0].ssrc)}}); routineErr != nil {
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
		if _, _, routineErr := sender.Read(make([]byte, 1400)); routineErr == nil {
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

	closePairNow(t, pcOffer, pcAnswer)
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

	_, err = pcOffer.AddTransceiverFromKind(RTPCodecTypeVideo, RTPTransceiverInit{Direction: RTPTransceiverDirectionRecvonly})
	if err != nil {
		t.Fatal(err)
	}

	_, err = pcAnswer.AddTransceiverFromKind(RTPCodecTypeAudio, RTPTransceiverInit{Direction: RTPTransceiverDirectionRecvonly})
	if err != nil {
		t.Fatal(err)
	}

	opusTrack, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: MimeTypeOpus}, "audio", "pion1")
	if err != nil {
		t.Fatal(err)
	}

	vp8Track, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: MimeTypeVP8}, "video", "pion2")
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

	closePairNow(t, pcOffer, pcAnswer)

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
	s.SetICETimeouts(time.Second/2, time.Second/2, time.Second/8)

	m := &MediaEngine{}
	assert.NoError(t, m.RegisterDefaultCodecs())

	pcOffer, pcAnswer, err := NewAPI(WithSettingEngine(s), WithMediaEngine(m)).newPair(Configuration{})
	if err != nil {
		t.Fatal(err)
	}

	vp8Track, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: MimeTypeVP8}, "video", "pion2")
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
					if pcAnswer.sctpTransport.association() != nil {
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

type undeclaredSsrcLogger struct{ unhandledSimulcastError chan struct{} }

func (u *undeclaredSsrcLogger) Trace(string)                  {}
func (u *undeclaredSsrcLogger) Tracef(string, ...interface{}) {}
func (u *undeclaredSsrcLogger) Debug(string)                  {}
func (u *undeclaredSsrcLogger) Debugf(string, ...interface{}) {}
func (u *undeclaredSsrcLogger) Info(string)                   {}
func (u *undeclaredSsrcLogger) Infof(string, ...interface{})  {}
func (u *undeclaredSsrcLogger) Warn(string)                   {}
func (u *undeclaredSsrcLogger) Warnf(string, ...interface{})  {}
func (u *undeclaredSsrcLogger) Error(string)                  {}
func (u *undeclaredSsrcLogger) Errorf(format string, _ ...interface{}) {
	if format == incomingUnhandledRTPSsrc {
		close(u.unhandledSimulcastError)
	}
}

type undeclaredSsrcLoggerFactory struct{ unhandledSimulcastError chan struct{} }

func (u *undeclaredSsrcLoggerFactory) NewLogger(string) logging.LeveledLogger {
	return &undeclaredSsrcLogger{u.unhandledSimulcastError}
}

// Filter SSRC lines
func filterSsrc(offer string) (filteredSDP string) {
	scanner := bufio.NewScanner(strings.NewReader(offer))
	for scanner.Scan() {
		l := scanner.Text()
		if strings.HasPrefix(l, "a=ssrc") {
			continue
		}

		filteredSDP += l + "\n"
	}
	return
}

// If a SessionDescription has a single media section and no SSRC
// assume that it is meant to handle all RTP packets
func TestUndeclaredSSRC(t *testing.T) {
	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	t.Run("No SSRC", func(t *testing.T) {
		pcOffer, pcAnswer, err := newPair()
		assert.NoError(t, err)

		vp8Writer, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: MimeTypeVP8}, "video", "pion2")
		assert.NoError(t, err)

		_, err = pcOffer.AddTrack(vp8Writer)
		assert.NoError(t, err)

		onTrackFired := make(chan struct{})
		pcAnswer.OnTrack(func(trackRemote *TrackRemote, r *RTPReceiver) {
			assert.Equal(t, trackRemote.StreamID(), vp8Writer.StreamID())
			assert.Equal(t, trackRemote.ID(), vp8Writer.ID())
			close(onTrackFired)
		})

		offer, err := pcOffer.CreateOffer(nil)
		assert.NoError(t, err)

		offerGatheringComplete := GatheringCompletePromise(pcOffer)
		assert.NoError(t, pcOffer.SetLocalDescription(offer))
		<-offerGatheringComplete

		offer.SDP = filterSsrc(pcOffer.LocalDescription().SDP)
		assert.NoError(t, pcAnswer.SetRemoteDescription(offer))

		answer, err := pcAnswer.CreateAnswer(nil)
		assert.NoError(t, err)

		answerGatheringComplete := GatheringCompletePromise(pcAnswer)
		assert.NoError(t, pcAnswer.SetLocalDescription(answer))
		<-answerGatheringComplete

		assert.NoError(t, pcOffer.SetRemoteDescription(*pcAnswer.LocalDescription()))

		sendVideoUntilDone(onTrackFired, t, []*TrackLocalStaticSample{vp8Writer})
		closePairNow(t, pcOffer, pcAnswer)
	})

	t.Run("Has RID", func(t *testing.T) {
		unhandledSimulcastError := make(chan struct{})

		m := &MediaEngine{}
		assert.NoError(t, m.RegisterDefaultCodecs())

		pcOffer, pcAnswer, err := NewAPI(WithSettingEngine(SettingEngine{
			LoggerFactory: &undeclaredSsrcLoggerFactory{unhandledSimulcastError},
		}), WithMediaEngine(m)).newPair(Configuration{})
		assert.NoError(t, err)

		vp8Writer, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: MimeTypeVP8}, "video", "pion2")
		assert.NoError(t, err)

		_, err = pcOffer.AddTrack(vp8Writer)
		assert.NoError(t, err)

		offer, err := pcOffer.CreateOffer(nil)
		assert.NoError(t, err)

		offerGatheringComplete := GatheringCompletePromise(pcOffer)
		assert.NoError(t, pcOffer.SetLocalDescription(offer))
		<-offerGatheringComplete

		// Append RID to end of SessionDescription. Will not be considered unhandled anymore
		offer.SDP = filterSsrc(pcOffer.LocalDescription().SDP) + "a=" + sdpAttributeRid + "\r\n"
		assert.NoError(t, pcAnswer.SetRemoteDescription(offer))

		answer, err := pcAnswer.CreateAnswer(nil)
		assert.NoError(t, err)

		answerGatheringComplete := GatheringCompletePromise(pcAnswer)
		assert.NoError(t, pcAnswer.SetLocalDescription(answer))
		<-answerGatheringComplete

		assert.NoError(t, pcOffer.SetRemoteDescription(*pcAnswer.LocalDescription()))

		sendVideoUntilDone(unhandledSimulcastError, t, []*TrackLocalStaticSample{vp8Writer})
		closePairNow(t, pcOffer, pcAnswer)
	})
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

	transceiver, err := pc.AddTransceiverFromTrack(track, RTPTransceiverInit{
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

	transceiver, err := pc.AddTransceiverFromTrack(track, RTPTransceiverInit{
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

func TestAddTransceiverAddTrack_Reuse(t *testing.T) {
	pc, err := NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	tr, err := pc.AddTransceiverFromKind(
		RTPCodecTypeVideo,
		RTPTransceiverInit{Direction: RTPTransceiverDirectionRecvonly},
	)
	assert.NoError(t, err)

	assert.Equal(t, []*RTPTransceiver{tr}, pc.GetTransceivers())

	addTrack := func() (TrackLocal, *RTPSender) {
		track, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: MimeTypeVP8}, "foo", "bar")
		assert.NoError(t, err)

		sender, err := pc.AddTrack(track)
		assert.NoError(t, err)

		return track, sender
	}

	track1, sender1 := addTrack()
	assert.Equal(t, 1, len(pc.GetTransceivers()))
	assert.Equal(t, sender1, tr.Sender())
	assert.Equal(t, track1, tr.Sender().Track())
	require.NoError(t, pc.RemoveTrack(sender1))

	track2, _ := addTrack()
	assert.Equal(t, 1, len(pc.GetTransceivers()))
	assert.Equal(t, track2, tr.Sender().Track())

	addTrack()
	assert.Equal(t, 2, len(pc.GetTransceivers()))

	assert.NoError(t, pc.Close())
}

func TestAddTransceiverAddTrack_NewRTPSender_Error(t *testing.T) {
	pc, err := NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	_, err = pc.AddTransceiverFromKind(
		RTPCodecTypeVideo,
		RTPTransceiverInit{Direction: RTPTransceiverDirectionRecvonly},
	)
	assert.NoError(t, err)

	dtlsTransport := pc.dtlsTransport
	pc.dtlsTransport = nil

	track, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: MimeTypeVP8}, "foo", "bar")
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
		RTPTransceiverInit{Direction: RTPTransceiverDirectionSendrecv},
	)
	assert.NoError(t, err)

	sender, receiver := tr.Sender(), tr.Receiver()
	assert.NoError(t, sender.Stop())
	_, _, err = sender.Read(make([]byte, 0, 1400))
	assert.ErrorIs(t, err, io.ErrClosedPipe)

	assert.NoError(t, receiver.Stop())
	_, _, err = receiver.Read(make([]byte, 0, 1400))
	assert.ErrorIs(t, err, io.ErrClosedPipe)

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

	transceiver, err := pc.AddTransceiverFromKind(RTPCodecTypeVideo, RTPTransceiverInit{
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
		RTPCodecCapability{MimeType: MimeTypeH264, SDPFmtpLine: "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42001f"},
		"track-id",
		"track-label",
	)
	if err != nil {
		t.Error(err.Error())
	}

	transceiver, err := pc.AddTransceiverFromTrack(track, RTPTransceiverInit{
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
			track, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: MimeTypeVP8}, fmt.Sprintf("video-%d", randutil.NewMathRandomGenerator().Uint32()), fmt.Sprintf("video-%d", randutil.NewMathRandomGenerator().Uint32()))
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

		closePairNow(t, pcOffer, pcAnswer)
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

	track1, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: MimeTypeVP8}, "video", "pion1")
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

	track2, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: MimeTypeVP8}, "video", "pion2")
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

	_, err = pcAnswer.AddTransceiverFromKind(RTPCodecTypeVideo, RTPTransceiverInit{Direction: RTPTransceiverDirectionRecvonly})
	assert.NoError(t, err)

	track1, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: MimeTypeVP8}, "video", "pion1")
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

	_, err = pcAnswer.AddTransceiverFromKind(RTPCodecTypeVideo, RTPTransceiverInit{Direction: RTPTransceiverDirectionRecvonly})
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

	closePairNow(t, pcOffer, pcAnswer)
}

func TestPeerConnection_Simulcast_Probe(t *testing.T) {
	lim := test.TimeOut(time.Second * 30) //nolint
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	// Assert that failed Simulcast probing doesn't cause
	// the handleUndeclaredSSRC to be leaked
	t.Run("Leak", func(t *testing.T) {
		track, err := NewTrackLocalStaticRTP(RTPCodecCapability{MimeType: MimeTypeVP8}, "video", "pion")
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

		peerConnectionConnected := untilConnectionState(PeerConnectionStateConnected, offerer, answerer)
		peerConnectionConnected.Wait()

		<-seenFiveStreams.Done()

		closePairNow(t, offerer, answerer)
		close(testFinished)
	})

	// Assert that NonSimulcast Traffic isn't incorrectly broken by the probe
	t.Run("Break NonSimulcast", func(t *testing.T) {
		unhandledSimulcastError := make(chan struct{})

		m := &MediaEngine{}
		if err := m.RegisterDefaultCodecs(); err != nil {
			panic(err)
		}
		registerSimulcastHeaderExtensions(m, RTPCodecTypeVideo)

		pcOffer, pcAnswer, err := NewAPI(WithSettingEngine(SettingEngine{
			LoggerFactory: &undeclaredSsrcLoggerFactory{unhandledSimulcastError},
		}), WithMediaEngine(m)).newPair(Configuration{})
		assert.NoError(t, err)

		firstTrack, err := NewTrackLocalStaticRTP(RTPCodecCapability{MimeType: MimeTypeVP8}, "firstTrack", "firstTrack")
		assert.NoError(t, err)

		_, err = pcOffer.AddTrack(firstTrack)
		assert.NoError(t, err)

		secondTrack, err := NewTrackLocalStaticRTP(RTPCodecCapability{MimeType: MimeTypeVP8}, "secondTrack", "secondTrack")
		assert.NoError(t, err)

		_, err = pcOffer.AddTrack(secondTrack)
		assert.NoError(t, err)

		assert.NoError(t, signalPairWithModification(pcOffer, pcAnswer, func(sessionDescription string) (filtered string) {
			shouldDiscard := false

			scanner := bufio.NewScanner(strings.NewReader(sessionDescription))
			for scanner.Scan() {
				if strings.HasPrefix(scanner.Text(), "m=video") {
					shouldDiscard = !shouldDiscard
				}

				if !shouldDiscard {
					filtered += scanner.Text() + "\r\n"
				}
			}

			return
		}))

		sequenceNumber := uint16(0)
		sendRTPPacket := func() {
			sequenceNumber++
			assert.NoError(t, firstTrack.WriteRTP(&rtp.Packet{
				Header: rtp.Header{
					Version:        2,
					SequenceNumber: sequenceNumber,
				},
				Payload: []byte{0x00},
			}))
			time.Sleep(20 * time.Millisecond)
		}

		for ; sequenceNumber <= 5; sequenceNumber++ {
			sendRTPPacket()
		}

		assert.NoError(t, signalPair(pcOffer, pcAnswer))

		trackRemoteChan := make(chan *TrackRemote, 1)
		pcAnswer.OnTrack(func(trackRemote *TrackRemote, _ *RTPReceiver) {
			trackRemoteChan <- trackRemote
		})

		trackRemote := func() *TrackRemote {
			for {
				select {
				case t := <-trackRemoteChan:
					return t
				default:
					sendRTPPacket()
				}
			}
		}()

		func() {
			for {
				select {
				case <-unhandledSimulcastError:
					return
				default:
					sendRTPPacket()
				}
			}
		}()

		_, _, err = trackRemote.Read(make([]byte, 1500))
		assert.NoError(t, err)

		closePairNow(t, pcOffer, pcAnswer)
	})
}

// Assert that CreateOffer returns an error for a RTPSender with no codecs
// pion/webrtc#1702
func TestPeerConnection_CreateOffer_NoCodecs(t *testing.T) {
	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	m := &MediaEngine{}

	pc, err := NewAPI(WithMediaEngine(m)).NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	track, err := NewTrackLocalStaticRTP(RTPCodecCapability{MimeType: MimeTypeVP8}, "video", "pion")
	assert.NoError(t, err)

	_, err = pc.AddTrack(track)
	assert.NoError(t, err)

	_, err = pc.CreateOffer(nil)
	assert.Equal(t, err, ErrSenderWithNoCodecs)

	assert.NoError(t, pc.Close())
}

// Assert that AddTrack is thread-safe
func TestPeerConnection_RaceReplaceTrack(t *testing.T) {
	pc, err := NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	addTrack := func() *TrackLocalStaticSample {
		track, err := NewTrackLocalStaticSample(RTPCodecCapability{MimeType: MimeTypeVP8}, "foo", "bar")
		assert.NoError(t, err)
		_, err = pc.AddTrack(track)
		assert.NoError(t, err)
		return track
	}

	for i := 0; i < 10; i++ {
		addTrack()
	}
	for _, tr := range pc.GetTransceivers() {
		assert.NoError(t, pc.RemoveTrack(tr.Sender()))
	}

	var wg sync.WaitGroup
	tracks := make([]*TrackLocalStaticSample, 10)
	wg.Add(10)
	for i := 0; i < 10; i++ {
		go func(j int) {
			tracks[j] = addTrack()
			wg.Done()
		}(i)
	}

	wg.Wait()

	for _, track := range tracks {
		have := false
		for _, t := range pc.GetTransceivers() {
			if t.Sender() != nil && t.Sender().Track() == track {
				have = true
				break
			}
		}
		if !have {
			t.Errorf("track was added but not found on senders")
		}
	}

	assert.NoError(t, pc.Close())
}

func TestPeerConnection_Simulcast(t *testing.T) {
	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	rids := []string{"a", "b", "c"}
	var ridMapLock sync.RWMutex
	ridMap := map[string]int{}

	// Enable Extension Headers needed for Simulcast
	m := &MediaEngine{}
	if err := m.RegisterDefaultCodecs(); err != nil {
		panic(err)
	}
	registerSimulcastHeaderExtensions(m, RTPCodecTypeVideo)

	assertRidCorrect := func(t *testing.T) {
		ridMapLock.Lock()
		defer ridMapLock.Unlock()

		for _, rid := range rids {
			assert.Equal(t, ridMap[rid], 1)
		}
		assert.Equal(t, len(ridMap), 3)
	}

	ridsFullfilled := func() bool {
		ridMapLock.Lock()
		defer ridMapLock.Unlock()

		ridCount := len(ridMap)
		return ridCount == 3
	}

	onTrackHandler := func(trackRemote *TrackRemote, _ *RTPReceiver) {
		ridMapLock.Lock()
		defer ridMapLock.Unlock()
		ridMap[trackRemote.RID()] = ridMap[trackRemote.RID()] + 1
	}

	t.Run("RTP Extension Based", func(t *testing.T) {
		pcOffer, pcAnswer, err := NewAPI(WithMediaEngine(m)).newPair(Configuration{})
		assert.NoError(t, err)

		vp8WriterA, err := NewTrackLocalStaticRTP(RTPCodecCapability{MimeType: MimeTypeVP8}, "video", "pion2", WithRTPStreamID("a"))
		assert.NoError(t, err)

		sender, err := pcOffer.AddTrack(vp8WriterA)
		assert.NoError(t, err)
		assert.NotNil(t, sender)

		vp8WriterB, err := NewTrackLocalStaticRTP(RTPCodecCapability{MimeType: MimeTypeVP8}, "video", "pion2", WithRTPStreamID("b"))
		assert.NoError(t, err)
		err = sender.AddEncoding(vp8WriterB)
		assert.NoError(t, err)

		vp8WriterC, err := NewTrackLocalStaticRTP(RTPCodecCapability{MimeType: MimeTypeVP8}, "video", "pion2", WithRTPStreamID("c"))
		assert.NoError(t, err)
		err = sender.AddEncoding(vp8WriterC)
		assert.NoError(t, err)

		ridMap = map[string]int{}
		pcAnswer.OnTrack(onTrackHandler)

		parameters := sender.GetParameters()
		assert.Equal(t, "a", parameters.Encodings[0].RID)
		assert.Equal(t, "b", parameters.Encodings[1].RID)
		assert.Equal(t, "c", parameters.Encodings[2].RID)

		var midID, ridID, rsidID uint8
		for _, extension := range parameters.HeaderExtensions {
			switch extension.URI {
			case sdp.SDESMidURI:
				midID = uint8(extension.ID)
			case sdp.SDESRTPStreamIDURI:
				ridID = uint8(extension.ID)
			case sdesRepairRTPStreamIDURI:
				rsidID = uint8(extension.ID)
			}
		}
		assert.NotZero(t, midID)
		assert.NotZero(t, ridID)
		assert.NotZero(t, rsidID)

		assert.NoError(t, signalPair(pcOffer, pcAnswer))

		// padding only packets should not affect simulcast probe
		var sequenceNumber uint16
		for sequenceNumber = 0; sequenceNumber < simulcastProbeCount+10; sequenceNumber++ {
			time.Sleep(20 * time.Millisecond)

			for _, track := range []*TrackLocalStaticRTP{vp8WriterA, vp8WriterB, vp8WriterC} {
				pkt := &rtp.Packet{
					Header: rtp.Header{
						Version:        2,
						SequenceNumber: sequenceNumber,
						PayloadType:    96,
						Padding:        true,
					},
					Payload: []byte{0x00, 0x02},
				}

				assert.NoError(t, track.WriteRTP(pkt))
			}
		}
		assert.False(t, ridsFullfilled(), "Simulcast probe should not be fulfilled by padding only packets")

		for ; !ridsFullfilled(); sequenceNumber++ {
			time.Sleep(20 * time.Millisecond)

			for ssrc, rid := range rids {
				header := &rtp.Header{
					Version:        2,
					SSRC:           uint32(ssrc),
					SequenceNumber: sequenceNumber,
					PayloadType:    96,
				}
				assert.NoError(t, header.SetExtension(midID, []byte("0")))

				// Send RSID for first 10 packets
				if sequenceNumber >= 10 {
					assert.NoError(t, header.SetExtension(ridID, []byte(rid)))
				} else {
					assert.NoError(t, header.SetExtension(rsidID, []byte(rid)))
					header.SSRC += 10
				}

				var writer *TrackLocalStaticRTP
				switch rid {
				case "a":
					writer = vp8WriterA
				case "b":
					writer = vp8WriterB
				case "c":
					writer = vp8WriterC
				}
				_, err = writer.bindings[0].writeStream.WriteRTP(header, []byte{0x00})
				assert.NoError(t, err)
			}
		}

		assertRidCorrect(t)
		closePairNow(t, pcOffer, pcAnswer)
	})
}

type simulcastTestTrackLocal struct {
	*TrackLocalStaticRTP
}

// don't use ssrc&payload in bindings to let the test write different stream packets.
func (s *simulcastTestTrackLocal) WriteRTP(pkt *rtp.Packet) error {
	packet := getPacketAllocationFromPool()

	defer resetPacketPoolAllocation(packet)

	*packet = *pkt

	s.mu.RLock()
	defer s.mu.RUnlock()

	writeErrs := []error{}

	for _, b := range s.bindings {
		if _, err := b.writeStream.WriteRTP(&packet.Header, packet.Payload); err != nil {
			writeErrs = append(writeErrs, err)
		}
	}

	return util.FlattenErrs(writeErrs)
}

func TestPeerConnection_Simulcast_RTX(t *testing.T) {
	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	rids := []string{"a", "b"}
	pcOffer, pcAnswer, err := newPair()
	assert.NoError(t, err)

	vp8WriterAStatic, err := NewTrackLocalStaticRTP(RTPCodecCapability{MimeType: MimeTypeVP8}, "video", "pion2", WithRTPStreamID(rids[0]))
	assert.NoError(t, err)

	vp8WriterBStatic, err := NewTrackLocalStaticRTP(RTPCodecCapability{MimeType: MimeTypeVP8}, "video", "pion2", WithRTPStreamID(rids[1]))
	assert.NoError(t, err)

	vp8WriterA, vp8WriterB := &simulcastTestTrackLocal{vp8WriterAStatic}, &simulcastTestTrackLocal{vp8WriterBStatic}

	sender, err := pcOffer.AddTrack(vp8WriterA)
	assert.NoError(t, err)
	assert.NotNil(t, sender)

	assert.NoError(t, sender.AddEncoding(vp8WriterB))

	var ridMapLock sync.RWMutex
	ridMap := map[string]int{}

	assertRidCorrect := func(t *testing.T) {
		ridMapLock.Lock()
		defer ridMapLock.Unlock()

		for _, rid := range rids {
			assert.Equal(t, ridMap[rid], 1)
		}
		assert.Equal(t, len(ridMap), 2)
	}

	ridsFullfilled := func() bool {
		ridMapLock.Lock()
		defer ridMapLock.Unlock()

		ridCount := len(ridMap)
		return ridCount == 2
	}

	var rtxPacketRead atomic.Int32
	var wg sync.WaitGroup
	wg.Add(2)

	pcAnswer.OnTrack(func(trackRemote *TrackRemote, _ *RTPReceiver) {
		ridMapLock.Lock()
		ridMap[trackRemote.RID()] = ridMap[trackRemote.RID()] + 1
		ridMapLock.Unlock()

		defer wg.Done()

		for {
			_, attr, rerr := trackRemote.ReadRTP()
			if rerr != nil {
				break
			}
			if pt, ok := attr.Get(AttributeRtxPayloadType).(byte); ok {
				if pt == 97 {
					rtxPacketRead.Add(1)
				}
			}
		}
	})

	parameters := sender.GetParameters()
	assert.Equal(t, "a", parameters.Encodings[0].RID)
	assert.Equal(t, "b", parameters.Encodings[1].RID)

	var midID, ridID, rsid uint8
	for _, extension := range parameters.HeaderExtensions {
		switch extension.URI {
		case sdp.SDESMidURI:
			midID = uint8(extension.ID)
		case sdp.SDESRTPStreamIDURI:
			ridID = uint8(extension.ID)
		case sdesRepairRTPStreamIDURI:
			rsid = uint8(extension.ID)
		}
	}
	assert.NotZero(t, midID)
	assert.NotZero(t, ridID)
	assert.NotZero(t, rsid)

	err = signalPairWithModification(pcOffer, pcAnswer, func(sdp string) string {
		// Original chrome sdp contains no ssrc info https://pastebin.com/raw/JTjX6zg6
		re := regexp.MustCompile("(?m)[\r\n]+^.*a=ssrc.*$")
		res := re.ReplaceAllString(sdp, "")
		return res
	})
	assert.NoError(t, err)

	// padding only packets should not affect simulcast probe
	var sequenceNumber uint16
	for sequenceNumber = 0; sequenceNumber < simulcastProbeCount+10; sequenceNumber++ {
		time.Sleep(20 * time.Millisecond)

		for i, track := range []*simulcastTestTrackLocal{vp8WriterA, vp8WriterB} {
			pkt := &rtp.Packet{
				Header: rtp.Header{
					Version:        2,
					SequenceNumber: sequenceNumber,
					PayloadType:    96,
					Padding:        true,
					SSRC:           uint32(i),
				},
				Payload: []byte{0x00, 0x02},
			}

			assert.NoError(t, track.WriteRTP(pkt))
		}
	}
	assert.False(t, ridsFullfilled(), "Simulcast probe should not be fulfilled by padding only packets")

	for ; !ridsFullfilled(); sequenceNumber++ {
		time.Sleep(20 * time.Millisecond)

		for i, track := range []*simulcastTestTrackLocal{vp8WriterA, vp8WriterB} {
			pkt := &rtp.Packet{
				Header: rtp.Header{
					Version:        2,
					SequenceNumber: sequenceNumber,
					PayloadType:    96,
					SSRC:           uint32(i),
				},
				Payload: []byte{0x00},
			}
			assert.NoError(t, pkt.Header.SetExtension(midID, []byte("0")))
			assert.NoError(t, pkt.Header.SetExtension(ridID, []byte(track.RID())))

			assert.NoError(t, track.WriteRTP(pkt))
		}
	}

	assertRidCorrect(t)

	for i := 0; i < simulcastProbeCount+10; i++ {
		sequenceNumber++
		time.Sleep(10 * time.Millisecond)

		for j, track := range []*simulcastTestTrackLocal{vp8WriterA, vp8WriterB} {
			pkt := &rtp.Packet{
				Header: rtp.Header{
					Version:        2,
					SequenceNumber: sequenceNumber,
					PayloadType:    97,
					SSRC:           uint32(100 + j),
				},
				Payload: []byte{0x00, 0x00, 0x00, 0x00, 0x00},
			}
			assert.NoError(t, pkt.Header.SetExtension(midID, []byte("0")))
			assert.NoError(t, pkt.Header.SetExtension(ridID, []byte(track.RID())))
			assert.NoError(t, pkt.Header.SetExtension(rsid, []byte(track.RID())))

			assert.NoError(t, track.WriteRTP(pkt))
		}
	}

	for ; rtxPacketRead.Load() == 0; sequenceNumber++ {
		time.Sleep(20 * time.Millisecond)

		for i, track := range []*simulcastTestTrackLocal{vp8WriterA, vp8WriterB} {
			pkt := &rtp.Packet{
				Header: rtp.Header{
					Version:        2,
					SequenceNumber: sequenceNumber,
					PayloadType:    96,
					SSRC:           uint32(i),
				},
				Payload: []byte{0x00},
			}
			assert.NoError(t, pkt.Header.SetExtension(midID, []byte("0")))
			assert.NoError(t, pkt.Header.SetExtension(ridID, []byte(track.RID())))

			assert.NoError(t, track.WriteRTP(pkt))
		}
	}

	closePairNow(t, pcOffer, pcAnswer)

	wg.Wait()

	assert.Greater(t, rtxPacketRead.Load(), int32(0), "no rtx packet read")
}

// Everytime we receieve a new SSRC we probe it and try to determine the proper way to handle it.
// In most cases a Track explicitly declares a SSRC and a OnTrack is fired. In two cases we don't
// know the SSRC ahead of time
// * Undeclared SSRC in a single media section (https://github.com/pion/webrtc/issues/880)
// * Simulcast
//
// The Undeclared SSRC processing code would run before Simulcast. If a Simulcast Offer/Answer only
// contained one Media Section we would never fire the OnTrack. We would assume it was a failed
// Undeclared SSRC processing. This test asserts that we properly handled this.
func TestPeerConnection_Simulcast_NoDataChannel(t *testing.T) {
	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	// Enable Extension Headers needed for Simulcast
	m := &MediaEngine{}
	if err := m.RegisterDefaultCodecs(); err != nil {
		panic(err)
	}
	registerSimulcastHeaderExtensions(m, RTPCodecTypeVideo)

	pcSender, pcReceiver, err := NewAPI(WithMediaEngine(m)).newPair(Configuration{})
	assert.NoError(t, err)

	var wg sync.WaitGroup
	wg.Add(4)

	var connectionWg sync.WaitGroup
	connectionWg.Add(2)

	connectionStateChangeHandler := func(state PeerConnectionState) {
		if state == PeerConnectionStateConnected {
			connectionWg.Done()
		}
	}

	pcSender.OnConnectionStateChange(connectionStateChangeHandler)
	pcReceiver.OnConnectionStateChange(connectionStateChangeHandler)

	pcReceiver.OnTrack(func(track *TrackRemote, _ *RTPReceiver) {
		defer wg.Done()
	})

	go func() {
		defer wg.Done()
		vp8WriterA, err := NewTrackLocalStaticRTP(RTPCodecCapability{MimeType: MimeTypeVP8}, "video", "pion", WithRTPStreamID("a"))
		assert.NoError(t, err)

		sender, err := pcSender.AddTrack(vp8WriterA)
		assert.NoError(t, err)
		assert.NotNil(t, sender)

		vp8WriterB, err := NewTrackLocalStaticRTP(RTPCodecCapability{MimeType: MimeTypeVP8}, "video", "pion", WithRTPStreamID("b"))
		assert.NoError(t, err)
		err = sender.AddEncoding(vp8WriterB)
		assert.NoError(t, err)

		vp8WriterC, err := NewTrackLocalStaticRTP(RTPCodecCapability{MimeType: MimeTypeVP8}, "video", "pion", WithRTPStreamID("c"))
		assert.NoError(t, err)
		err = sender.AddEncoding(vp8WriterC)
		assert.NoError(t, err)

		parameters := sender.GetParameters()
		var midID, ridID, rsidID uint8
		for _, extension := range parameters.HeaderExtensions {
			switch extension.URI {
			case sdp.SDESMidURI:
				midID = uint8(extension.ID)
			case sdp.SDESRTPStreamIDURI:
				ridID = uint8(extension.ID)
			case sdesRepairRTPStreamIDURI:
				rsidID = uint8(extension.ID)
			}
		}
		assert.NotZero(t, midID)
		assert.NotZero(t, ridID)
		assert.NotZero(t, rsidID)

		// signaling
		offerSDP, err := pcSender.CreateOffer(nil)
		assert.NoError(t, err)
		err = pcSender.SetLocalDescription(offerSDP)
		assert.NoError(t, err)

		err = pcReceiver.SetRemoteDescription(offerSDP)
		assert.NoError(t, err)
		answerSDP, err := pcReceiver.CreateAnswer(nil)
		assert.NoError(t, err)

		answerGatheringComplete := GatheringCompletePromise(pcReceiver)
		err = pcReceiver.SetLocalDescription(answerSDP)
		assert.NoError(t, err)
		<-answerGatheringComplete

		assert.NoError(t, pcSender.SetRemoteDescription(*pcReceiver.LocalDescription()))

		connectionWg.Wait()

		var seqNo uint16
		for i := 0; i < 100; i++ {
			pkt := &rtp.Packet{
				Header: rtp.Header{
					Version:        2,
					SequenceNumber: seqNo,
					PayloadType:    96,
				},
				Payload: []byte{0x00, 0x00},
			}

			assert.NoError(t, pkt.SetExtension(ridID, []byte("a")))
			assert.NoError(t, pkt.SetExtension(midID, []byte(sender.rtpTransceiver.Mid())))
			assert.NoError(t, vp8WriterA.WriteRTP(pkt))

			assert.NoError(t, pkt.SetExtension(ridID, []byte("b")))
			assert.NoError(t, pkt.SetExtension(midID, []byte(sender.rtpTransceiver.Mid())))
			assert.NoError(t, vp8WriterB.WriteRTP(pkt))

			assert.NoError(t, pkt.SetExtension(ridID, []byte("c")))
			assert.NoError(t, pkt.SetExtension(midID, []byte(sender.rtpTransceiver.Mid())))
			assert.NoError(t, vp8WriterC.WriteRTP(pkt))

			seqNo++
		}
	}()

	wg.Wait()

	closePairNow(t, pcSender, pcReceiver)
}
