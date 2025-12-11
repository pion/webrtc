// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package oggreader

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

// buildOggFile generates a valid oggfile that can
// be used for tests.
func buildOggContainer() []byte {
	return []byte{
		0x4f, 0x67, 0x67, 0x53, 0x00, 0x02, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x8e, 0x9b, 0x20, 0xaa, 0x00, 0x00,
		0x00, 0x00, 0x61, 0xee, 0x61, 0x17, 0x01, 0x13, 0x4f, 0x70,
		0x75, 0x73, 0x48, 0x65, 0x61, 0x64, 0x01, 0x02, 0x00, 0x0f,
		0x80, 0xbb, 0x00, 0x00, 0x00, 0x00, 0x00, 0x4f, 0x67, 0x67,
		0x53, 0x00, 0x00, 0xda, 0x93, 0xc2, 0xd9, 0x00, 0x00, 0x00,
		0x00, 0x8e, 0x9b, 0x20, 0xaa, 0x02, 0x00, 0x00, 0x00, 0x49,
		0x97, 0x03, 0x37, 0x01, 0x05, 0x98, 0x36, 0xbe, 0x88, 0x9e,
	}
}

// buildSurroundOggContainerShort has mapping family 1 but omits the mapping table (invalid length).
func buildSurroundOggContainerShort() []byte {
	return []byte{
		0x4f, 0x67, 0x67, 0x53, 0x00, 0x02, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x58, 0x49, 0xac, 0xe2, 0x00, 0x00,
		0x00, 0x00, 0xc1, 0xa8, 0x7d, 0x4e, 0x01, 0x13, 0x4f, 0x70,
		0x75, 0x73, 0x48, 0x65, 0x61, 0x64, 0x01, 0x06, 0x38, 0x01,
		0x80, 0xbb, 0x00, 0x00, 0x00, 0x00, 0x01,
	}
}

// buildUnknownMappingFamilyContainer creates an ID page with an unrecognized channel mapping family.
func buildUnknownMappingFamilyContainer(mappingFamily, channels uint8) []byte {
	payload := []byte{
		0x4f, 0x70, 0x75, 0x73, 0x48, 0x65, 0x61, 0x64, // "OpusHead"
		0x01,       // version
		channels,   // channel count
		0x38, 0x01, // preskip (0x0138)
		0x80, 0xbb, 0x00, 0x00, // sample rate (48000)
		0x00, 0x00, // output gain
		mappingFamily,
	}

	segmentTable := []byte{byte(len(payload))}

	header := []byte{
		0x4f, 0x67, 0x67, 0x53, // "OggS"
		0x00,                                           // version
		0x02,                                           // beginning of stream
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // granule position
		0x00, 0x00, 0x00, 0x00, // bitstream serial number
		0x00, 0x00, 0x00, 0x00, // page sequence number
		0x00, 0x00, 0x00, 0x00, // checksum (ignored with checksum disabled)
		0x01, // page segments
	}

	packet := make([]byte, 0, len(header)+len(segmentTable)+len(payload))
	packet = append(packet, header...)
	packet = append(packet, segmentTable...)
	packet = append(packet, payload...)

	return packet
}

// buildChannelMappingFamilyContainer builds an Opus ID page for mapping families that
// follow the Figure 3 layout (families 1, 2, 3, 255).
func buildChannelMappingFamilyContainer(
	mappingFamily, channels, streamCount, coupledCount uint8,
	mapping []byte,
) []byte {
	payload := []byte{
		0x4f, 0x70, 0x75, 0x73, 0x48, 0x65, 0x61, 0x64, // "OpusHead"
		0x01,       // version
		channels,   // channel count
		0x38, 0x01, // preskip (0x0138)
		0x80, 0xbb, 0x00, 0x00, // sample rate (48000)
		0x00, 0x00, // output gain
		mappingFamily,
		streamCount,
		coupledCount,
	}
	payload = append(payload, mapping...)

	segmentTable := []byte{byte(len(payload))}

	header := []byte{
		0x4f, 0x67, 0x67, 0x53, // "OggS"
		0x00,                                           // version
		0x02,                                           // header type (beginning of stream)
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // granule position
		0x00, 0x00, 0x00, 0x00, // bitstream serial number
		0x00, 0x00, 0x00, 0x00, // page sequence number
		0x00, 0x00, 0x00, 0x00, // checksum (ignored when checksum disabled)
		0x01, // page segments
	}

	packet := make([]byte, 0, len(header)+len(segmentTable)+len(payload))
	packet = append(packet, header...)
	packet = append(packet, segmentTable...)
	packet = append(packet, payload...)

	return packet
}

