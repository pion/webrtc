// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js

package webrtc

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pion/interceptor"
	"github.com/pion/interceptor/pkg/flexfec"
	"github.com/pion/interceptor/pkg/nack"
	"github.com/pion/interceptor/pkg/red"
	"github.com/pion/interceptor/pkg/report"
	"github.com/pion/interceptor/pkg/rfc8888"
	"github.com/pion/interceptor/pkg/stats"
	"github.com/pion/interceptor/pkg/twcc"
	"github.com/pion/rtp"
	"github.com/pion/sdp/v3"
)

var (
	errOpusREDNilMediaEngine         = errors.New("media engine must not be nil")
	errOpusREDNilInterceptorRegistry = errors.New("interceptor registry must not be nil")
	errOpusREDPayloadTypeOutOfRange  = errors.New("opus RED payload types must fit in seven bits")
	errOpusREDPayloadTypeZero        = errors.New("opus RED payload type must not be zero")
	errOpusREDPayloadTypeCollision   = errors.New("opus and RED payload types must be distinct")
)

// RegisterDefaultInterceptors will register some useful interceptors.
// If you want to customize which interceptors are loaded, you should copy the code from this method and remove
// unwanted interceptors. You can also use RegisterDefaultInterceptorsWithOptions to pass in options to modify behavior.
func RegisterDefaultInterceptors(mediaEngine *MediaEngine, interceptorRegistry *interceptor.Registry) error {
	return RegisterDefaultInterceptorsWithOptions(mediaEngine, interceptorRegistry)
}

// RegisterDefaultInterceptorsWithOptions will register some useful interceptors with the provided options.
// If you want to customize which interceptors are loaded, you should copy the code from this method and remove
// unwanted interceptors, or pass in options to modify behavior.
func RegisterDefaultInterceptorsWithOptions(mediaEngine *MediaEngine, interceptorRegistry *interceptor.Registry,
	opts ...InterceptorOption,
) error {
	var options interceptorOptions
	for _, opt := range opts {
		opt(&options)
	}

	if options.loggerFactory != nil {
		// Set logger factory for all interceptors
		options.nackGeneratorOptions = append(options.nackGeneratorOptions,
			nack.WithGeneratorLoggerFactory(options.loggerFactory))
		options.nackResponderOptions = append(options.nackResponderOptions,
			nack.WithResponderLoggerFactory(options.loggerFactory))
		options.reportReceiverOptions = append(options.reportReceiverOptions,
			report.WithReceiverLoggerFactory(options.loggerFactory))
		options.reportSenderOptions = append(options.reportSenderOptions,
			report.WithSenderLoggerFactory(options.loggerFactory))
		options.statsOptions = append(options.statsOptions, stats.WithLoggerFactory(options.loggerFactory))
		options.twccOptions = append(options.twccOptions, twcc.WithLoggerFactory(options.loggerFactory))
	}

	if err := ConfigureNackWithOptions(mediaEngine, interceptorRegistry, options.nackGeneratorOptions,
		options.nackResponderOptions...); err != nil {
		return err
	}

	if err := ConfigureRTCPReportsWithOptions(interceptorRegistry, options.reportReceiverOptions,
		options.reportSenderOptions...); err != nil {
		return err
	}

	if err := ConfigureSimulcastExtensionHeaders(mediaEngine); err != nil {
		return err
	}

	if err := ConfigureStatsInterceptorWithOptions(interceptorRegistry, options.statsOptions...); err != nil {
		return err
	}

	return ConfigureTWCCSenderWithOptions(mediaEngine, interceptorRegistry, options.twccOptions...)
}

// ConfigureStatsInterceptor will setup everything necessary for generating RTP stream statistics.
func ConfigureStatsInterceptor(interceptorRegistry *interceptor.Registry) error {
	return ConfigureStatsInterceptorWithOptions(interceptorRegistry)
}

