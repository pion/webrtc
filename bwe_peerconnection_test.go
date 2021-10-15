package webrtc

import (
	"fmt"
	"io"
	"time"

	"github.com/pion/interceptor"
	"github.com/pion/interceptor/pkg/cc"
	"github.com/pion/interceptor/pkg/packetdump"
	"github.com/pion/transport/vnet"
)

type sender struct {
	privateIP string
	publicIP  string

	tracks []trackConfig

	rtpWriter  io.Writer
	rtcpWriter io.Writer
}

func (s *sender) createPeer(router *vnet.Router, bwe cc.BandwidthEstimator) (*PeerConnection, error) {
	sendNet := vnet.NewNet(&vnet.NetConfig{
		StaticIPs: []string{s.privateIP},
		StaticIP:  "",
	})

	if err := router.AddNet(sendNet); err != nil {
		return nil, fmt.Errorf("failed to add sendNet to routerA: %w", err)
	}

	offerSettingEngine := SettingEngine{}
	offerSettingEngine.SetVNet(sendNet)
	offerSettingEngine.SetICETimeouts(time.Second, time.Second, 200*time.Millisecond)
	offerSettingEngine.SetNAT1To1IPs([]string{s.publicIP}, ICECandidateTypeHost)

	offerMediaEngine := &MediaEngine{}
	if err := offerMediaEngine.RegisterDefaultCodecs(); err != nil {
		return nil, err
	}

	offerRTPDumperInterceptor, err := packetdump.NewSenderInterceptor(
		packetdump.RTPFormatter(rtpFormat),
		packetdump.RTPWriter(s.rtpWriter),
	)
	if err != nil {
		return nil, err
	}
	offerRTCPDumperInterceptor, err := packetdump.NewReceiverInterceptor(
		packetdump.RTCPFormatter(rtcpFormat),
		packetdump.RTCPWriter(s.rtcpWriter),
	)
	if err != nil {
		return nil, err
	}

	offerInterceptorRegistry := &interceptor.Registry{}
	offerInterceptorRegistry.Add(offerRTPDumperInterceptor)
	offerInterceptorRegistry.Add(offerRTCPDumperInterceptor)

	ccInterceptor, err := cc.NewControllerInterceptor(cc.SetBWE(func() cc.BandwidthEstimator { return bwe }))
	if err != nil {
		return nil, err
	}
	offerInterceptorRegistry.Add(ccInterceptor)

	err = ConfigureTWCCHeaderExtensionSender(offerMediaEngine, offerInterceptorRegistry)
	if err != nil {
		return nil, err
	}

	offerPeerConnection, err := NewAPI(
		WithSettingEngine(offerSettingEngine),
		WithMediaEngine(offerMediaEngine),
		WithInterceptorRegistry(offerInterceptorRegistry),
	).NewPeerConnection(Configuration{})
	if err != nil {
		return nil, err
	}

	return offerPeerConnection, nil
}

type receiver struct {
	privateIP string
	publicIP  string

	rtpWriter  io.Writer
	rtcpWriter io.Writer
}

func (r *receiver) createPeer(router *vnet.Router) (*PeerConnection, error) {
	receiveNet := vnet.NewNet(&vnet.NetConfig{
		StaticIPs: []string{r.privateIP},
		StaticIP:  "",
	})
	if err := router.AddNet(receiveNet); err != nil {
		return nil, fmt.Errorf("failed to add receiveNet to routerB: %w", err)
	}

	answerSettingEngine := SettingEngine{}
	answerSettingEngine.SetVNet(receiveNet)
	answerSettingEngine.SetICETimeouts(time.Second, time.Second, 200*time.Millisecond)
	answerSettingEngine.SetNAT1To1IPs([]string{r.publicIP}, ICECandidateTypeHost)

	answerMediaEngine := &MediaEngine{}
	if err := answerMediaEngine.RegisterDefaultCodecs(); err != nil {
		return nil, err
	}

	answerRTPDumperInterceptor, err := packetdump.NewReceiverInterceptor(
		packetdump.RTPFormatter(rtpFormat),
		packetdump.RTPWriter(r.rtpWriter),
	)
	if err != nil {
		return nil, err
	}
	answerRTCPDumperInterceptor, err := packetdump.NewSenderInterceptor(
		packetdump.RTCPFormatter(rtcpFormat),
		packetdump.RTCPWriter(r.rtcpWriter),
	)
	if err != nil {
		return nil, err
	}

	answerInterceptorRegistry := &interceptor.Registry{}
	answerInterceptorRegistry.Add(answerRTPDumperInterceptor)
	answerInterceptorRegistry.Add(answerRTCPDumperInterceptor)
	err = ConfigureTWCCSender(answerMediaEngine, answerInterceptorRegistry)
	if err != nil {
		return nil, err
	}

	answerPeerConnection, err := NewAPI(
		WithSettingEngine(answerSettingEngine),
		WithMediaEngine(answerMediaEngine),
		WithInterceptorRegistry(answerInterceptorRegistry),
	).NewPeerConnection(Configuration{})
	if err != nil {
		return nil, err
	}

	return answerPeerConnection, nil
}
