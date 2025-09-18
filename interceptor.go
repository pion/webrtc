// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js
// +build !js

package webrtc

import (
	"sync"
	"sync/atomic"

	"github.com/pion/interceptor"
	"github.com/pion/interceptor/pkg/flexfec"
	"github.com/pion/interceptor/pkg/nack"
	"github.com/pion/interceptor/pkg/report"
	"github.com/pion/interceptor/pkg/rfc8888"
	"github.com/pion/interceptor/pkg/stats"
	"github.com/pion/interceptor/pkg/twcc"
	"github.com/pion/rtp"
	"github.com/pion/sdp/v3"
)

// RegisterDefaultInterceptors will register some useful interceptors.
// If you want to customize which interceptors are loaded, you should copy the
// code from this method and remove unwanted interceptors.
func RegisterDefaultInterceptors(mediaEngine *MediaEngine, interceptorRegistry *interceptor.Registry) error {
	if err := ConfigureNack(mediaEngine, interceptorRegistry); err != nil {
		return err
	}

	if err := ConfigureRTCPReports(interceptorRegistry); err != nil {
		return err
	}

	if err := ConfigureSimulcastExtensionHeaders(mediaEngine); err != nil {
		return err
	}

	if err := ConfigureStatsInterceptor(interceptorRegistry); err != nil {
		return err
	}

	return ConfigureTWCCSender(mediaEngine, interceptorRegistry)
}

// ConfigureStatsInterceptor will setup everything necessary for generating RTP stream statistics.
func ConfigureStatsInterceptor(interceptorRegistry *interceptor.Registry) error {
	statsInterceptor, err := stats.NewInterceptor()
	if err != nil {
		return err
	}
	statsInterceptor.OnNewPeerConnection(func(id string, stats stats.Getter) {
		statsGetter.Store(id, stats)
	})
	interceptorRegistry.Add(statsInterceptor)

	return nil
}

// lookupStats returns the stats getter for a given peerconnection.statsId.
func lookupStats(id string) (stats.Getter, bool) {
	if value, exists := statsGetter.Load(id); exists {
		if getter, ok := value.(stats.Getter); ok {
			return getter, true
		}
	}

	return nil, false
}

// key: string (peerconnection.statsId), value: stats.Getter
var statsGetter sync.Map // nolint:gochecknoglobals

// ConfigureRTCPReports will setup everything necessary for generating Sender and Receiver Reports.
func ConfigureRTCPReports(interceptorRegistry *interceptor.Registry) error {
	reciver, err := report.NewReceiverInterceptor()
	if err != nil {
		return err
	}

	sender, err := report.NewSenderInterceptor()
	if err != nil {
		return err
	}

	interceptorRegistry.Add(reciver)
	interceptorRegistry.Add(sender)

	return nil
}

// ConfigureNack will setup everything necessary for handling generating/responding to nack messages.
func ConfigureNack(mediaEngine *MediaEngine, interceptorRegistry *interceptor.Registry) error {
	generator, err := nack.NewGeneratorInterceptor()
	if err != nil {
		return err
	}

	responder, err := nack.NewResponderInterceptor()
	if err != nil {
		return err
	}

	mediaEngine.RegisterFeedback(RTCPFeedback{Type: "nack"}, RTPCodecTypeVideo)
	mediaEngine.RegisterFeedback(RTCPFeedback{Type: "nack", Parameter: "pli"}, RTPCodecTypeVideo)
	interceptorRegistry.Add(responder)
	interceptorRegistry.Add(generator)

	return nil
}

// ConfigureTWCCHeaderExtensionSender will setup everything necessary for adding
// a TWCC header extension to outgoing RTP packets. This will allow the remote peer to generate TWCC reports.
func ConfigureTWCCHeaderExtensionSender(mediaEngine *MediaEngine, interceptorRegistry *interceptor.Registry) error {
	if err := mediaEngine.RegisterHeaderExtension(
		RTPHeaderExtensionCapability{URI: sdp.TransportCCURI}, RTPCodecTypeVideo,
	); err != nil {
		return err
	}

	if err := mediaEngine.RegisterHeaderExtension(
		RTPHeaderExtensionCapability{URI: sdp.TransportCCURI}, RTPCodecTypeAudio,
	); err != nil {
		return err
	}

	i, err := twcc.NewHeaderExtensionInterceptor()
	if err != nil {
		return err
	}

	interceptorRegistry.Add(i)

	return nil
}

// ConfigureTWCCSender will setup everything necessary for generating TWCC reports.
// This must be called after registering codecs with the MediaEngine.
func ConfigureTWCCSender(mediaEngine *MediaEngine, interceptorRegistry *interceptor.Registry) error {
	mediaEngine.RegisterFeedback(RTCPFeedback{Type: TypeRTCPFBTransportCC}, RTPCodecTypeVideo)
	if err := mediaEngine.RegisterHeaderExtension(
		RTPHeaderExtensionCapability{URI: sdp.TransportCCURI}, RTPCodecTypeVideo,
	); err != nil {
		return err
	}

	mediaEngine.RegisterFeedback(RTCPFeedback{Type: TypeRTCPFBTransportCC}, RTPCodecTypeAudio)
	if err := mediaEngine.RegisterHeaderExtension(
		RTPHeaderExtensionCapability{URI: sdp.TransportCCURI}, RTPCodecTypeAudio,
	); err != nil {
		return err
	}

	generator, err := twcc.NewSenderInterceptor()
	if err != nil {
		return err
	}

	interceptorRegistry.Add(generator)

	return nil
}

