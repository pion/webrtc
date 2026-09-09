// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js

package webrtc

import (
	"regexp"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/pion/interceptor"
	mock_interceptor "github.com/pion/interceptor/pkg/mock"
	"github.com/pion/interceptor/pkg/red"
	"github.com/pion/logging"
	"github.com/pion/rtp"
	"github.com/pion/sdp/v3"
	"github.com/pion/transport/v4/test"
	"github.com/pion/transport/v4/vnet"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testOpusREDOpusPayloadType = PayloadType(111)
	testOpusREDPayloadType     = PayloadType(63)
)

type opusREDObservation struct {
	mu sync.Mutex

	wirePayloadTypes  []uint8
	localStreamInfos  []interceptor.StreamInfo
	remoteStreamInfos []interceptor.StreamInfo
}

func (o *opusREDObservation) factory() interceptor.Factory {
	return &mock_interceptor.Factory{
		NewInterceptorFn: func(string) (interceptor.Interceptor, error) {
			return &mock_interceptor.Interceptor{
				BindLocalStreamFn: func(
					info *interceptor.StreamInfo,
					writer interceptor.RTPWriter,
				) interceptor.RTPWriter {
					o.mu.Lock()
					o.localStreamInfos = append(o.localStreamInfos, *info)
					o.mu.Unlock()

					return interceptor.RTPWriterFunc(func(
						header *rtp.Header,
						payload []byte,
						attributes interceptor.Attributes,
					) (int, error) {
						o.mu.Lock()
						o.wirePayloadTypes = append(o.wirePayloadTypes, header.PayloadType)
						o.mu.Unlock()

						return writer.Write(header, payload, attributes)
					})
				},
				BindRemoteStreamFn: func(
					info *interceptor.StreamInfo,
					reader interceptor.RTPReader,
				) interceptor.RTPReader {
					o.mu.Lock()
					o.remoteStreamInfos = append(o.remoteStreamInfos, *info)
					o.mu.Unlock()

					return interceptor.RTPReaderFunc(func(
						buffer []byte,
						attributes interceptor.Attributes,
					) (int, interceptor.Attributes, error) {
						n, readAttributes, err := reader.Read(buffer, attributes)
						if err == nil {
							var header rtp.Header
							if _, headerErr := header.Unmarshal(buffer[:n]); headerErr == nil {
								o.mu.Lock()
								o.wirePayloadTypes = append(o.wirePayloadTypes, header.PayloadType)
								o.mu.Unlock()
							}
						}

						return n, readAttributes, err
					})
				},
			}, nil
		},
	}
}

func (o *opusREDObservation) snapshot() (
	[]uint8, []interceptor.StreamInfo, []interceptor.StreamInfo,
) {
	o.mu.Lock()
	defer o.mu.Unlock()

	return slices.Clone(o.wirePayloadTypes), slices.Clone(o.localStreamInfos), slices.Clone(o.remoteStreamInfos)
}

func newOpusREDAPIOptions(
	t *testing.T,
	settingEngine SettingEngine,
	observer interceptor.Factory,
	withAudioStreamExtensions bool,
) []func(*API) {
	t.Helper()

	mediaEngine := &MediaEngine{}
	require.NoError(t, mediaEngine.RegisterDefaultCodecs())
	if withAudioStreamExtensions {
		require.NoError(t, mediaEngine.RegisterHeaderExtension(
			RTPHeaderExtensionCapability{URI: sdp.SDESMidURI},
			RTPCodecTypeAudio,
		))
		require.NoError(t, mediaEngine.RegisterHeaderExtension(
			RTPHeaderExtensionCapability{URI: sdp.SDESRTPStreamIDURI},
			RTPCodecTypeAudio,
		))
	}

	registry := &interceptor.Registry{}
	if observer != nil {
		registry.Add(observer)
	}
	require.NoError(t, ConfigureOpusRED(
		testOpusREDOpusPayloadType,
		testOpusREDPayloadType,
		mediaEngine,
		registry,
	))

	return []func(*API){
		WithMediaEngine(mediaEngine),
		WithInterceptorRegistry(registry),
		WithSettingEngine(settingEngine),
	}
}