func TestOggReader_ParseValidHeader(t *testing.T) {
	reader, header, err := NewWith(bytes.NewReader(buildOggContainer()))
	assert.NoError(t, err)
	assert.NotNil(t, reader)
	assert.NotNil(t, header)

	assert.EqualValues(t, header.ChannelMap, 0)
	assert.EqualValues(t, header.Channels, 2)
	assert.EqualValues(t, header.OutputGain, 0)
	assert.EqualValues(t, header.PreSkip, 0xf00)
	assert.EqualValues(t, header.SampleRate, 48000)
	assert.EqualValues(t, header.Version, 1)
}

func TestOggReader_ParseNextPage(t *testing.T) {
	ogg := bytes.NewReader(buildOggContainer())

	reader, _, err := NewWith(ogg)
	assert.NoError(t, err)
	assert.NotNil(t, reader)
	assert.Equal(t, int64(47), reader.bytesReadSuccesfully)

	payload, _, err := reader.ParseNextPage()
	assert.Equal(t, []byte{0x98, 0x36, 0xbe, 0x88, 0x9e}, payload)
	assert.NoError(t, err)
	assert.Equal(t, int64(80), reader.bytesReadSuccesfully)

	_, _, err = reader.ParseNextPage()
	assert.Equal(t, err, io.EOF)
}

func TestOggReader_ParseErrors(t *testing.T) {
	t.Run("Assert that Reader isn't nil", func(t *testing.T) {
		_, _, err := NewWith(nil)
		assert.Equal(t, err, errNilStream)
	})

	t.Run("Invalid ID Page Header Signature", func(t *testing.T) {
		ogg := buildOggContainer()
		ogg[0] = 0

		_, _, err := newWith(bytes.NewReader(ogg), false)
		assert.ErrorIs(t, err, errBadIDPageSignature)
	})

	t.Run("Invalid ID Page Header Type", func(t *testing.T) {
		ogg := buildOggContainer()
		ogg[5] = 0

		_, _, err := newWith(bytes.NewReader(ogg), false)
		assert.ErrorIs(t, err, errBadIDPageType)
	})

	t.Run("Invalid ID Page Payload Length", func(t *testing.T) {
		ogg := buildOggContainer()
		ogg[27] = 0

		_, _, err := newWith(bytes.NewReader(ogg), false)
		assert.ErrorIs(t, err, errBadIDPageLength)
	})

	t.Run("Invalid ID Page Payload Length", func(t *testing.T) {
		ogg := buildOggContainer()
		ogg[35] = 0

		_, _, err := newWith(bytes.NewReader(ogg), false)
		assert.ErrorIs(t, err, errBadIDPagePayloadSignature)
	})

	t.Run("Invalid Page Checksum", func(t *testing.T) {
		ogg := buildOggContainer()
		ogg[22] = 0

		_, _, err := NewWith(bytes.NewReader(ogg))
		assert.ErrorIs(t, err, errChecksumMismatch)
	})

	t.Run("Invalid Multichannel ID Page Payload Length", func(t *testing.T) {
		_, _, err := newWith(bytes.NewReader(buildSurroundOggContainerShort()), false)
		assert.ErrorIs(t, err, errBadIDPageLength)
	})

	t.Run("Unsupported Channel Mapping Family", func(t *testing.T) {
		_, _, err := newWith(bytes.NewReader(buildUnknownMappingFamilyContainer(4, 2)), false)
		assert.ErrorIs(t, err, errUnsupportedChannelMappingFamily)
	})
}

