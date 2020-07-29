// +build !js

package webrtc

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"math/rand"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/sdp/v2"
	"github.com/pion/transport/test"
	"github.com/pion/webrtc/v3/pkg/media"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		expectedTrackID    = "video"
		expectedTrackLabel = "pion"
	)

	api := NewAPI()
	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	api.mediaEngine.RegisterDefaultCodecs()
	pcOffer, pcAnswer, err := api.newPair(Configuration{})
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

	pcAnswer.OnTrack(func(track *Track, receiver *RTPReceiver) {
		if track.ID() != expectedTrackID {
			trackMetadataValid <- fmt.Errorf("Incoming Track ID is invalid expected(%s) actual(%s)", expectedTrackID, track.ID())
			return
		}

		if track.Label() != expectedTrackLabel {
			trackMetadataValid <- fmt.Errorf("Incoming Track Label is invalid expected(%s) actual(%s)", expectedTrackLabel, track.Label())
			return
		}
		close(trackMetadataValid)

		go func() {
			for {
				time.Sleep(time.Millisecond * 100)
				if routineErr := pcAnswer.WriteRTCP([]rtcp.Packet{&rtcp.RapidResynchronizationRequest{SenderSSRC: track.SSRC(), MediaSSRC: track.SSRC()}}); routineErr != nil {
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

	vp8Track, err := pcOffer.NewTrack(DefaultPayloadTypeVP8, rand.Uint32(), expectedTrackID, expectedTrackLabel)
	if err != nil {
		t.Fatal(err)
	}
	rtpSender, err := pcOffer.AddTrack(vp8Track)
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		for {
			time.Sleep(time.Millisecond * 100)
			if routineErr := vp8Track.WriteSample(media.Sample{Data: []byte{0x00}, Samples: 1}); routineErr != nil {
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
			if routineErr := pcOffer.WriteRTCP([]rtcp.Packet{&rtcp.PictureLossIndication{SenderSSRC: vp8Track.SSRC(), MediaSSRC: vp8Track.SSRC()}}); routineErr != nil {
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
		if _, routineErr := rtpSender.Read(make([]byte, 1400)); routineErr == nil {
			close(awaitRTCPSenderRecv)
		}
	}()

	err = signalPair(pcOffer, pcAnswer)
	if err != nil {
		t.Fatal(err)
	}

	err, ok := <-trackMetadataValid
	if ok {
		t.Fatal(err)
	}

	<-awaitRTPRecv
	<-awaitRTPSend

	<-awaitRTCPSenderRecv
	err, ok = <-awaitRTCPSenderSend
	if ok {
		t.Fatal(err)
	}

	<-awaitRTCPReceiverRecv
	err, ok = <-awaitRTCPReceiverSend
	if ok {
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

	api := NewAPI()
	api.mediaEngine.RegisterDefaultCodecs()
	pcOffer, pcAnswer, err := api.newPair(Configuration{})
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

	opusTrack, err := pcOffer.NewTrack(DefaultPayloadTypeOpus, rand.Uint32(), "audio", "pion1")
	if err != nil {
		t.Fatal(err)
	}
	vp8Track, err := pcOffer.NewTrack(DefaultPayloadTypeVP8, rand.Uint32(), "video", "pion2")
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

	pcAnswer.OnTrack(func(track *Track, receiver *RTPReceiver) {
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

	api := NewAPI(WithSettingEngine(s))
	api.mediaEngine.RegisterDefaultCodecs()

	pcOffer, pcAnswer, err := api.newPair(Configuration{})
	if err != nil {
		t.Fatal(err)
	}

	vp8Track, err := pcOffer.NewTrack(DefaultPayloadTypeVP8, rand.Uint32(), "video", "pion2")
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
		if rtpErr := vp8Track.WriteSample(media.Sample{Data: []byte{0x00}, Samples: 1}); rtpErr != nil {
			t.Fatal(rtpErr)
		} else if rtcpErr := pcOffer.WriteRTCP([]rtcp.Packet{&rtcp.PictureLossIndication{MediaSSRC: 0}}); rtcpErr != nil {
			t.Fatal(rtcpErr)
		}
	}

	assert.NoError(t, pcOffer.Close())
}

/*
Integration test for behavior around media and closing

* Writing and Reading from tracks should return io.EOF when the PeerConnection is closed
*/
func TestPeerConnection_Media_Closed(t *testing.T) {
	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	api := NewAPI()
	api.mediaEngine.RegisterDefaultCodecs()
	pcOffer, pcAnswer, err := api.newPair(Configuration{})
	if err != nil {
		t.Fatal(err)
	}

	_, err = pcAnswer.AddTransceiverFromKind(RTPCodecTypeVideo)
	if err != nil {
		t.Fatal(err)
	}

	vp8Writer, err := pcOffer.NewTrack(DefaultPayloadTypeVP8, rand.Uint32(), "video", "pion2")
	if err != nil {
		t.Fatal(err)
	}

	if _, err = pcOffer.AddTrack(vp8Writer); err != nil {
		t.Fatal(err)
	}

	answerChan := make(chan *Track)
	pcAnswer.OnTrack(func(t *Track, r *RTPReceiver) {
		answerChan <- t
	})

	err = signalPair(pcOffer, pcAnswer)
	if err != nil {
		t.Fatal(err)
	}

	vp8Reader := func() *Track {
		for {
			if err = vp8Writer.WriteSample(media.Sample{Data: []byte{0x00}, Samples: 1}); err != nil {
				t.Fatal(err)
			}
			time.Sleep(time.Millisecond * 25)

			select {
			case t := <-answerChan:
				return t
			default:
				continue
			}
		}
	}()

	closeChan := make(chan error)
	go func() {
		time.Sleep(time.Second)
		closeChan <- pcAnswer.Close()
	}()
	if _, err = vp8Reader.Read(make([]byte, 1)); err != io.EOF {
		t.Fatal("Reading from closed Track did not return io.EOF")
	} else if err = <-closeChan; err != nil {
		t.Fatal(err)
	}

	assert.NoError(t, pcOffer.Close())
	assert.NoError(t, pcAnswer.Close())

	if err = vp8Writer.WriteSample(media.Sample{Data: []byte{0x00}, Samples: 1}); err != io.ErrClosedPipe {
		t.Fatal("Write to Track with no RTPSenders did not return io.ErrClosedPipe")
	} else if err = pcAnswer.WriteRTCP([]rtcp.Packet{&rtcp.RapidResynchronizationRequest{SenderSSRC: 0, MediaSSRC: 0}}); err != io.ErrClosedPipe {
		t.Fatal("WriteRTCP to closed PeerConnection did not return io.ErrClosedPipe")
	}
}

// If a SessionDescription has a single media section and no SSRC
// assume that it is meant to handle all RTP packets
func TestUndeclaredSSRC(t *testing.T) {
	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	api := NewAPI()
	api.mediaEngine.RegisterDefaultCodecs()
	pcOffer, pcAnswer, err := api.newPair(Configuration{})
	assert.NoError(t, err)

	_, err = pcOffer.CreateDataChannel("test-channel", nil)
	assert.NoError(t, err)

	_, err = pcAnswer.AddTransceiverFromKind(RTPCodecTypeVideo)
	assert.NoError(t, err)

	vp8Writer, err := pcOffer.NewTrack(DefaultPayloadTypeVP8, rand.Uint32(), "video", "pion2")
	assert.NoError(t, err)

	_, err = pcOffer.AddTrack(vp8Writer)
	assert.NoError(t, err)

	onTrackFired := make(chan *Track)
	pcAnswer.OnTrack(func(t *Track, r *RTPReceiver) {
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
		if strings.HasPrefix(l, "m=") {
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
			assert.NoError(t, vp8Writer.WriteSample(media.Sample{Data: []byte{0x00}, Samples: 1}))
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

func TestOfferRejectionMissingCodec(t *testing.T) {
	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	api := NewAPI()
	api.mediaEngine.RegisterDefaultCodecs()
	pc, err := api.NewPeerConnection(Configuration{})
	if err != nil {
		t.Fatal(err)
	}

	noCodecAPI := NewAPI()
	noCodecPC, err := noCodecAPI.NewPeerConnection(Configuration{})
	if err != nil {
		t.Fatal(err)
	}

	track, err := pc.NewTrack(DefaultPayloadTypeVP8, rand.Uint32(), "video", "pion2")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pc.AddTrack(track); err != nil {
		t.Fatal(err)
	}

	if err := signalPair(pc, noCodecPC); err != nil {
		t.Fatal(err)
	}

	var sdes sdp.SessionDescription
	if err := sdes.Unmarshal([]byte(pc.RemoteDescription().SDP)); err != nil {
		t.Fatal(err)
	}
	var videoDesc sdp.MediaDescription
	for _, m := range sdes.MediaDescriptions {
		if m.MediaName.Media == "video" {
			videoDesc = *m
		}
	}

	if got, want := videoDesc.MediaName.Formats, []string{"0"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("rejecting unknown codec: sdp m=%s, want trailing 0", *videoDesc.MediaName.String())
	}

	assert.NoError(t, noCodecPC.Close())
	assert.NoError(t, pc.Close())
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

	track, err := pc.NewTrack(
		DefaultPayloadTypeOpus,
		0xDEADBEEF,
		"track-id",
		"track-label",
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

	track, err := pc.NewTrack(
		DefaultPayloadTypeOpus,
		0xDEADBEEF,
		"track-id",
		"track-label",
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

	transceiver, err := pc.AddTransceiver(RTPCodecTypeVideo, RtpTransceiverInit{
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
	mediaEngine := MediaEngine{}
	mediaEngine.RegisterDefaultCodecs()
	api := NewAPI(WithMediaEngine(mediaEngine))
	pc, err := api.NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	tr, err := pc.AddTransceiverFromKind(
		RTPCodecTypeVideo,
		RtpTransceiverInit{Direction: RTPTransceiverDirectionRecvonly},
	)
	assert.NoError(t, err)

	assert.Equal(t, []*RTPTransceiver{tr}, pc.GetTransceivers())

	addTrack := func() (*Track, *RTPSender) {
		track, err := pc.NewTrack(DefaultPayloadTypeVP8, rand.Uint32(), "foo", "bar")
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
	mediaEngine := MediaEngine{}
	mediaEngine.RegisterDefaultCodecs()
	api := NewAPI(WithMediaEngine(mediaEngine))
	pc, err := api.NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	_, err = pc.AddTransceiverFromKind(
		RTPCodecTypeVideo,
		RtpTransceiverInit{Direction: RTPTransceiverDirectionRecvonly},
	)
	assert.NoError(t, err)

	dtlsTransport := pc.dtlsTransport
	pc.dtlsTransport = nil

	track, err := pc.NewTrack(DefaultPayloadTypeVP8, rand.Uint32(), "foo", "bar")
	assert.NoError(t, err)

	_, err = pc.AddTrack(track)
	assert.Error(t, err, "DTLSTransport must not be nil")

	assert.Equal(t, 1, len(pc.GetTransceivers()))

	pc.dtlsTransport = dtlsTransport
	assert.NoError(t, pc.Close())
}

func TestRtpSenderReceiver_ReadClose_Error(t *testing.T) {
	mediaEngine := MediaEngine{}
	mediaEngine.RegisterDefaultCodecs()
	api := NewAPI(WithMediaEngine(mediaEngine))
	pc, err := api.NewPeerConnection(Configuration{})
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

	track, err := pc.NewTrack(
		DefaultPayloadTypeH264,
		0xDEADBEEF,
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

func TestOmitMediaFromBundleIfUnsupported(t *testing.T) {
	const sdpOfferWithAudioAndVideo = `v=0
o=- 6476616870435111971 2 IN IP4 127.0.0.1
s=-
t=0 0
a=group:BUNDLE 0 1
m=audio 9 UDP/TLS/RTP/SAVPF 111
c=IN IP4 0.0.0.0
a=rtcp:9 IN IP4 0.0.0.0
a=ice-ufrag:sRIG
a=ice-pwd:yZb5ZMsBlPoK577sGhjvEUtT
a=ice-options:trickle
a=fingerprint:sha-256 27:EF:25:BF:57:45:BC:1C:0D:36:42:FF:5E:93:71:D2:41:58:EA:46:FD:A8:2A:F3:13:94:6E:E6:43:23:CB:D7
a=setup:actpass
a=mid:0
a=sendrecv
a=rtpmap:111 opus/48000/2
a=fmtp:111 minptime=10;useinbandfec=1
m=video 9 UDP/TLS/RTP/SAVPF 96
c=IN IP4 0.0.0.0
a=rtcp:9 IN IP4 0.0.0.0
a=ice-ufrag:sRIG
a=ice-pwd:yZb5ZMsBlPoK577sGhjvEUtT
a=ice-options:trickle
a=fingerprint:sha-256 27:EF:25:BF:57:45:BC:1C:0D:36:42:FF:5E:93:71:D2:41:58:EA:46:FD:A8:2A:F3:13:94:6E:E6:43:23:CB:D7
a=setup:actpass
a=mid:1
a=sendrecv
a=rtpmap:96 H264/90000
a=fmtp:96 level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=640c1f
`
	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	mediaEngine := MediaEngine{}
	mediaEngine.RegisterCodec(
		NewRTPH264Codec(DefaultPayloadTypeH264, 90000),
	)

	api := NewAPI(
		WithMediaEngine(mediaEngine),
	)

	pc, err := api.NewPeerConnection(Configuration{})
	if err != nil {
		t.Error(err.Error())
	}

	if err = pc.SetRemoteDescription(SessionDescription{
		Type: SDPTypeOffer,
		SDP:  sdpOfferWithAudioAndVideo,
	}); nil != err {
		t.Error(err.Error())
	}

	answer, err := pc.CreateAnswer(nil)
	if nil != err {
		t.Error(err.Error())
	}

	success := false
	for _, attr := range answer.parsed.Attributes {
		if attr.Key == "group" {
			if attr.Value == "BUNDLE 1" {
				success = true
			}
		}
	}

	if !success {
		t.Fail()
	}
	assert.NoError(t, pc.Close())
}

func TestGetRegisteredRTPCodecs(t *testing.T) {
	mediaEngine := MediaEngine{}
	expectedCodec := NewRTPH264Codec(DefaultPayloadTypeH264, 90000)
	mediaEngine.RegisterCodec(expectedCodec)

	api := NewAPI(WithMediaEngine(mediaEngine))
	pc, err := api.NewPeerConnection(Configuration{})
	if err != nil {
		t.Error(err.Error())
	}

	codecs := pc.GetRegisteredRTPCodecs(RTPCodecTypeVideo)
	if len(codecs) != 1 {
		t.Errorf("expected to get only 1 codec but got %d codecs", len(codecs))
	}

	actualCodec := codecs[0]
	if actualCodec != expectedCodec {
		t.Errorf("expected to get %v but got %v", expectedCodec, actualCodec)
	}

	assert.NoError(t, pc.Close())
}

func TestPlanBMediaExchange(t *testing.T) {
	runTest := func(trackCount int, t *testing.T) {
		addSingleTrack := func(p *PeerConnection) *Track {
			track, err := p.NewTrack(DefaultPayloadTypeVP8, rand.Uint32(), fmt.Sprintf("video-%d", rand.Uint32()), fmt.Sprintf("video-%d", rand.Uint32()))
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
		pcAnswer.OnTrack(func(track *Track, r *RTPReceiver) {
			onTrackWaitGroup.Done()
		})

		done := make(chan struct{})
		go func() {
			onTrackWaitGroup.Wait()
			close(done)
		}()

		_, err = pcAnswer.AddTransceiverFromKind(RTPCodecTypeVideo)
		assert.NoError(t, err)

		outboundTracks := []*Track{}
		for i := 0; i < trackCount; i++ {
			outboundTracks = append(outboundTracks, addSingleTrack(pcOffer))
		}

		assert.NoError(t, signalPair(pcOffer, pcAnswer))

		func() {
			for {
				select {
				case <-time.After(20 * time.Millisecond):
					for _, track := range outboundTracks {
						assert.NoError(t, track.WriteSample(media.Sample{Data: []byte{0x00}, Samples: 1}))
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

	track1, err := pcOffer.NewTrack(DefaultPayloadTypeVP8, rand.Uint32(), "video", "pion1")
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

	track2, err := pcOffer.NewTrack(DefaultPayloadTypeVP8, rand.Uint32(), "video", "pion2")
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
		return false, fmt.Errorf("no transceiver with mid %q", mid)
	}

	api := NewAPI()
	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	api.mediaEngine.RegisterDefaultCodecs()
	pcOffer, pcAnswer, err := api.newPair(Configuration{})
	require.NoError(t, err)

	_, err = pcAnswer.AddTransceiverFromKind(RTPCodecTypeVideo, RtpTransceiverInit{Direction: RTPTransceiverDirectionRecvonly})
	assert.NoError(t, err)

	track1, err := pcOffer.NewTrack(DefaultPayloadTypeVP8, rand.Uint32(), "video", "pion1")
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
