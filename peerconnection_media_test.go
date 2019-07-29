// +build !js

package webrtc

import (
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
	"github.com/pion/rtp"
	"github.com/pion/sdp/v2"
	"github.com/pion/transport/test"
	"github.com/pion/webrtc/v2/pkg/media"
	"github.com/stretchr/testify/assert"
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

func TestSRTPDrainLeak(t *testing.T) {
	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	sawRTCPDrainMessage := make(chan bool, 1)
	sawRTPDrainmessage := make(chan bool, 1)

	s := SettingEngine{
		LoggerFactory: testCatchAllLoggerFactory{
			callback: func(msg string) {
				if strings.Contains(msg, "Incoming unhandled RTP ssrc") {
					select {
					case sawRTPDrainmessage <- true:
					default:
					}
				} else if strings.Contains(msg, "Incoming unhandled RTCP ssrc") {
					select {
					case sawRTCPDrainMessage <- true:
					default:
					}
				}
			},
		},
	}
	api := NewAPI(WithSettingEngine(s))
	api.mediaEngine.RegisterDefaultCodecs()

	pcOffer, pcAnswer, err := api.newPair()
	if err != nil {
		t.Fatal(err)
	}

	dtlsConnected := make(chan interface{})

	pcOffer.dtlsTransport.OnStateChange(func(s DTLSTransportState) {
		if s == DTLSTransportStateConnected {
			close(dtlsConnected)
		}

	})

	err = signalPair(pcOffer, pcAnswer)
	if err != nil {
		t.Fatal(err)
	}

	<-dtlsConnected

	srtpSession, err := pcOffer.dtlsTransport.getSRTPSession()
	if err != nil {
		t.Fatal(err)
	}
	srtpStream, err := srtpSession.OpenWriteStream()
	if err != nil {
		t.Fatal(err)
	}

	srtcpSession, err := pcOffer.dtlsTransport.getSRTCPSession()
	if err != nil {
		t.Fatal(err)
	}
	srtcpStream, err := srtcpSession.OpenWriteStream()
	if err != nil {
		t.Fatal(err)
	}

	// Send 5 RTP/RTCP packets with different SSRCes
	var i uint32
	for i = 0; i < 5; i++ {
		if _, err = srtpStream.WriteRTP(&rtp.Header{Version: 2, SSRC: i}, []byte{0x00, 0x01, 0x03}); err != nil {
			t.Fatal(err)
		}
	}
	for i = 0; i < 5; i++ {
		var raw []byte
		raw, err = rtcp.Marshal([]rtcp.Packet{&rtcp.PictureLossIndication{MediaSSRC: i}})
		if err != nil {
			t.Fatal(err)
		}

		if _, err = srtcpStream.Write(raw); err != nil {
			t.Fatal(err)
		}
	}

	<-sawRTCPDrainMessage
	<-sawRTPDrainmessage
	err = pcOffer.Close()
	if err != nil {
		t.Fatal(err)
	}

	err = pcAnswer.Close()
	if err != nil {
		t.Fatal(err)
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
		expectedTrackID    = "video"
		expectedTrackLabel = "pion"
	)

	api := NewAPI()
	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	api.mediaEngine.RegisterDefaultCodecs()
	pcOffer, pcAnswer, err := api.newPair()
	if err != nil {
		t.Fatal(err)
	}

	_, err = pcAnswer.AddTransceiver(RTPCodecTypeVideo)
	if err != nil {
		t.Fatal(err)
	}

	awaitRTPRecv := make(chan bool)
	awaitRTPRecvClosed := make(chan bool)
	awaitRTPSend := make(chan bool)

	awaitRTCPSenderRecv := make(chan bool)
	awaitRTCPSenderSend := make(chan error)

	awaitRTCPRecieverRecv := make(chan error)
	awaitRTCPRecieverSend := make(chan error)

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
					awaitRTCPRecieverSend <- routineErr
					return
				}

				select {
				case <-awaitRTCPSenderRecv:
					close(awaitRTCPRecieverSend)
					return
				default:
				}
			}
		}()

		go func() {
			_, routineErr := receiver.Read(make([]byte, 1400))
			if routineErr != nil {
				awaitRTCPRecieverRecv <- routineErr
			} else {
				close(awaitRTCPRecieverRecv)
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
	rtpReceiver, err := pcOffer.AddTrack(vp8Track)
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
			case <-awaitRTCPRecieverRecv:
				close(awaitRTCPSenderSend)
				return
			default:
			}
		}
	}()

	go func() {
		if _, routineErr := rtpReceiver.Read(make([]byte, 1400)); routineErr == nil {
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

	<-awaitRTCPRecieverRecv
	err, ok = <-awaitRTCPRecieverSend
	if ok {
		t.Fatal(err)
	}

	err = pcOffer.Close()
	if err != nil {
		t.Fatal(err)
	}

	err = pcAnswer.Close()
	if err != nil {
		t.Fatal(err)
	}

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
	iceComplete := make(chan bool)

	lim := test.TimeOut(time.Second * 30)
	defer lim.Stop()

	report := test.CheckRoutines(t)
	defer report()

	api := NewAPI()
	api.mediaEngine.RegisterDefaultCodecs()
	pcOffer, pcAnswer, err := api.newPair()
	if err != nil {
		t.Fatal(err)
	}

	_, err = pcOffer.AddTransceiver(RTPCodecTypeVideo, RtpTransceiverInit{Direction: RTPTransceiverDirectionRecvonly})
	if err != nil {
		t.Fatal(err)
	}

	_, err = pcAnswer.AddTransceiver(RTPCodecTypeVideo, RtpTransceiverInit{Direction: RTPTransceiverDirectionRecvonly})
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

	var onTrackFiredLock sync.RWMutex
	onTrackFired := false

	pcAnswer.OnTrack(func(track *Track, receiver *RTPReceiver) {
		onTrackFiredLock.Lock()
		defer onTrackFiredLock.Unlock()
		onTrackFired = true
	})

	pcAnswer.OnICEConnectionStateChange(func(iceState ICEConnectionState) {
		if iceState == ICEConnectionStateConnected {
			go func() {
				time.Sleep(3 * time.Second) // TODO PeerConnection.Close() doesn't block for all subsystems
				close(iceComplete)
			}()
		}
	})

	err = signalPair(pcOffer, pcAnswer)
	if err != nil {
		t.Fatal(err)
	}
	<-iceComplete

	// Each PeerConnection should have one sender, one receiver and two transceivers
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

	err = pcOffer.Close()
	if err != nil {
		t.Fatal(err)
	}

	err = pcAnswer.Close()
	if err != nil {
		t.Fatal(err)
	}

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
	s.SetConnectionTimeout(time.Duration(1)*time.Second, time.Duration(250)*time.Millisecond)

	api := NewAPI(WithSettingEngine(s))
	api.mediaEngine.RegisterDefaultCodecs()

	pcOffer, pcAnswer, err := api.newPair()
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
					haveAssocation := pcAnswer.sctpTransport.association != nil
					pcAnswer.sctpTransport.lock.RUnlock()

					if haveAssocation {
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

	err = pcOffer.Close()
	if err != nil {
		t.Fatal(err)
	}
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
	pcOffer, pcAnswer, err := api.newPair()
	if err != nil {
		t.Fatal(err)
	}

	_, err = pcAnswer.AddTransceiver(RTPCodecTypeVideo)
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

	if err = pcOffer.Close(); err != nil {
		t.Fatal(err)
	} else if err = vp8Writer.WriteSample(media.Sample{Data: []byte{0x00}, Samples: 1}); err != io.ErrClosedPipe {
		t.Fatal("Write to Track with no RTPSenders did not return io.ErrClosedPipe")
	}

	if err = pcAnswer.WriteRTCP([]rtcp.Packet{&rtcp.RapidResynchronizationRequest{SenderSSRC: 0, MediaSSRC: 0}}); err != io.ErrClosedPipe {
		t.Fatal("WriteRTCP to closed PeerConnection did not return io.ErrClosedPipe")
	}

	err = pcOffer.Close()
	if err != nil {
		t.Fatal(err)
	}

	err = pcAnswer.Close()
	if err != nil {
		t.Fatal(err)
	}
}

func TestOfferRejectionMissingCodec(t *testing.T) {
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
}

func TestAddTransceiverFromTrackSendOnly(t *testing.T) {

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

	if transceiver.Receiver != nil {
		t.Errorf("Transceiver shouldn't have a receiver")
	}

	if transceiver.Sender == nil {
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
}

func TestAddTransceiverFromTrackSendRecv(t *testing.T) {

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

	if transceiver.Receiver == nil {
		t.Errorf("Transceiver should have a receiver")
	}

	if transceiver.Sender == nil {
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
}

func TestAddTransceiver(t *testing.T) {

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

	if transceiver.Receiver == nil {
		t.Errorf("Transceiver should have a receiver")
	}

	if transceiver.Sender == nil {
		t.Errorf("Transceiver should have a sender")
	}

	offer, err := pc.CreateOffer(nil)
	if err != nil {
		t.Error(err.Error())
	}

	if !offerMediaHasDirection(offer, RTPCodecTypeVideo, RTPTransceiverDirectionSendrecv) {
		t.Errorf("Direction on SDP is not %s", RTPTransceiverDirectionSendrecv)
	}
}

func TestAddTransceiverFromKind(t *testing.T) {

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

	if transceiver.Receiver == nil {
		t.Errorf("Transceiver should have a receiver")
	}

	if transceiver.Sender != nil {
		t.Errorf("Transceiver shouldn't have a sender")
	}

	offer, err := pc.CreateOffer(nil)
	if err != nil {
		t.Error(err.Error())
	}

	if !offerMediaHasDirection(offer, RTPCodecTypeVideo, RTPTransceiverDirectionRecvonly) {
		t.Errorf("Direction on SDP is not %s", RTPTransceiverDirectionRecvonly)
	}
}

func TestAddTransceiverFromKindFailsSendOnly(t *testing.T) {

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
}

func TestAddTransceiverFromTrackFailsRecvOnly(t *testing.T) {

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
}