func TestOggReader_ChannelMappingFamily1(t *testing.T) {
	type testCase struct {
		name       string
		channels   uint8
		streams    uint8
		coupled    uint8
		channelMap []byte
	}

	cases := []testCase{
		{name: "1-mono", channels: 1, streams: 1, coupled: 0, channelMap: []byte{0}},
		{name: "2-stereo", channels: 2, streams: 1, coupled: 1, channelMap: []byte{0, 1}},
		{name: "3-linear-surround", channels: 3, streams: 2, coupled: 1, channelMap: []byte{0, 2, 1}},
		{name: "4-quad", channels: 4, streams: 2, coupled: 2, channelMap: []byte{0, 1, 2, 3}},
		{name: "5-5.0", channels: 5, streams: 3, coupled: 2, channelMap: []byte{0, 1, 2, 3, 4}},
		{name: "6-5.1", channels: 6, streams: 4, coupled: 2, channelMap: []byte{0, 4, 1, 2, 3, 5}},
		{name: "7-6.1", channels: 7, streams: 4, coupled: 3, channelMap: []byte{0, 1, 2, 3, 4, 5, 6}},
		{name: "8-7.1", channels: 8, streams: 5, coupled: 3, channelMap: []byte{0, 1, 2, 3, 4, 5, 6, 7}},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			reader, err := NewWithOptions(
				bytes.NewReader(buildChannelMappingFamilyContainer(1, tc.channels, tc.streams, tc.coupled, tc.channelMap)),
				WithDoChecksum(false),
			)
			assert.NoError(t, err)
			assert.NotNil(t, reader)

			payload, pageHeader, err := reader.ParseNextPage()
			assert.NoError(t, err)
			sig, ok := pageHeader.HeaderType(payload)
			assert.True(t, ok)
			assert.Equal(t, HeaderOpusID, sig)

			header, err := ParseOpusHead(payload)
			assert.NoError(t, err)
			assert.NotNil(t, header)

			assert.EqualValues(t, 1, header.Version)
			assert.EqualValues(t, tc.channels, header.Channels)
			assert.EqualValues(t, 0x138, header.PreSkip)
			assert.EqualValues(t, 48e3, header.SampleRate)
			assert.EqualValues(t, 0, header.OutputGain)
			assert.EqualValues(t, 1, header.ChannelMap)
			assert.EqualValues(t, tc.streams, header.StreamCount)
			assert.EqualValues(t, tc.coupled, header.CoupledCount)
			assert.Equal(t, string(tc.channelMap), header.ChannelMapping)
		})
	}
}

func TestOggReader_KnownChannelMappingFamilies(t *testing.T) {
	cases := []struct {
		name          string
		mappingFamily uint8
		channels      uint8
		streams       uint8
		coupled       uint8
		channelMap    []byte
	}{
		{name: "family-2", mappingFamily: 2, channels: 2, streams: 1, coupled: 1, channelMap: []byte{0, 1}},
		{name: "family-255", mappingFamily: 255, channels: 2, streams: 1, coupled: 1, channelMap: []byte{0, 1}},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			container := buildChannelMappingFamilyContainer(
				tc.mappingFamily, tc.channels, tc.streams, tc.coupled, tc.channelMap,
			)
			reader, err := NewWithOptions(bytes.NewReader(container), WithDoChecksum(false))
			assert.NoError(t, err)
			assert.NotNil(t, reader)

			payload, pageHeader, err := reader.ParseNextPage()
			assert.NoError(t, err)
			sig, ok := pageHeader.HeaderType(payload)
			assert.True(t, ok)
			assert.Equal(t, HeaderOpusID, sig)

			header, err := ParseOpusHead(payload)
			assert.NoError(t, err)
			assert.NotNil(t, header)

			assert.EqualValues(t, tc.mappingFamily, header.ChannelMap)
			assert.EqualValues(t, tc.channels, header.Channels)
			assert.EqualValues(t, 0x138, header.PreSkip)
			assert.EqualValues(t, 48e3, header.SampleRate)
			assert.EqualValues(t, 0, header.OutputGain)
		})
	}
}

