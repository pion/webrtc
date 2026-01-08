// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package oggreader

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
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

// buildOpusTagsPayload builds an OpusTags payload.
func buildOpusTagsPayload(vendor string, comments []UserComment) []byte {
	payload := []byte("OpusTags")

	vendorBytes := []byte(vendor)
	vendorLen := make([]byte, 4)
	//nolint:gosec // G115: test-only, sized by construction
	binary.LittleEndian.PutUint32(vendorLen, uint32(len(vendorBytes)))
	payload = append(payload, vendorLen...)
	payload = append(payload, vendorBytes...)

	commentCount := make([]byte, 4)
	//nolint:gosec // G115: test-only, sized by construction
	binary.LittleEndian.PutUint32(commentCount, uint32(len(comments)))
	payload = append(payload, commentCount...)

	for _, c := range comments {
		comment := c.Comment + "=" + c.Value
		commentBytes := []byte(comment)
		commentLen := make([]byte, 4)
		//nolint:gosec // G115: test-only, sized by construction
		binary.LittleEndian.PutUint32(commentLen, uint32(len(commentBytes)))
		payload = append(payload, commentLen...)
		payload = append(payload, commentBytes...)
	}

	return payload
}

// buildOggPage builds a complete Ogg page with header, segment table, and payload.
func buildOggPage(serial uint32, pageIndex uint32, headerType uint8, payload []byte) []byte {
	serialBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(serialBytes, serial)

	indexBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(indexBytes, pageIndex)

	// Build segment table (single segment containing entire payload)
	segmentTable := []byte{byte(len(payload))}

	// Build page header (27 bytes)
	header := []byte{
		0x4f, 0x67, 0x67, 0x53, // "OggS"
		0x00,                                           // version
		headerType,                                     // header type
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // granule position
		serialBytes[0], serialBytes[1], serialBytes[2], serialBytes[3], // serial number
		indexBytes[0], indexBytes[1], indexBytes[2], indexBytes[3], // page sequence number
		0x00, 0x00, 0x00, 0x00, // checksum (will be zero, checksum disabled in test)
		0x01, // page segments count
	}

	page := make([]byte, 0, len(header)+len(segmentTable)+len(payload))
	page = append(page, header...)
	page = append(page, segmentTable...)
	page = append(page, payload...)

	return page
}

// buildOpusHeadPayload builds an OpusHead payload.
func buildOpusHeadPayload(
	version, channels uint8,
	preskip uint16,
	sampleRate uint32,
	outputGain uint16,
	channelMap uint8,
) []byte {
	payload := []byte("OpusHead")
	payload = append(payload, version)
	payload = append(payload, channels)

	preskipBytes := make([]byte, 2)
	binary.LittleEndian.PutUint16(preskipBytes, preskip)
	payload = append(payload, preskipBytes...)

	sampleRateBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(sampleRateBytes, sampleRate)
	payload = append(payload, sampleRateBytes...)

	outputGainBytes := make([]byte, 2)
	binary.LittleEndian.PutUint16(outputGainBytes, outputGain)
	payload = append(payload, outputGainBytes...)
	payload = append(payload, channelMap)

	return payload
}

// buildTwoTrackOggContainer builds a complete two-track Ogg container.
// Track 1: OpusHead (index 0) + OpusTags (index 1).
// Track 2: OpusHead (index 0) + OpusTags (index 1).
func buildTwoTrackOggContainer(
	serial1, serial2 uint32,
	track1Comments, track2Comments []UserComment,
) []byte {
	opusHeadPayload := buildOpusHeadPayload(1, 2, 0x0138, 48000, 0, 0)

	vendor := "TestVendor"
	track1TagsPayload := buildOpusTagsPayload(vendor, track1Comments)
	track2TagsPayload := buildOpusTagsPayload(vendor, track2Comments)

	track1OpusHeadPage := buildOggPage(serial1, 0, pageHeaderTypeBeginningOfStream, opusHeadPayload)
	track1OpusTagsPage := buildOggPage(serial1, 1, 0, track1TagsPayload)
	track2OpusHeadPage := buildOggPage(serial2, 0, pageHeaderTypeBeginningOfStream, opusHeadPayload)
	track2OpusTagsPage := buildOggPage(serial2, 1, 0, track2TagsPayload)

	totalLen := len(track1OpusHeadPage) + len(track1OpusTagsPage) +
		len(track2OpusHeadPage) + len(track2OpusTagsPage)
	container := make([]byte, 0, totalLen)
	container = append(container, track1OpusHeadPage...)
	container = append(container, track1OpusTagsPage...)
	container = append(container, track2OpusHeadPage...)
	container = append(container, track2OpusTagsPage...)

	return container
}