func createOpusREDVNetPair(
	t *testing.T,
	observer interceptor.Factory,
	withAudioStreamExtensions bool,
) (*PeerConnection, *PeerConnection, *vnet.Router) {
	t.Helper()

	wan, err := vnet.NewRouter(&vnet.RouterConfig{
		CIDR:          "1.2.3.0/24",
		LoggerFactory: logging.NewDefaultLoggerFactory(),
	})
	require.NoError(t, err)

	offerNet, err := vnet.NewNet(&vnet.NetConfig{StaticIPs: []string{"1.2.3.4"}})
	require.NoError(t, err)
	require.NoError(t, wan.AddNet(offerNet))
	answerNet, err := vnet.NewNet(&vnet.NetConfig{StaticIPs: []string{"1.2.3.5"}})
	require.NoError(t, err)
	require.NoError(t, wan.AddNet(answerNet))
	require.NoError(t, wan.Start())

	offerSettingEngine := SettingEngine{}
	offerSettingEngine.SetNet(offerNet)
	offerSettingEngine.SetICETimeouts(time.Second, time.Second, 200*time.Millisecond)
	answerSettingEngine := SettingEngine{}
	answerSettingEngine.SetNet(answerNet)
	answerSettingEngine.SetICETimeouts(time.Second, time.Second, 200*time.Millisecond)

	offerPeer, err := NewAPI(newOpusREDAPIOptions(
		t, offerSettingEngine, observer, withAudioStreamExtensions,
	)...).NewPeerConnection(Configuration{})
	require.NoError(t, err)
	answerPeer, err := NewAPI(newOpusREDAPIOptions(
		t, answerSettingEngine, observer, withAudioStreamExtensions,
	)...).NewPeerConnection(Configuration{})
	require.NoError(t, err)

	return offerPeer, answerPeer, wan
}

type opusREDReadResult struct {
	packets     []*rtp.Packet
	codec       RTPCodecParameters
	payloadType PayloadType
	rid         string
	err         error
}

func readOpusREDPackets(track *TrackRemote, count int) opusREDReadResult {
	result := opusREDReadResult{
		packets: make([]*rtp.Packet, 0, count),
		rid:     track.RID(),
	}
	for range count {
		packet, _, err := track.ReadRTP()
		if err != nil {
			result.err = err

			return result
		}
		result.packets = append(result.packets, packet)
	}
	result.codec = track.Codec()
	result.payloadType = track.PayloadType()

	return result
}

func assertOpusREDStreamInfo(t *testing.T, infos []interceptor.StreamInfo, ssrc uint32) {
	t.Helper()
	for _, info := range infos {
		if info.SSRC != ssrc || info.MimeType != MimeTypeOpus {
			continue
		}

		assert.Equal(t, uint8(testOpusREDOpusPayloadType), info.PayloadType)
		assert.Equal(t, uint8(testOpusREDPayloadType), info.PayloadTypeForwardErrorCorrection)
		assert.Zero(t, info.SSRCForwardErrorCorrection)

		return
	}
	assert.Failf(t, "missing Opus RED StreamInfo", "no StreamInfo found for SSRC %d", ssrc)
}

