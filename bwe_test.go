package webrtc

import (
	"context"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/pion/interceptor"
	"github.com/pion/interceptor/pkg/cc"
	"github.com/pion/logging"
	"github.com/pion/rtcp"
	"github.com/pion/rtp"
	"github.com/pion/transport/test"
	"github.com/pion/transport/vnet"
	"github.com/stretchr/testify/assert"
)

const (
	defaultReferenceCapacity = 1 * vnet.MBit
	defaultMaxBurst          = 100 * vnet.KBit

	leftCIDR = "10.0.1.0/24"

	leftPublicIP1  = "10.0.1.1"
	leftPrivateIP1 = "10.0.1.101"

	leftPublicIP2  = "10.0.1.2"
	leftPrivateIP2 = "10.0.1.102"

	leftPublicIP3  = "10.0.1.3"
	leftPrivateIP3 = "10.0.1.103"

	rightCIDR = "10.0.2.0/24"

	rightPublicIP1  = "10.0.2.1"
	rightPrivateIP1 = "10.0.2.101"

	rightPublicIP2  = "10.0.2.2"
	rightPrivateIP2 = "10.0.2.102"

	rightPublicIP3  = "10.0.2.3"
	rightPrivateIP3 = "10.0.2.103"
)

var (
	defaultVideotrack = trackConfig{
		capability: RTPCodecCapability{MimeType: MimeTypeVP8},
		id:         "video1",
		streamID:   "pion",
		vbr:        true,
		codec:      newSimpleFPSBasedCodec(1 * vnet.MBit),
		startAfter: 0,
	}
	defaultAudioTrack = trackConfig{
		capability: RTPCodecCapability{MimeType: MimeTypeOpus},
		id:         "audio1",
		streamID:   "pion",
		codec:      newSimpleFPSBasedCodec(20 * vnet.KBit),
		vbr:        false,
		startAfter: 0,
	}

	senderRTPLogWriter = io.Discard
	//senderRTPLogWriter = os.Stdout

	senderRTCPLogWriter = io.Discard
	//senderRTCPLogWriter = os.Stdout

	receiverRTPLogWriter = io.Discard
	//receiverRTPLogWriter = os.Stdout

	receiverRTCPLogWriter = io.Discard
	//receiverRTCPLogWriter = os.Stdout
)

type bandwidthVariationPhase struct {
	duration      time.Duration
	capacityRatio float64
}

type trackConfig struct {
	capability RTPCodecCapability
	id         string
	streamID   string
	codec      syntheticCodec
	vbr        bool
	startAfter time.Duration
}

type senderReceiverPair struct {
	sender   sender
	receiver receiver
}

type testcase struct {
	name              string
	referenceCapacity int64
	totalDuration     time.Duration
	left              routerConfig
	right             routerConfig
	forward           []senderReceiverPair
	backward          []senderReceiverPair
	forwardPhases     []bandwidthVariationPhase
	backwardPhases    []bandwidthVariationPhase
}