func TestOggReader_ParseExtraFieldsForNonZeroMappingFamily(t *testing.T) {
	cases := []struct {
		name          string
		mappingFamily uint8
		channels      uint8
		streams       uint8
		coupled       uint8
		channelMap    []byte
	}{
		{name: "family-1-stereo", mappingFamily: 1, channels: 2, streams: 1, coupled: 1, channelMap: []byte{0, 1}},
		{name: "family-1-5.1", mappingFamily: 1, channels: 6, streams: 4, coupled: 2, channelMap: []byte{0, 4, 1, 2, 3, 5}},
		{
			name:          "family-1-7.1",
			mappingFamily: 1,
			channels:      8,
			streams:       5,
			coupled:       3,
			channelMap:    []byte{0, 1, 2, 3, 4, 5, 6, 7},
		},
		{name: "family-2", mappingFamily: 2, channels: 4, streams: 2, coupled: 2, channelMap: []byte{0, 1, 2, 3}},
		{name: "family-255", mappingFamily: 255, channels: 5, streams: 3, coupled: 2, channelMap: []byte{0, 1, 2, 3, 4}},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			container := buildChannelMappingFamilyContainer(
				tc.mappingFamily, tc.channels, tc.streams, tc.coupled, tc.channelMap,
			)
			reader, err := NewWithOptions(bytes.NewReader(container), WithDoChecksum(false))
			assert.NoError(t, err)
			assert.NotNil(t, reader)

			payload, pageHeader, err := reader.ParseNextPage()
			assert.NoError(t, err)
			sig, ok := pageHeader.HeaderType(payload)
			assert.True(t, ok)
			assert.Equal(t, HeaderOpusID, sig)

			header, err := ParseOpusHead(payload)
			assert.NoError(t, err)
			assert.NotNil(t, header)

			assert.EqualValues(t, tc.mappingFamily, header.ChannelMap)
			assert.EqualValues(t, tc.channels, header.Channels)
			assert.EqualValues(t, tc.streams, header.StreamCount)
			assert.EqualValues(t, tc.coupled, header.CoupledCount)
			assert.Equal(t, string(tc.channelMap), header.ChannelMapping)
		})
	}
}

func TestOggReader_NewWithOptions(t *testing.T) {
	t.Run("With checksum enabled (default)", func(t *testing.T) {
		reader, err := NewWithOptions(bytes.NewReader(buildOggContainer()))
		assert.NoError(t, err)
		assert.NotNil(t, reader)
		assert.True(t, reader.doChecksum)

		payload, pageHeader, err := reader.ParseNextPage()
		assert.NoError(t, err)
		assert.NotNil(t, payload)
		assert.NotNil(t, pageHeader)
		assert.Equal(t, string(HeaderOpusID), string(payload[:8]))
	})

	t.Run("With checksum enabled explicitly", func(t *testing.T) {
		reader, err := NewWithOptions(bytes.NewReader(buildOggContainer()), WithDoChecksum(true))
		assert.NoError(t, err)
		assert.NotNil(t, reader)
		assert.True(t, reader.doChecksum)

		ogg := buildOggContainer()
		ogg[22] = 0
		reader2, err := NewWithOptions(bytes.NewReader(ogg), WithDoChecksum(true))
		assert.NoError(t, err)
		assert.NotNil(t, reader2)

		_, _, err = reader2.ParseNextPage()
		assert.Equal(t, errChecksumMismatch, err)
	})

	t.Run("With checksum disabled", func(t *testing.T) {
		reader, err := NewWithOptions(bytes.NewReader(buildOggContainer()), WithDoChecksum(false))
		assert.NoError(t, err)
		assert.NotNil(t, reader)
		assert.False(t, reader.doChecksum)

		ogg := buildOggContainer()
		ogg[22] = 0
		reader2, err := NewWithOptions(bytes.NewReader(ogg), WithDoChecksum(false))
		assert.NoError(t, err)
		assert.NotNil(t, reader2)

		payload, pageHeader, err := reader2.ParseNextPage()
		assert.NoError(t, err)
		assert.NotNil(t, payload)
		assert.NotNil(t, pageHeader)
	})
}