func TestOpusREDTransparentRecoveryAndInterceptorOrdering(t *testing.T) {
	defer test.TimeOut(20 * time.Second).Stop()

	observation := &opusREDObservation{}
	offerPeer, answerPeer, wan := createOpusREDVNetPair(t, observation.factory(), false)
	closed := false
	t.Cleanup(func() {
		if !closed {
			closePairNow(t, offerPeer, answerPeer)
			assert.NoError(t, wan.Stop())
		}
	})

	track, err := NewTrackLocalStaticRTP(
		RTPCodecCapability{MimeType: MimeTypeOpus, ClockRate: 48000, Channels: 2},
		"audio",
		"pion",
	)
	require.NoError(t, err)
	sender, err := offerPeer.AddTrack(track)
	require.NoError(t, err)
	resultChannel := make(chan opusREDReadResult, 1)
	answerPeer.OnTrack(func(remote *TrackRemote, _ *RTPReceiver) {
		resultChannel <- readOpusREDPackets(remote, 3)
	})

	connected := untilConnectionState(PeerConnectionStateConnected, offerPeer, answerPeer)
	require.NoError(t, signalPair(offerPeer, answerPeer))
	connected.Wait()

	ssrc := uint32(sender.GetParameters().Encodings[0].SSRC)
	var wireMutex sync.Mutex
	wirePayloadTypes := []uint8{}
	mediaPacketCount := 0
	wan.AddChunkFilter(func(chunk vnet.Chunk) bool {
		var header rtp.Header
		if _, parseErr := header.Unmarshal(chunk.UserData()); parseErr != nil || header.SSRC != ssrc {
			return true
		}
		if header.PayloadType != uint8(testOpusREDOpusPayloadType) &&
			header.PayloadType != uint8(testOpusREDPayloadType) {
			return true
		}

		wireMutex.Lock()
		defer wireMutex.Unlock()
		wirePayloadTypes = append(wirePayloadTypes, header.PayloadType)
		mediaPacketCount++

		return mediaPacketCount != 2
	})

	for i := range 3 {
		require.NoError(t, track.WriteRTP(&rtp.Packet{
			Header: rtp.Header{
				Version:        2,
				SequenceNumber: uint16(100 + i),       //nolint:gosec // Test input is bounded.
				Timestamp:      uint32(960 * (i + 1)), //nolint:gosec // Test input is bounded.
			},
			Payload: []byte{byte(i + 1)}, //nolint:gosec // Test input is bounded.
		}))
	}

	var result opusREDReadResult
	select {
	case result = <-resultChannel:
	case <-time.After(5 * time.Second):
		require.Fail(t, "timed out waiting for recovered Opus packets")
	}
	require.NoError(t, result.err)
	require.Len(t, result.packets, 3)
	assert.Equal(t, MimeTypeOpus, result.codec.MimeType)
	assert.Equal(t, testOpusREDOpusPayloadType, result.payloadType)
	for i, packet := range result.packets {
		assert.Equal(t, uint8(testOpusREDOpusPayloadType), packet.PayloadType)
		assert.Equal(t, uint16(100+i), packet.SequenceNumber) //nolint:gosec // Test input is bounded.
		assert.Equal(t, []byte{byte(i + 1)}, packet.Payload)  //nolint:gosec // Test input is bounded.
	}

	wireMutex.Lock()
	assert.Equal(t, []uint8{
		uint8(testOpusREDOpusPayloadType),
		uint8(testOpusREDPayloadType),
		uint8(testOpusREDPayloadType),
	}, wirePayloadTypes)
	wireMutex.Unlock()
	observedPayloadTypes, localStreamInfos, remoteStreamInfos := observation.snapshot()
	assert.Contains(t, observedPayloadTypes, uint8(testOpusREDPayloadType))
	assertOpusREDStreamInfo(t, localStreamInfos, ssrc)
	assertOpusREDStreamInfo(t, remoteStreamInfos, ssrc)

	closePairNow(t, offerPeer, answerPeer)
	require.NoError(t, wan.Stop())
	closed = true
}