var testCases = []testcase{
	{
		name:              "TestVariableAvailableCapacitySingleFlow",
		referenceCapacity: defaultReferenceCapacity,
		totalDuration:     100 * time.Second,
		left: routerConfig{
			cidr:      leftCIDR,
			staticIPs: []string{fmt.Sprintf("%v/%v", leftPublicIP1, leftPrivateIP1)},
		},
		right: routerConfig{
			cidr:      rightCIDR,
			staticIPs: []string{fmt.Sprintf("%v/%v", rightPublicIP1, rightPrivateIP1)},
		},
		forward: []senderReceiverPair{
			{
				sender: sender{
					privateIP:  leftPrivateIP1,
					publicIP:   leftPublicIP1,
					tracks:     []trackConfig{defaultAudioTrack, defaultVideotrack},
					rtpWriter:  senderRTPLogWriter,
					rtcpWriter: senderRTCPLogWriter,
				},
				receiver: receiver{
					privateIP:  rightPrivateIP1,
					publicIP:   rightPublicIP1,
					rtpWriter:  receiverRTPLogWriter,
					rtcpWriter: receiverRTCPLogWriter,
				},
			},
		},
		backward: []senderReceiverPair{},
		forwardPhases: []bandwidthVariationPhase{
			{duration: 40 * time.Second, capacityRatio: 1},
			{duration: 20 * time.Second, capacityRatio: 2.5},
			{duration: 20 * time.Second, capacityRatio: 0.6},
			{duration: 20 * time.Second, capacityRatio: 1.0},
		},
		backwardPhases: []bandwidthVariationPhase{},
	},
	{
		name:              "TestVariableAvailableCapacityMultipleFlow",
		referenceCapacity: defaultReferenceCapacity,
		totalDuration:     125 * time.Second,
		left: routerConfig{
			cidr:      leftCIDR,
			staticIPs: []string{fmt.Sprintf("%v/%v", leftPublicIP1, leftPrivateIP1), fmt.Sprintf("%v/%v", leftPublicIP2, leftPrivateIP2)},
		},
		right: routerConfig{
			cidr:      rightCIDR,
			staticIPs: []string{fmt.Sprintf("%v/%v", rightPublicIP1, rightPrivateIP1), fmt.Sprintf("%v/%v", rightPublicIP2, rightPrivateIP2)},
		},
		forward: []senderReceiverPair{
			{
				sender: sender{
					privateIP:  leftPrivateIP1,
					publicIP:   leftPublicIP1,
					tracks:     []trackConfig{defaultVideotrack, defaultAudioTrack},
					rtpWriter:  senderRTPLogWriter,
					rtcpWriter: senderRTCPLogWriter,
				},
				receiver: receiver{
					privateIP:  rightPrivateIP1,
					publicIP:   rightPublicIP1,
					rtpWriter:  receiverRTPLogWriter,
					rtcpWriter: receiverRTCPLogWriter,
				},
			},
			{
				sender: sender{
					privateIP:  leftPrivateIP2,
					publicIP:   leftPublicIP2,
					tracks:     []trackConfig{},
					rtpWriter:  senderRTPLogWriter,
					rtcpWriter: senderRTCPLogWriter,
				},
				receiver: receiver{
					privateIP:  rightPrivateIP2,
					publicIP:   rightPublicIP2,
					rtpWriter:  receiverRTPLogWriter,
					rtcpWriter: receiverRTCPLogWriter,
				},
			},
		},
		backward: []senderReceiverPair{},
		forwardPhases: []bandwidthVariationPhase{
			{duration: 25 * time.Second, capacityRatio: 2.0},
			{duration: 25 * time.Second, capacityRatio: 1.0},
			{duration: 25 * time.Second, capacityRatio: 1.75},
			{duration: 25 * time.Second, capacityRatio: 0.5},
			{duration: 25 * time.Second, capacityRatio: 1.0},
		},
		backwardPhases: []bandwidthVariationPhase{},
	},
	{
		name:              "TestCongestedFeedbackLinkWithBiDirectionalMediaFlows",
		referenceCapacity: defaultReferenceCapacity,
		totalDuration:     100 * time.Second,
		left: routerConfig{
			cidr: leftCIDR,
			staticIPs: []string{
				fmt.Sprintf("%v/%v", leftPublicIP1, leftPrivateIP1),
				fmt.Sprintf("%v/%v", leftPublicIP2, leftPrivateIP2),
			},
		},
		right: routerConfig{
			cidr: rightCIDR,
			staticIPs: []string{
				fmt.Sprintf("%v/%v", rightPublicIP1, rightPrivateIP1),
				fmt.Sprintf("%v/%v", rightPublicIP2, rightPrivateIP2),
			},
		},
		forward: []senderReceiverPair{
			{
				sender: sender{
					privateIP:  leftPrivateIP1,
					publicIP:   leftPublicIP1,
					tracks:     []trackConfig{defaultVideotrack, defaultAudioTrack},
					rtpWriter:  senderRTPLogWriter,
					rtcpWriter: senderRTCPLogWriter,
				},
				receiver: receiver{
					privateIP:  rightPrivateIP1,
					publicIP:   rightPublicIP1,
					rtpWriter:  receiverRTPLogWriter,
					rtcpWriter: receiverRTCPLogWriter,
				},
			},
		},
		backward: []senderReceiverPair{
			{
				sender: sender{
					privateIP:  rightPrivateIP2,
					publicIP:   rightPublicIP2,
					tracks:     []trackConfig{},
					rtpWriter:  senderRTPLogWriter,
					rtcpWriter: senderRTCPLogWriter,
				},
				receiver: receiver{
					privateIP:  leftPrivateIP2,
					publicIP:   leftPublicIP2,
					rtpWriter:  receiverRTPLogWriter,
					rtcpWriter: receiverRTCPLogWriter,
				},
			},
		},
		forwardPhases: []bandwidthVariationPhase{
			{duration: 20 * time.Second, capacityRatio: 2.0},
			{duration: 20 * time.Second, capacityRatio: 1.0},
			{duration: 20 * time.Second, capacityRatio: 0.5},
			{duration: 40 * time.Second, capacityRatio: 2.0},
		},
		backwardPhases: []bandwidthVariationPhase{
			{duration: 35 * time.Second, capacityRatio: 2.0},
			{duration: 35 * time.Second, capacityRatio: 0.8},
			{duration: 30 * time.Second, capacityRatio: 2.0},
		},
	},
	{
		name:              "TestRoundTripTimeFairness",
		referenceCapacity: 4 * vnet.MBit,
		totalDuration:     300 * time.Second,
		left: routerConfig{
			cidr: leftCIDR,
			staticIPs: []string{
				fmt.Sprintf("%v/%v", leftPublicIP1, leftPrivateIP1),
				fmt.Sprintf("%v/%v", leftPublicIP2, leftPrivateIP2),
				fmt.Sprintf("%v/%v", leftPublicIP3, leftPrivateIP3),
			},
		},
		right: routerConfig{
			cidr: rightCIDR,
			staticIPs: []string{
				fmt.Sprintf("%v/%v", rightPublicIP1, rightPrivateIP1),
				fmt.Sprintf("%v/%v", rightPublicIP2, rightPrivateIP2),
				fmt.Sprintf("%v/%v", rightPublicIP3, rightPrivateIP3),
			},
		},
		forward: []senderReceiverPair{
			{
				sender: sender{
					privateIP:  leftPrivateIP1,
					publicIP:   leftPublicIP1,
					tracks:     []trackConfig{},
					rtpWriter:  senderRTPLogWriter,
					rtcpWriter: senderRTCPLogWriter,
				},
				receiver: receiver{
					privateIP:  rightPrivateIP1,
					publicIP:   rightPublicIP1,
					rtpWriter:  receiverRTPLogWriter,
					rtcpWriter: receiverRTCPLogWriter,
				},
			},
			{
				sender: sender{
					privateIP:  leftPrivateIP2,
					publicIP:   leftPublicIP2,
					tracks:     []trackConfig{},
					rtpWriter:  senderRTPLogWriter,
					rtcpWriter: senderRTCPLogWriter,
				},
				receiver: receiver{
					privateIP: rightPrivateIP1,
					publicIP:  rightPublicIP1,
					// TODO(mathis): Use separate RTP loggers for separate pairs
					rtpWriter:  receiverRTPLogWriter,
					rtcpWriter: receiverRTCPLogWriter,
				},
			},
			{
				sender: sender{
					privateIP:  leftPrivateIP3,
					publicIP:   leftPublicIP3,
					tracks:     []trackConfig{},
					rtpWriter:  receiverRTPLogWriter,
					rtcpWriter: receiverRTCPLogWriter,
				},
				receiver: receiver{
					privateIP:  rightPrivateIP3,
					publicIP:   rightPublicIP3,
					rtpWriter:  receiverRTPLogWriter,
					rtcpWriter: receiverRTCPLogWriter,
				},
			},
		},
		backward:       []senderReceiverPair{},
		forwardPhases:  []bandwidthVariationPhase{},
		backwardPhases: []bandwidthVariationPhase{},
	},
}

