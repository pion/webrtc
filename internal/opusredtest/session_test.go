//go:build opusred && !js

// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package opusredtest

import (
	"io"
	"testing"

	"github.com/pion/interceptor"
	"github.com/pion/rtp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testRegistry(session *Session) *interceptor.Registry {
	registry := &interceptor.Registry{}
	registry.Add(session.WireFactory())
	registry.Add(session.SenderFactory())
	registry.Add(session.ReceiverFactory())
	registry.Add(session.SourceFactory())
	return registry
}

func TestSessionAdaptersPreserveOriginalMetadataAndEvidence(t *testing.T) {
	session, err := NewSession(1200)
	require.NoError(t, err)
	chain, err := testRegistry(session).Build("trial")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, chain.Close()) })
	mapping := testMapping()
	mapping.OpusFMTP = "minptime=10;useinbandfec=1"
	require.NoError(t, session.Arm(mapping))
	info := &interceptor.StreamInfo{
		ID: "audio", SSRC: 42, MimeType: "audio/red", PayloadType: 63, ClockRate: 48000, Channels: 2, SDPFmtpLine: "111/111",
		RTPHeaderExtensions: []interceptor.RTPHeaderExtension{{ID: 1, URI: "test"}},
		RTCPFeedback:        []interceptor.RTCPFeedback{{Type: "transport-cc"}},
		Attributes:          interceptor.Attributes{"original": true},
	}
	originalMetadata := metadata(info)
	var packets []rtp.Packet
	sink := interceptor.RTPWriterFunc(func(header *rtp.Header, payload []byte, _ interceptor.Attributes) (int, error) {
		packets = append(packets, rtp.Packet{Header: header.Clone(), Payload: append([]byte(nil), payload...)})
		return NewPacketRecord(header, payload).RTPSize, nil
	})
	writer := chain.BindLocalStream(info, sink)
	payload := []byte{0xf8, 0xff, 0xfe}
	expectedDigest := payloadHash(payload)
	for index := range 2 {
		header := &rtp.Header{Version: 2, PayloadType: 111, SSRC: 42, SequenceNumber: uint16(10 + index), Timestamp: uint32(960 * (index + 1))}
		_, err := writer.Write(header, payload, interceptor.Attributes{})
		require.NoError(t, err)
		assert.Equal(t, uint8(111), header.PayloadType)
	}
	payload[0] = 0
	assert.Equal(t, originalMetadata, metadata(info))
	require.Len(t, packets, 2)
	assert.Equal(t, uint8(111), packets[0].PayloadType)
	assert.Equal(t, uint8(63), packets[1].PayloadType)
	reader := chain.BindRemoteStream(info, interceptor.RTPReaderFunc(func(buffer []byte, attributes interceptor.Attributes) (int, interceptor.Attributes, error) {
		if len(packets) == 0 {
			return 0, attributes, io.EOF
		}
		raw, err := packets[0].Marshal()
		if err != nil {
			return 0, attributes, err
		}
		packets = packets[1:]
		return copy(buffer, raw), attributes, nil
	}))
	for index := range 2 {
		buffer := make([]byte, 1500)
		n, _, err := reader.Read(buffer, nil)
		require.NoError(t, err)
		var output rtp.Packet
		require.NoError(t, output.Unmarshal(buffer[:n]))
		assert.Equal(t, uint16(10+index), output.SequenceNumber)
		assert.Equal(t, uint8(111), output.PayloadType)
		assert.Equal(t, expectedDigest, payloadHash(output.Payload))
	}
	evidence := session.Snapshot()
	require.Empty(t, evidence.Faults)
	require.Len(t, evidence.Original, 2)
	require.Len(t, evidence.Sent, 2)
	require.Len(t, evidence.Received, 2)
	assert.Equal(t, evidence.Sent, evidence.Received)
	assert.Equal(t, expectedDigest, evidence.Original[0].PayloadSHA256)
	require.Len(t, evidence.Sent[1].Blocks, 1)
	assert.Equal(t, expectedDigest, evidence.Sent[1].Blocks[0].PayloadSHA256)
	require.Len(t, evidence.Bindings, 8)
	for _, binding := range evidence.Bindings {
		assert.Equal(t, originalMetadata, binding.Original)
		assert.Equal(t, uint32(42), binding.Adapted.SSRC)
		if binding.Factory == "sender" || binding.Factory == "receiver" {
			assert.Equal(t, uint8(111), binding.Adapted.PayloadType)
			assert.Equal(t, uint8(63), binding.Adapted.PayloadTypeForwardErrorCorrection)
			assert.Equal(t, "audio/opus", binding.Adapted.MimeType)
			assert.Equal(t, mapping.OpusFMTP, binding.Adapted.SDPFmtpLine)
		}
	}
	evidence.Original[0].SSRC = 999
	evidence.Sent[1].Blocks[0].PayloadSHA256 = "changed"
	evidence.Bindings[0].Original.RTPHeaderExtensions[0].URI = "changed"
	current := session.Snapshot()
	assert.Equal(t, uint32(42), current.Original[0].SSRC)
	assert.Equal(t, expectedDigest, current.Sent[1].Blocks[0].PayloadSHA256)
	assert.Equal(t, "test", current.Bindings[0].Original.RTPHeaderExtensions[0].URI)
}

func TestSessionRejectsUnarmedOrNilAudioBinding(t *testing.T) {
	session, err := NewSession(1200)
	require.NoError(t, err)
	instance, err := session.WireFactory().NewInterceptor("trial")
	require.NoError(t, err)
	sink := interceptor.RTPWriterFunc(func(*rtp.Header, []byte, interceptor.Attributes) (int, error) { return 0, nil })
	writer := instance.BindLocalStream(&interceptor.StreamInfo{MimeType: "audio/opus"}, sink)
	_, err = writer.Write(&rtp.Header{}, nil, nil)
	require.Error(t, err)
	require.Error(t, session.Arm(testMapping()))
	reader := instance.BindRemoteStream(nil, interceptor.RTPReaderFunc(func([]byte, interceptor.Attributes) (int, interceptor.Attributes, error) { return 0, nil, io.EOF }))
	_, _, err = reader.Read(make([]byte, 1500), nil)
	require.Error(t, err)
	assert.Len(t, session.Snapshot().Faults, 2)
}

func TestSessionMappingIsImmutable(t *testing.T) {
	_, err := NewSession(0)
	require.Error(t, err)
	session, err := NewSession(1200)
	require.NoError(t, err)
	require.NoError(t, session.Arm(testMapping()))
	require.Error(t, session.Arm(testMapping()))
}