func TestOpusREDUndeclaredRIDFirstPacket(t *testing.T) { //nolint:cyclop
	defer test.TimeOut(20 * time.Second).Stop()

	offerPeer, answerPeer, wan := createOpusREDVNetPair(t, nil, true)
	closed := false
	t.Cleanup(func() {
		if !closed {
			closePairNow(t, offerPeer, answerPeer)
			assert.NoError(t, wan.Stop())
		}
	})

	trackA, err := NewTrackLocalStaticRTP(
		RTPCodecCapability{MimeType: MimeTypeOpus, ClockRate: 48000, Channels: 2},
		"audio",
		"pion",
		WithRTPStreamID("a"),
	)
	require.NoError(t, err)
	trackB, err := NewTrackLocalStaticRTP(
		RTPCodecCapability{MimeType: MimeTypeOpus, ClockRate: 48000, Channels: 2},
		"audio",
		"pion",
		WithRTPStreamID("b"),
	)
	require.NoError(t, err)
	sender, err := offerPeer.AddTrack(trackA)
	require.NoError(t, err)
	require.NoError(t, sender.AddEncoding(trackB))

	resultChannel := make(chan opusREDReadResult, 1)
	answerPeer.OnTrack(func(remote *TrackRemote, _ *RTPReceiver) {
		if remote.RID() == trackA.RID() {
			resultChannel <- readOpusREDPackets(remote, 2)
		}
	})

	connected := untilConnectionState(PeerConnectionStateConnected, offerPeer, answerPeer)
	stripSSRC := regexp.MustCompile("(?m)[\r\n]+^.*a=ssrc.*$")
	require.NoError(t, signalPairWithModification(offerPeer, answerPeer, func(raw string) string {
		return stripSSRC.ReplaceAllString(raw, "")
	}))
	connected.Wait()

	var midID, ridID uint8
	for _, extension := range sender.GetParameters().HeaderExtensions {
		switch extension.URI {
		case sdp.SDESMidURI:
			midID = uint8(extension.ID) //nolint:gosec // RTP extension IDs are bounded.
		case sdp.SDESRTPStreamIDURI:
			ridID = uint8(extension.ID) //nolint:gosec // RTP extension IDs are bounded.
		}
	}
	require.NotZero(t, midID)
	require.NotZero(t, ridID)

	ssrc := uint32(sender.GetParameters().Encodings[0].SSRC)
	mediaPacketCount := 0
	wan.AddChunkFilter(func(chunk vnet.Chunk) bool {
		var header rtp.Header
		if _, parseErr := header.Unmarshal(chunk.UserData()); parseErr != nil || header.SSRC != ssrc {
			return true
		}
		if header.PayloadType != uint8(testOpusREDOpusPayloadType) &&
			header.PayloadType != uint8(testOpusREDPayloadType) {
			return true
		}
		mediaPacketCount++

		return mediaPacketCount != 1
	})

	for i := range 2 {
		packet := &rtp.Packet{
			Header: rtp.Header{
				Version:        2,
				SequenceNumber: uint16(200 + i),       //nolint:gosec // Test input is bounded.
				Timestamp:      uint32(960 * (i + 1)), //nolint:gosec // Test input is bounded.
			},
			Payload: []byte{byte(i + 1)}, //nolint:gosec // Test input is bounded.
		}
		require.NoError(t, packet.SetExtension(midID, []byte(sender.rtpTransceiver.Mid())))
		require.NoError(t, packet.SetExtension(ridID, []byte(trackA.RID())))
		require.NoError(t, trackA.WriteRTP(packet))
	}

	var result opusREDReadResult
	select {
	case result = <-resultChannel:
	case <-time.After(5 * time.Second):
		require.Fail(t, "timed out waiting for undeclared RED stream")
	}
	require.NoError(t, result.err)
	assert.Equal(t, "a", result.rid)
	assert.Equal(t, MimeTypeOpus, result.codec.MimeType)
	assert.Equal(t, testOpusREDOpusPayloadType, result.payloadType)
	require.Len(t, result.packets, 2)
	assert.Equal(t, []uint16{200, 201}, []uint16{
		result.packets[0].SequenceNumber,
		result.packets[1].SequenceNumber,
	})
	for _, packet := range result.packets {
		assert.Equal(t, uint8(testOpusREDOpusPayloadType), packet.PayloadType)
	}

	closePairNow(t, offerPeer, answerPeer)
	require.NoError(t, wan.Stop())
	closed = true
}

func ExampleConfigureOpusRED() {
	mediaEngine := &MediaEngine{}
	_ = mediaEngine.RegisterDefaultCodecs()
	registry := &interceptor.Registry{}

	// Add reports, stats, NACK, and TWCC feedback before RED so they see wire packets.
	_ = RegisterDefaultInterceptors(mediaEngine, registry)
	_ = ConfigureOpusRED(111, 63, mediaEngine, registry, red.SenderMaxPacketSize(1200))
	// Header mutators belong outside RED and are therefore registered afterward.
	_ = ConfigureTWCCHeaderExtensionSender(mediaEngine, registry)

	_ = NewAPI(WithMediaEngine(mediaEngine), WithInterceptorRegistry(registry))
}