func processPages(reader *OggReader) ([]HeaderType, []*OpusTags, error) {
	var headersFound []HeaderType
	var opusTagsFound []*OpusTags

	for {
		payload, pageHeader, err := reader.ParseNextPage()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return nil, nil, err
		}

		sig, ok := pageHeader.HeaderType(payload)
		if !ok {
			continue
		}

		headersFound = append(headersFound, sig)
		if sig == HeaderOpusTags {
			tags, err := ParseOpusTags(payload)
			if err != nil {
				return nil, nil, err
			}
			if tags != nil {
				opusTagsFound = append(opusTagsFound, tags)
			}
		}
	}

	return headersFound, opusTagsFound, nil
}

func countHeaderTypes(headersFound []HeaderType) (int, int) {
	opusIDCount := 0
	opusTagsCount := 0
	for _, h := range headersFound {
		switch h {
		case HeaderOpusID:
			opusIDCount++
		case HeaderOpusTags:
			opusTagsCount++
		default:
		}
	}

	return opusIDCount, opusTagsCount
}

func userCommentsToMap(comments []UserComment) map[string]string {
	out := make(map[string]string, len(comments))
	for _, c := range comments {
		out[c.Comment] = c.Value
	}

	return out
}

func TestOggReader_DetectHeadersAndTags(t *testing.T) {
	serial1 := uint32(0xd03ed35d)
	serial2 := uint32(0xfa6e13f0)

	track1Title := hex.EncodeToString([]byte{
		0x6e, 0x65, 0x76, 0x65, 0x72, 0x20, 0x67, 0x6f, 0x6e, 0x6e, 0x61, 0x20,
		0x67, 0x69, 0x76, 0x65, 0x20, 0x79, 0x6f, 0x75, 0x20, 0x75, 0x70,
	})

	track1Comments := []UserComment{
		{Comment: "title", Value: track1Title},
		{Comment: "encoder", Value: "test-encoder-v1.0"},
	}
	track2Comments := []UserComment{
		{Comment: "title", Value: "Noise Track 2"},
		{Comment: "encoder", Value: "test-encoder-v1.0"},
	}
	data := buildTwoTrackOggContainer(serial1, serial2, track1Comments, track2Comments)

	reader, err := NewWithOptions(bytes.NewReader(data), WithDoChecksum(false))
	assert.NoError(t, err)
	assert.NotNil(t, reader)

	headersFound, opusTagsFound, err := processPages(reader)
	assert.NoError(t, err)

	assert.Greater(t, len(headersFound), 0, "Should find at least one header or tag")

	opusIDCount, opusTagsCount := countHeaderTypes(headersFound)

	assert.Equal(t, 2, opusIDCount, "Should find exactly 2 OpusHead pages")
	assert.Equal(t, 2, opusTagsCount, "Should find exactly 2 OpusTags pages")

	assert.Equal(t, 2, len(opusTagsFound), "Should parse 2 OpusTags")

	assert.Equal(t, "TestVendor", opusTagsFound[0].Vendor)
	assert.Equal(t, "TestVendor", opusTagsFound[1].Vendor)

	track1 := userCommentsToMap(opusTagsFound[0].UserComments)
	track2 := userCommentsToMap(opusTagsFound[1].UserComments)

	assert.Equal(t, track1Title, track1["title"])
	assert.Equal(t, "test-encoder-v1.0", track1["encoder"])
	assert.Equal(t, "Noise Track 2", track2["title"])
	assert.Equal(t, "test-encoder-v1.0", track2["encoder"])
}