// ConfigureStatsInterceptorWithOptions will setup everything necessary for generating RTP stream statistics
// with the provided options.
func ConfigureStatsInterceptorWithOptions(interceptorRegistry *interceptor.Registry, opts ...stats.Option) error {
	statsInterceptor, err := stats.NewInterceptor(opts...)
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

// cleanupStats removes the stats getter for a given peerconnection.statsId.
func cleanupStats(id string) {
	statsGetter.Delete(id)
}

// key: string (peerconnection.statsId), value: stats.Getter
var statsGetter sync.Map // nolint:gochecknoglobals

// ConfigureRTCPReports will setup everything necessary for generating Sender and Receiver Reports.
func ConfigureRTCPReports(interceptorRegistry *interceptor.Registry) error {
	return ConfigureRTCPReportsWithOptions(interceptorRegistry, nil)
}

// ConfigureRTCPReportsWithOptions will setup everything necessary for generating Sender and Receiver Reports
// with the provided options.
func ConfigureRTCPReportsWithOptions(interceptorRegistry *interceptor.Registry, recvOpts []report.ReceiverOption,
	sendOpts ...report.SenderOption,
) error {
	receiver, err := report.NewReceiverInterceptor(recvOpts...)
	if err != nil {
		return err
	}

	sender, err := report.NewSenderInterceptor(sendOpts...)
	if err != nil {
		return err
	}

	interceptorRegistry.Add(receiver)
	interceptorRegistry.Add(sender)

	return nil
}

// ConfigureNack will setup everything necessary for handling generating/responding to nack messages.
func ConfigureNack(mediaEngine *MediaEngine, interceptorRegistry *interceptor.Registry) error {
	return ConfigureNackWithOptions(mediaEngine, interceptorRegistry, nil)
}

// ConfigureNackWithOptions will setup everything necessary for handling generating/responding to nack messages
// with the provided options.
func ConfigureNackWithOptions(mediaEngine *MediaEngine, interceptorRegistry *interceptor.Registry,
	genOpts []nack.GeneratorOption, respOpts ...nack.ResponderOption,
) error {
	generator, err := nack.NewGeneratorInterceptor(genOpts...)
	if err != nil {
		return err
	}

	responder, err := nack.NewResponderInterceptor(respOpts...)
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

	twccInterceptor, err := twcc.NewHeaderExtensionInterceptor()
	if err != nil {
		return err
	}

	interceptorRegistry.Add(twccInterceptor)

	return nil
}

// ConfigureTWCCSender will setup everything necessary for generating TWCC reports.
// This must be called after registering codecs with the MediaEngine.
func ConfigureTWCCSender(mediaEngine *MediaEngine, interceptorRegistry *interceptor.Registry) error {
	return ConfigureTWCCSenderWithOptions(mediaEngine, interceptorRegistry)
}

// ConfigureTWCCSenderWithOptions will setup everything necessary for generating TWCC reports with the provided options.
// This must be called after registering codecs with the MediaEngine.
func ConfigureTWCCSenderWithOptions(mediaEngine *MediaEngine, interceptorRegistry *interceptor.Registry,
	opts ...twcc.Option,
) error {
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

	generator, err := twcc.NewSenderInterceptor(opts...)
	if err != nil {
		return err
	}

	interceptorRegistry.Add(generator)

	return nil
}

// ConfigureCongestionControlFeedback registers congestion control feedback as
// defined in RFC 8888 (https://datatracker.ietf.org/doc/rfc8888/)
func ConfigureCongestionControlFeedback(mediaEngine *MediaEngine, interceptorRegistry *interceptor.Registry) error {
	return ConfigureCongestionControlFeedbackWithOptions(mediaEngine, interceptorRegistry)
}

// ConfigureCongestionControlFeedbackWithOptions registers congestion control feedback as
// defined in RFC 8888 (https://datatracker.ietf.org/doc/rfc8888/) with the provided options.
func ConfigureCongestionControlFeedbackWithOptions(mediaEngine *MediaEngine, interceptorRegistry *interceptor.Registry,
	opts ...rfc8888.Option,
) error {
	mediaEngine.RegisterFeedback(RTCPFeedback{Type: TypeRTCPFBACK, Parameter: "ccfb"}, RTPCodecTypeVideo)
	mediaEngine.RegisterFeedback(RTCPFeedback{Type: TypeRTCPFBACK, Parameter: "ccfb"}, RTPCodecTypeAudio)
	generator, err := rfc8888.NewSenderInterceptor(opts...)
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

func validateOpusREDConfiguration(
	opusPayloadType, redPayloadType PayloadType,
	mediaEngine *MediaEngine,
	interceptorRegistry *interceptor.Registry,
) error {
	switch {
	case mediaEngine == nil:
		return errOpusREDNilMediaEngine
	case interceptorRegistry == nil:
		return errOpusREDNilInterceptorRegistry
	case opusPayloadType > 0x7f || redPayloadType > 0x7f:
		return errOpusREDPayloadTypeOutOfRange
	case redPayloadType == 0:
		return errOpusREDPayloadTypeZero
	case opusPayloadType == redPayloadType:
		return errOpusREDPayloadTypeCollision
	default:
		return nil
	}
}

func newOpusREDInterceptorFactories(
	options []red.SenderOption,
) (*red.SenderInterceptorFactory, *red.ReceiverInterceptorFactory, error) {
	sender, err := red.NewSenderInterceptor(options...)
	if err != nil {
		return nil, nil, err
	}
	configuredSender, err := sender.NewInterceptor("")
	if err != nil {
		return nil, nil, err
	}
	if err = configuredSender.Close(); err != nil {
		return nil, nil, err
	}
	receiver, err := red.NewReceiverInterceptor()
	if err != nil {
		return nil, nil, err
	}

	return sender, receiver, nil
}

func registerOpusREDCodec(mediaEngine *MediaEngine, opusPayloadType, redPayloadType PayloadType) error {
	mediaEngine.mu.Lock()
	defer mediaEngine.mu.Unlock()

	opusIndex := -1
	for i, codec := range mediaEngine.audioCodecs {
		if codec.PayloadType == redPayloadType {
			return ErrCodecAlreadyRegistered
		}
		if codec.PayloadType == opusPayloadType && strings.EqualFold(codec.MimeType, MimeTypeOpus) {
			opusIndex = i
		}
	}
	if opusIndex == -1 {
		return ErrCodecNotFound
	}

	redCodec := RTPCodecParameters{
		RTPCodecCapability: RTPCodecCapability{
			MimeType:    MimeTypeRED,
			ClockRate:   48000,
			Channels:    2,
			SDPFmtpLine: fmt.Sprintf("%d/%d", opusPayloadType, opusPayloadType),
		},
		PayloadType: redPayloadType,
		statsID:     fmt.Sprintf("RTPCodec-%d", time.Now().UnixNano()),
	}
	mediaEngine.audioCodecs = append(mediaEngine.audioCodecs, RTPCodecParameters{})
	copy(mediaEngine.audioCodecs[opusIndex+1:], mediaEngine.audioCodecs[opusIndex:])
	mediaEngine.audioCodecs[opusIndex] = redCodec

	return nil
}

// ConfigureOpusRED registers an audio/red codec associated with an existing
// Opus codec and installs transparent RFC 2198 sender and receiver interceptors.
//
// It must be called before creating a PeerConnection. Register network-observing
// interceptors, such as reports, stats, NACK, and TWCC feedback, before calling
// ConfigureOpusRED so they observe wire RED packets. Register packet-header
// mutators, such as the TWCC header-extension sender, after ConfigureOpusRED.
// Applications continue to read and write ordinary Opus RTP packets.
func ConfigureOpusRED(
	opusPayloadType PayloadType,
	redPayloadType PayloadType,
	mediaEngine *MediaEngine,
	interceptorRegistry *interceptor.Registry,
	options ...red.SenderOption,
) error {
	if err := validateOpusREDConfiguration(
		opusPayloadType,
		redPayloadType,
		mediaEngine,
		interceptorRegistry,
	); err != nil {
		return err
	}

	sender, receiver, err := newOpusREDInterceptorFactories(options)
	if err != nil {
		return err
	}
	if err = registerOpusREDCodec(mediaEngine, opusPayloadType, redPayloadType); err != nil {
		return err
	}

	interceptorRegistry.Add(sender)
	interceptorRegistry.Add(receiver)

	return nil
}

// interceptorToTrackLocalWriter is an RTPWriter that holds a reference to interceptor.RTPWriter.
type interceptorToTrackLocalWriter struct{ interceptor atomic.Value } // interceptor.RTPWriter }

// WriteRTP writes an RTP packet using the underlying interceptor.RTPWriter.
func (i *interceptorToTrackLocalWriter) WriteRTP(header *rtp.Header, payload []byte) (int, error) {
	if writer, ok := i.interceptor.Load().(interceptor.RTPWriter); ok && writer != nil {
		return writer.Write(header, payload, nil)
	}

	return 0, nil
}

// Write writes a raw RTP packet using the underlying interceptor.RTPWriter.
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