// buildMultiTrackOggContainer generates a minimal two-track Ogg file
// with two Opus ID header pages (one for each track).
func buildMultiTrackOggContainer(
	firstSerial, secondSerial uint32,
	channels uint8,
	sampleRate uint32,
	preskip uint16,
	version uint8,
	channelMap uint8,
	outputGain uint16,
) []byte {
	firstSerialBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(firstSerialBytes, firstSerial)
	secondSerialBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(secondSerialBytes, secondSerial)

	preskipBytes := make([]byte, 2)
	binary.LittleEndian.PutUint16(preskipBytes, preskip)

	sampleRateBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(sampleRateBytes, sampleRate)

	outputGainBytes := make([]byte, 2)
	binary.LittleEndian.PutUint16(outputGainBytes, outputGain)

	firstPageHeader := []byte{
		0x4f, 0x67, 0x67, 0x53, // "OggS"
		0x00,                                           // version
		0x02,                                           // header type (beginning of stream)
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // granule position
		firstSerialBytes[0], firstSerialBytes[1], firstSerialBytes[2], firstSerialBytes[3], // serial number
		0x00, 0x00, 0x00, 0x00, // page sequence number
		0xd7, 0xb7, 0x51, 0x4a, // checksum
		0x01, // page segments
		0x13, // segment size (19 bytes)
	}

	firstPayload := []byte{
		0x4f, 0x70, 0x75, 0x73, 0x48, 0x65, 0x61, 0x64, // "OpusHead"
		version,                          // version
		channels,                         // channels
		preskipBytes[0], preskipBytes[1], // preskip
		sampleRateBytes[0], sampleRateBytes[1], sampleRateBytes[2], sampleRateBytes[3], // sample rate
		outputGainBytes[0], outputGainBytes[1], // output gain
		channelMap, // channel mapping family
	}

	// Second track: Opus ID page
	// Ogg page header (27 bytes)
	secondPageHeader := []byte{
		0x4f, 0x67, 0x67, 0x53, // "OggS"
		0x00,                                           // version
		0x02,                                           // header type (beginning of stream)
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // granule position
		secondSerialBytes[0], secondSerialBytes[1], secondSerialBytes[2], secondSerialBytes[3], // serial number
		0x00, 0x00, 0x00, 0x00, // page sequence number
		0xaf, 0xaa, 0x01, 0x8b, // checksum
		0x01, // page segments
		0x13, // segment size (19 bytes)
	}

	// Second track: OpusHead payload (19 bytes)
	secondPayload := []byte{
		0x4f, 0x70, 0x75, 0x73, 0x48, 0x65, 0x61, 0x64, // "OpusHead"
		version,                          // version
		channels,                         // channels
		preskipBytes[0], preskipBytes[1], // preskip
		sampleRateBytes[0], sampleRateBytes[1], sampleRateBytes[2], sampleRateBytes[3], // sample rate
		outputGainBytes[0], outputGainBytes[1], // output gain
		channelMap, // channel mapping family
	}

	container := make([]byte, 0, len(firstPageHeader)+len(firstPayload)+len(secondPageHeader)+len(secondPayload))
	container = append(container, firstPageHeader...)
	container = append(container, firstPayload...)
	container = append(container, secondPageHeader...)
	container = append(container, secondPayload...)

	return container
}

func TestOggReader_MultiTrackFile(t *testing.T) {
	firstSerial := uint32(0xd03ed35d)
	secondSerial := uint32(0xfa6e13f0)
	channels := uint8(1)
	sampleRate := uint32(48000)
	preskip := uint16(0x0138)
	version := uint8(1)
	channelMap := uint8(0)
	outputGain := uint16(0)

	data := buildMultiTrackOggContainer(
		firstSerial, secondSerial,
		channels, sampleRate, preskip,
		version, channelMap, outputGain,
	)

	reader, err := NewWithOptions(bytes.NewReader(data), WithDoChecksum(false))
	assert.NoError(t, err)
	assert.NotNil(t, reader)

	var headers []*OggHeader
	var pageHeaders []*OggPageHeader

	for {
		payload, pageHeader, err := reader.ParseNextPage()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			assert.NoError(t, err, "Error reading page")

			break
		}

		sig, ok := pageHeader.HeaderType(payload)
		assert.True(t, ok)
		assert.Equal(t, HeaderOpusID, sig)

		header, err2 := ParseOpusHead(payload)
		assert.NoError(t, err2)
		assert.NotNil(t, header)
		headers = append(headers, header)
		pageHeaders = append(pageHeaders, pageHeader)

		t.Logf("Found header %d: Channels=%d, SampleRate=%d, Serial=%d",
			len(headers), header.Channels, header.SampleRate, pageHeader.Serial)
	}

	assert.Equal(t, 2, len(headers), "Should find exactly 2 headers")
	assert.Equal(t, channels, headers[0].Channels, "First track should be mono")
	assert.Equal(t, channels, headers[1].Channels, "Second track should be mono")
	assert.Equal(t, sampleRate, headers[0].SampleRate, "First track should be 48kHz")
	assert.Equal(t, sampleRate, headers[1].SampleRate, "Second track should be 48kHz")

	assert.Equal(t, firstSerial, pageHeaders[0].Serial, "First track serial should match")
	assert.Equal(t, secondSerial, pageHeaders[1].Serial, "Second track serial should match")
	assert.NotEqual(t, pageHeaders[0].Serial, pageHeaders[1].Serial, "Serial numbers should be different")

	t.Logf("Multi-track file: found %d headers", len(headers))
}