func TestParseOpusTagsErrors(t *testing.T) {
	makeHeader := func(length int) []byte {
		payload := make([]byte, length)
		copy(payload, []byte(HeaderOpusTags))

		return payload
	}

	tests := []struct {
		name       string
		payload    []byte
		errMessage string
	}{
		{
			name:       "payload too short",
			payload:    []byte("short"),
			errMessage: "payload too short",
		},
		{
			name:       "bad signature",
			payload:    append([]byte("OpusHead"), make([]byte, 8)...), // length 16, wrong magic
			errMessage: "expected \"OpusTags\"",
		},
		{
			name: "vendor length longer than payload",
			payload: func() []byte {
				payload := makeHeader(20)
				binary.LittleEndian.PutUint32(payload[8:], 10) // vendor length larger than remaining bytes

				return payload
			}(),
			errMessage: "vendor string",
		},
		{
			name: "unreasonable comment count",
			payload: func() []byte {
				payload := makeHeader(17) // 8 (magic) + 4 (vendor len) + 1 (vendor) + 4 (comment count)
				binary.LittleEndian.PutUint32(payload[8:], 1)
				payload[12] = 'v'
				binary.LittleEndian.PutUint32(payload[13:], 3) // comment count too large for remaining payload

				return payload
			}(),
			errMessage: "unreasonable comment count",
		},
		{
			name: "payload too short for first comment length",
			payload: func() []byte {
				payload := makeHeader(16) // exactly header + vendor len + comment count, but no room for comment len
				binary.LittleEndian.PutUint32(payload[8:], 0)
				binary.LittleEndian.PutUint32(payload[12:], 1)

				return payload
			}(),
			errMessage: "comment len 0",
		},
		{
			name: "payload too short for comment data",
			payload: func() []byte {
				payload := makeHeader(20) // room for comment len, but not the comment itself
				binary.LittleEndian.PutUint32(payload[8:], 0)
				binary.LittleEndian.PutUint32(payload[12:], 1)
				binary.LittleEndian.PutUint32(payload[16:], 10) // comment claims 10 bytes, none available

				return payload
			}(),
			errMessage: "comment 0",
		},
		{
			name: "invalid comment format",
			payload: func() []byte {
				comment := []byte("novalue")
				payload := makeHeader(20 + len(comment)) // 8 magic + 4 vendor len + 4 comment count + 4 comment len + comment

				binary.LittleEndian.PutUint32(payload[8:], 0)                     // vendor length
				binary.LittleEndian.PutUint32(payload[12:], 1)                    // one comment
				binary.LittleEndian.PutUint32(payload[16:], uint32(len(comment))) //nolint:gosec
				copy(payload[20:], comment)                                       // missing '=' separator

				return payload
			}(),
			errMessage: "invalid comment 0",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			tags, err := ParseOpusTags(tc.payload)
			assert.Nil(t, tags)
			assert.Error(t, err)
			assert.ErrorIs(t, err, errBadOpusTagsSignature)
			assert.ErrorContains(t, err, tc.errMessage)
		})
	}
}

func TestParseVendorStringMissingCommentCount(t *testing.T) {
	const (
		headerMagicLen = 8
		u32Size        = 4
	)

	// Build payload with just enough room for magic, vendor length, and vendor string
	// but not enough for the comment count field to trigger the vendor error path.
	payload := make([]byte, headerMagicLen+u32Size+1) // 13 bytes total
	copy(payload, []byte(HeaderOpusTags))
	binary.LittleEndian.PutUint32(payload[headerMagicLen:], 1) // vendor length
	payload[headerMagicLen+u32Size] = 'v'                      // single vendor byte

	vendor, end, err := parseVendorString(payload, headerMagicLen, u32Size, headerMagicLen+u32Size)
	assert.Empty(t, vendor)
	assert.Zero(t, end)
	assert.ErrorIs(t, err, errBadOpusTagsSignature)
	assert.ErrorContains(t, err, "vendor+comment count")
}

func TestParseOpusHead_EmptyPayload_NoPanic(t *testing.T) {
	_, err := ParseOpusHead([]byte{})
	assert.Error(t, err)
}

func TestParseOpusHead_ChannelMappingSliceOverflow_NoPanic(t *testing.T) {
	const channels uint8 = 235

	payload := makeOpusHeadWithChannelMapping(channels, 1)

	h, err := ParseOpusHead(payload)
	assert.NoError(t, err)

	assert.Equal(t, len(h.ChannelMapping), int(channels))
}

func makeOpusHeadWithChannelMapping(channels uint8, mappingFamily uint8) []byte {
	baseLen := 19
	totalLen := baseLen
	if mappingFamily != 0 {
		totalLen = 21 + int(channels)
	}

	pack := make([]byte, totalLen)
	copy(pack[0:8], []byte("OpusHead"))

	pack[8] = 1
	pack[9] = channels

	binary.LittleEndian.PutUint16(pack[10:12], 0)
	binary.LittleEndian.PutUint32(pack[12:16], 48000)
	binary.LittleEndian.PutUint16(pack[16:18], 0)
	pack[18] = mappingFamily

	if mappingFamily != 0 {
		pack[19] = channels
		pack[20] = 0

		for i := 0; i < int(channels); i++ {
			pack[21+i] = uint8(i) //nolint:gosec // G115: test-only, uint8(i) is in range
		}
	}

	return pack
}