// ConfigureCongestionControlFeedback registers congestion control feedback as
// defined in RFC 8888 (https://datatracker.ietf.org/doc/rfc8888/)
func ConfigureCongestionControlFeedback(mediaEngine *MediaEngine, interceptorRegistry *interceptor.Registry) error {
	mediaEngine.RegisterFeedback(RTCPFeedback{Type: TypeRTCPFBACK, Parameter: "ccfb"}, RTPCodecTypeVideo)
	mediaEngine.RegisterFeedback(RTCPFeedback{Type: TypeRTCPFBACK, Parameter: "ccfb"}, RTPCodecTypeAudio)
	generator, err := rfc8888.NewSenderInterceptor()
	if err != nil {
		return err
	}
	interceptorRegistry.Add(generator)

	return nil
}

// ConfigureSimulcastExtensionHeaders enables the RTP Extension Headers needed for Simulcast.
func ConfigureSimulcastExtensionHeaders(mediaEngine *MediaEngine) error {
	if err := mediaEngine.RegisterHeaderExtension(
		RTPHeaderExtensionCapability{URI: sdp.SDESMidURI}, RTPCodecTypeVideo,
	); err != nil {
		return err
	}

	if err := mediaEngine.RegisterHeaderExtension(
		RTPHeaderExtensionCapability{URI: sdp.SDESRTPStreamIDURI}, RTPCodecTypeVideo,
	); err != nil {
		return err
	}

	return mediaEngine.RegisterHeaderExtension(
		RTPHeaderExtensionCapability{URI: sdp.SDESRepairRTPStreamIDURI}, RTPCodecTypeVideo,
	)
}

// ConfigureFlexFEC03 registers flexfec-03 codec with provided payloadType in mediaEngine
// and adds corresponding interceptor to the registry.
// Note that this function should be called before any other interceptor that modifies RTP packets
// (i.e. TWCCHeaderExtensionSender) is added to the registry, so that packets generated by flexfec
// interceptor are not modified.
func ConfigureFlexFEC03(
	payloadType PayloadType,
	mediaEngine *MediaEngine,
	interceptorRegistry *interceptor.Registry,
	options ...flexfec.FecOption,
) error {
	codecFEC := RTPCodecParameters{
		RTPCodecCapability: RTPCodecCapability{
			MimeType:     MimeTypeFlexFEC03,
			ClockRate:    90000,
			SDPFmtpLine:  "repair-window=10000000",
			RTCPFeedback: nil,
		},
		PayloadType: payloadType,
	}

	if err := mediaEngine.RegisterCodec(codecFEC, RTPCodecTypeVideo); err != nil {
		return err
	}

	generator, err := flexfec.NewFecInterceptor(options...)
	if err != nil {
		return err
	}

	interceptorRegistry.Add(generator)

	return nil
}

type interceptorToTrackLocalWriter struct{ interceptor atomic.Value } // interceptor.RTPWriter }

func (i *interceptorToTrackLocalWriter) WriteRTP(header *rtp.Header, payload []byte) (int, error) {
	if writer, ok := i.interceptor.Load().(interceptor.RTPWriter); ok && writer != nil {
		return writer.Write(header, payload, interceptor.Attributes{})
	}

	return 0, nil
}

func (i *interceptorToTrackLocalWriter) Write(b []byte) (int, error) {
	packet := &rtp.Packet{}
	if err := packet.Unmarshal(b); err != nil {
		return 0, err
	}

	return i.WriteRTP(&packet.Header, packet.Payload)
}

//nolint:unparam
func createStreamInfo(
	id string,
	ssrc, ssrcRTX, ssrcFEC SSRC,
	payloadType, payloadTypeRTX, payloadTypeFEC PayloadType,
	codec RTPCodecCapability,
	webrtcHeaderExtensions []RTPHeaderExtensionParameter,
) *interceptor.StreamInfo {
	headerExtensions := make([]interceptor.RTPHeaderExtension, 0, len(webrtcHeaderExtensions))
	for _, h := range webrtcHeaderExtensions {
		headerExtensions = append(headerExtensions, interceptor.RTPHeaderExtension{ID: h.ID, URI: h.URI})
	}

	feedbacks := make([]interceptor.RTCPFeedback, 0, len(codec.RTCPFeedback))
	for _, f := range codec.RTCPFeedback {
		feedbacks = append(feedbacks, interceptor.RTCPFeedback{Type: f.Type, Parameter: f.Parameter})
	}

	return &interceptor.StreamInfo{
		ID:                                id,
		Attributes:                        interceptor.Attributes{},
		SSRC:                              uint32(ssrc),
		SSRCRetransmission:                uint32(ssrcRTX),
		SSRCForwardErrorCorrection:        uint32(ssrcFEC),
		PayloadType:                       uint8(payloadType),
		PayloadTypeRetransmission:         uint8(payloadTypeRTX),
		PayloadTypeForwardErrorCorrection: uint8(payloadTypeFEC),
		RTPHeaderExtensions:               headerExtensions,
		MimeType:                          codec.MimeType,
		ClockRate:                         codec.ClockRate,
		Channels:                          codec.Channels,
		SDPFmtpLine:                       codec.SDPFmtpLine,
		RTCPFeedback:                      feedbacks,
	}
}