func TestBandwidthEstimation(t *testing.T) {
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			log := logging.NewDefaultLoggerFactory().NewLogger("test")

			report := test.CheckRoutines(t)
			defer report()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			leftRouter, rightRouter, wan, err := createNetwork(ctx, tc.left, tc.right)
			assert.NoError(t, err)

			leftRouter.tbf.Set(vnet.TBFRate(tc.referenceCapacity), vnet.TBFMaxBurst(defaultMaxBurst))
			rightRouter.tbf.Set(vnet.TBFRate(tc.referenceCapacity), vnet.TBFMaxBurst(defaultMaxBurst))

			receivedMetrics := make(chan int)
			go func() {
				ticker := time.NewTicker(1 * time.Second)
				bytesReceived := 0
				for {
					select {
					case <-ctx.Done():
						return
					case b := <-receivedMetrics:
						bytesReceived += b
					case <-ticker.C:
						log.Tracef("received %v bit/s", bytesReceived*8)
						bytesReceived = 0
					}
				}
			}()
			onTrack := func(trackRemote *TrackRemote, rtpReceiver *RTPReceiver) {
				for {
					if err := rtpReceiver.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
						log.Errorf("failed to SetReadDeadline for rtpReceiver: %v", err)
					}
					if err := trackRemote.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
						log.Errorf("failed to SetReadDeadline for trackRemote: %v", err)
					}

					p, _, err := trackRemote.ReadRTP()
					if err == io.EOF {
						log.Info("trackRemote.ReadRTP received EOF")
						return
					}
					if err != nil {
						log.Infof("trackRemote.ReadRTP returned error: %v", err)
						return
					}
					receivedMetrics <- p.MarshalSize()
				}
			}

			mss := []*mediaSender{}
			for _, forward := range tc.forward {

				// TODO: Configure other BWE's here
				bwe := cc.GCCFactory()
				spc, err := forward.sender.createPeer(leftRouter.Router, bwe)
				assert.NoError(t, err)

				ms := newMediaSender(log, spc, bwe)
				for _, track := range forward.sender.tracks {
					assert.NoError(t, ms.addTrack(track))
				}
				mss = append(mss, ms)

				rpc, err := forward.receiver.createPeer(rightRouter.Router)
				assert.NoError(t, err)

				rpc.OnTrack(onTrack)

				wg := untilConnectionState(PeerConnectionStateConnected, spc, rpc)
				assert.NoError(t, signalPair(spc, rpc))
				defer closePairNow(t, spc, rpc)
				wg.Wait()
			}

			for _, backwardPair := range tc.backward {

				// TODO: Configure other BWE's here
				bwe := cc.GCCFactory()
				spc, err := backwardPair.sender.createPeer(rightRouter.Router, bwe)
				assert.NoError(t, err)
				ms := newMediaSender(log, spc, bwe)
				for _, track := range backwardPair.sender.tracks {
					assert.NoError(t, ms.addTrack(track))
				}
				mss = append(mss, ms)

				rpc, err := backwardPair.receiver.createPeer(leftRouter.Router)
				assert.NoError(t, err)

				rpc.OnTrack(onTrack)

				wg := untilConnectionState(PeerConnectionStateConnected, spc, rpc)
				assert.NoError(t, signalPair(spc, rpc))
				defer closePairNow(t, spc, rpc)
				wg.Wait()
			}

			for _, ms := range mss {
				go ms.start(ctx)
			}

			go func() {
				for _, phase := range tc.forwardPhases {
					nextRate := int64(float64(tc.referenceCapacity) * phase.capacityRatio)
					rightRouter.tbf.Set(vnet.TBFRate(nextRate), vnet.TBFMaxBurst(defaultMaxBurst))
					log.Tracef("updated forward link capacity to %v", nextRate)
					select {
					case <-ctx.Done():
						return
					case <-time.After(phase.duration):
					}
				}
			}()
			go func() {
				for _, phase := range tc.backwardPhases {
					nextRate := int64(float64(tc.referenceCapacity) * phase.capacityRatio)
					leftRouter.tbf.Set(vnet.TBFRate(nextRate), vnet.TBFMaxBurst(defaultMaxBurst))
					log.Tracef("updated backward link capacity to %v", nextRate)
					select {
					case <-ctx.Done():
						return
					case <-time.After(phase.duration):
					}
				}
			}()

			time.Sleep(tc.totalDuration)
			assert.NoError(t, wan.Stop())
		})
	}
}

func rtpFormat(pkt *rtp.Packet, attributes interceptor.Attributes) string {
	// TODO(mathis): Replace timestamp by attributes.GetTimestamp as soon as
	// implemented in interceptors
	return fmt.Sprintf("%v, %v, %v, %v, %v, %v, %v\n",
		time.Now().UnixMilli(),
		pkt.PayloadType,
		pkt.SSRC,
		pkt.SequenceNumber,
		pkt.Timestamp,
		pkt.Marker,
		pkt.MarshalSize(),
	)
}

func rtcpFormat(pkts []rtcp.Packet, attributes interceptor.Attributes) string {
	// TODO(mathis): Replace timestamp by attributes.GetTimestamp as soon as
	// implemented in interceptors
	res := fmt.Sprintf("%v\t", time.Now().UnixMilli())
	for _, pkt := range pkts {
		switch feedback := pkt.(type) {
		case *rtcp.TransportLayerCC:
			res += feedback.String()
		}
	}
	return res
}
