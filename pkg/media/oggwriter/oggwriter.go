// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

// Package oggwriter implements OGG media container writer
package oggwriter

import (
	"encoding/binary"
	"errors"
	"io"
	"os"
	"sync"

	"github.com/pion/rtp"
	"github.com/pion/rtp/codecs"
	"github.com/pion/webrtc/v4/internal/util"
)

const (
	pageHeaderTypeContinuationOfStream = 0x00
	pageHeaderTypeContinuationOfPacket = 0x01
	pageHeaderTypeBeginningOfStream    = 0x02
	pageHeaderTypeEndOfStream          = 0x04
	defaultPreSkip                     = 3840 // 3840 recommended in the RFC
	defaultSampleRate                  = 48000
	opusGranuleSampleRate              = 48000
	maxOpusPacketSamples               = opusGranuleSampleRate * 120 / 1000
	defaultChannelCount                = 2
	idPageSignature                    = "OpusHead"
	commentPageSignature               = "OpusTags"
	pageHeaderSignature                = "OggS"
	pageHeaderSize                     = 27
	maxOggPageSegments                 = 255
	noGranulePosition                  = ^uint64(0)
)

var (
	errFileNotOpened        = errors.New("file not opened")
	errInvalidNilPacket     = errors.New("invalid nil packet")
	errDuplicateTrackSSRC   = errors.New("duplicate Ogg track SSRC")
	errDuplicateTrackSerial = errors.New("duplicate Ogg track serial")
	errTracksStarted        = errors.New("cannot add Ogg tracks after writing has started")
	errPacketSSRCMismatch   = errors.New("RTP packet SSRC does not match Ogg track SSRC")
	errInvalidOpusPacket    = errors.New("invalid Opus packet")
	errInvalidChannelCount  = errors.New("invalid channel count for family 0")
)

type pageRewriter interface {
	io.Seeker
	io.WriterAt
}

type writerConfig struct {
	sampleRate   uint32
	channelCount uint8
	pageRewriter pageRewriter
}

type trackConfig struct {
	sampleRate   uint32
	channelCount uint8
	serial       uint32
	serialSet    bool
}

// WriterOption configures a multiplexed Ogg writer.
type WriterOption interface {
	applyWriterConfig(*writerConfig) error
}

// TrackOption configures a logical Opus stream in a multiplexed Ogg container.
type TrackOption interface {
	applyTrackConfig(*trackConfig) error
}

// WriterTrackOption configures both Writer defaults and per-track overrides.
type WriterTrackOption interface {
	WriterOption
	TrackOption
}

type writerOptionFunc func(*writerConfig) error

func (f writerOptionFunc) applyWriterConfig(config *writerConfig) error {
	return f(config)
}

type trackOptionFunc func(*trackConfig) error

func (f trackOptionFunc) applyTrackConfig(config *trackConfig) error {
	return f(config)
}

type sampleRateOption uint32

func (o sampleRateOption) applyWriterConfig(config *writerConfig) error {
	config.sampleRate = uint32(o)

	return nil
}

func (o sampleRateOption) applyTrackConfig(config *trackConfig) error {
	config.sampleRate = uint32(o)

	return nil
}

type channelCountOption uint16

func (o channelCountOption) applyWriterConfig(config *writerConfig) error {
	channelCount, err := validateChannelCount(uint16(o))
	if err != nil {
		return err
	}

	config.channelCount = channelCount

	return nil
}

func (o channelCountOption) applyTrackConfig(config *trackConfig) error {
	channelCount, err := validateChannelCount(uint16(o))
	if err != nil {
		return err
	}

	config.channelCount = channelCount

	return nil
}

type oggTrack struct {
	sampleRate              uint32
	channelCount            uint8
	preSkip                 uint16
	serial                  uint32
	pageIndex               uint32
	previousGranulePosition uint64
	lastPayload             []byte
	lastGranulePosition     uint64
	lastPageIndex           uint32
	lastPageOffset          int64
	lastPageHeaderType      uint8
	lastPageWritten         bool
}

type oggPage struct {
	data       []byte
	payload    []byte
	headerType uint8
	granulePos uint64
	pageIndex  uint32
}

// OggWriter is used to take RTP packets and write them to a single-track OGG.
type OggWriter struct {
	mu            sync.Mutex
	stream        io.Writer
	fd            *os.File
	checksumTable *[256]uint32
	track         *oggTrack
}

// Writer is used to write multiple logical Opus streams into one OGG.
// Tracks are added before writing starts, and each returned Track owns WriteRTP.
type Writer struct {
	mu            sync.Mutex
	stream        io.Writer
	pageRewriter  pageRewriter
	sampleRate    uint32
	channelCount  uint8
	checksumTable *[256]uint32
	tracks        map[uint32]*Track
	trackOrder    []*Track
	started       bool
}

// Track writes RTP packets for a single logical Opus stream in a Writer.
type Track struct {
	parent *Writer
	ssrc   uint32
	track  *oggTrack
}

// WithSerial sets the Ogg bitstream serial number for a track.
func WithSerial(serial uint32) TrackOption {
	return trackOptionFunc(func(config *trackConfig) error {
		config.serial = serial
		config.serialSet = true

		return nil
	})
}

// WithSampleRate sets the default sample rate for a Writer or overrides it for a Track.
func WithSampleRate(sampleRate uint32) WriterTrackOption {
	return sampleRateOption(sampleRate)
}

// WithChannelCount sets the default channel count for a Writer or overrides it for a Track.
func WithChannelCount(channelCount uint16) WriterTrackOption {
	return channelCountOption(channelCount)
}

// WithSeekableOutput enables close-time Ogg page rewrites for outputs that support Seek and WriteAt.
func WithSeekableOutput(output interface {
	io.Seeker
	io.WriterAt
},
) WriterOption {
	return writerOptionFunc(func(config *writerConfig) error {
		config.pageRewriter = output

		return nil
	})
}

// New builds a new single-track OGG Opus writer.
func New(fileName string, sampleRate uint32, channelCount uint16) (*OggWriter, error) {
	if _, err := validateChannelCount(channelCount); err != nil {
		return nil, err
	}

	file, err := os.Create(fileName) //nolint:gosec
	if err != nil {
		return nil, err
	}

	writer, err := newWith(file, file, sampleRate, channelCount)
	if err != nil {
		return nil, file.Close()
	}

	return writer, nil
}

// NewWith initializes a new single-track OGG Opus writer with an io.Writer output.
func NewWith(out io.Writer, sampleRate uint32, channelCount uint16) (*OggWriter, error) {
	return newWith(out, nil, sampleRate, channelCount)
}

func newWith(out io.Writer, fd *os.File, sampleRate uint32, channelCount uint16) (*OggWriter, error) {
	if out == nil {
		return nil, errFileNotOpened
	}
	validatedChannelCount, err := validateChannelCount(channelCount)
	if err != nil {
		return nil, err
	}

	writer := &OggWriter{
		stream:        out,
		fd:            fd,
		checksumTable: generateChecksumTable(),
		track:         newTrackState(sampleRate, validatedChannelCount, util.RandUint32()),
	}

	if err := writer.writeTrackHeaders(writer.track); err != nil {
		return nil, err
	}

	return writer, nil
}

// NewWriter creates a new multi-track OGG Opus writer with an io.Writer output.
func NewWriter(out io.Writer, opts ...WriterOption) (*Writer, error) {
	if out == nil {
		return nil, errFileNotOpened
	}

	config := &writerConfig{
		sampleRate:   defaultSampleRate,
		channelCount: defaultChannelCount,
	}
	for _, opt := range opts {
		if err := opt.applyWriterConfig(config); err != nil {
			return nil, err
		}
	}

	writer := &Writer{
		stream:        out,
		pageRewriter:  config.pageRewriter,
		sampleRate:    config.sampleRate,
		channelCount:  config.channelCount,
		checksumTable: generateChecksumTable(),
		tracks:        map[uint32]*Track{},
	}

	return writer, nil
}

// NewTrack registers a logical Opus stream and returns its Track.
// It must be called before the first packet is written.
func (w *Writer) NewTrack(ssrc uint32, opts ...TrackOption) (*Track, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.stream == nil {
		return nil, errFileNotOpened
	}
	if w.started {
		return nil, errTracksStarted
	}
	if _, ok := w.tracks[ssrc]; ok {
		return nil, errDuplicateTrackSSRC
	}

	config := &trackConfig{
		sampleRate:   w.sampleRate,
		channelCount: w.channelCount,
	}
	for _, opt := range opts {
		if err := opt.applyTrackConfig(config); err != nil {
			return nil, err
		}
	}

	serial := config.serial
	if !config.serialSet {
		serial = w.allocateSerial()
	} else if w.serialInUse(serial) {
		return nil, errDuplicateTrackSerial
	}

	track := &Track{
		parent: w,
		ssrc:   ssrc,
		track:  newTrackState(config.sampleRate, config.channelCount, serial),
	}
	w.tracks[ssrc] = track
	w.trackOrder = append(w.trackOrder, track)

	return track, nil
}

// WriteRTP adds a new packet and writes the appropriate headers for it.
func (w *OggWriter) WriteRTP(packet *rtp.Packet) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.stream == nil {
		return errFileNotOpened
	}

	return w.writeRTP(w.track, packet)
}

// WriteRTP adds a packet to a single logical stream inside a Writer.
func (w *Track) WriteRTP(packet *rtp.Packet) error {
	parent := w.parent

	parent.mu.Lock()
	defer parent.mu.Unlock()

	if parent.stream == nil {
		return errFileNotOpened
	}
	if packet == nil {
		return errInvalidNilPacket
	}
	if packet.SSRC != w.ssrc {
		return errPacketSSRCMismatch
	}
	payload, shouldWrite, err := opusPayloadFromPacket(packet)
	if err != nil {
		return err
	}
	if !shouldWrite {
		return nil
	}
	if err := parent.startLocked(); err != nil {
		return err
	}

	return parent.writeOpusPayload(w.track, payload)
}

// Close stops single-track recording.
func (w *OggWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	defer func() {
		w.fd = nil
		w.stream = nil
	}()

	if w.fd == nil {
		if closer, ok := w.stream.(io.Closer); ok {
			return closer.Close()
		}

		return nil
	}

	if err := markTrackEndOfStream(w.fd, w.checksumTable, w.track); err != nil {
		return err
	}

	return w.fd.Close()
}

// Close stops multi-track recording.
func (w *Writer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	defer func() {
		w.pageRewriter = nil
		w.stream = nil
	}()

	if w.stream == nil {
		return nil
	}
	if err := w.startLocked(); err != nil {
		return err
	}

	if w.pageRewriter != nil {
		for _, track := range w.trackOrder {
			if err := markTrackEndOfStream(w.pageRewriter, w.checksumTable, track.track); err != nil {
				return err
			}
		}
	}

	if closer, ok := w.stream.(io.Closer); ok {
		return closer.Close()
	}

	return nil
}

func newTrackState(sampleRate uint32, channelCount uint8, serial uint32) *oggTrack {
	return &oggTrack{
		sampleRate:   sampleRate,
		channelCount: channelCount,
		preSkip:      defaultPreSkip,
		serial:       serial,
	}
}

func validateChannelCount(channelCount uint16) (uint8, error) {
	switch channelCount {
	case 1, 2:
		return uint8(channelCount), nil
	default:
		return 0, errInvalidChannelCount
	}
}

func (w *Writer) serialInUse(serial uint32) bool {
	for _, track := range w.trackOrder {
		if track.track.serial == serial {
			return true
		}
	}

	return false
}

func (w *Writer) allocateSerial() uint32 {
	serial := util.RandUint32()
	for w.serialInUse(serial) {
		serial = util.RandUint32()
	}

	return serial
}

func (w *OggWriter) writeTrackHeaders(track *oggTrack) error {
	return writeTrackHeaders(track, func(track *oggTrack, payload []byte, headerType uint8, granulePos uint64) error {
		return writePage(w.stream, pageRewriterForFile(w.fd), w.checksumTable, track, payload, headerType, granulePos)
	})
}

func (w *Writer) writeTrackIDHeader(track *oggTrack) error {
	return writeTrackIDHeader(track, func(track *oggTrack, payload []byte, headerType uint8, granulePos uint64) error {
		return writePage(w.stream, w.pageRewriter, w.checksumTable, track, payload, headerType, granulePos)
	})
}

func (w *Writer) writeTrackCommentHeader(track *oggTrack) error {
	return writeTrackCommentHeader(
		track,
		func(track *oggTrack, payload []byte, headerType uint8, granulePos uint64) error {
			return writePage(w.stream, w.pageRewriter, w.checksumTable, track, payload, headerType, granulePos)
		},
	)
}

func writeTrackHeaders(track *oggTrack, writePageFunc func(*oggTrack, []byte, uint8, uint64) error) error {
	if err := writeTrackIDHeader(track, writePageFunc); err != nil {
		return err
	}

	return writeTrackCommentHeader(track, writePageFunc)
}

func writeTrackIDHeader(track *oggTrack, writePageFunc func(*oggTrack, []byte, uint8, uint64) error) error {
	return writePageFunc(track, buildIDHeader(track), pageHeaderTypeBeginningOfStream, 0)
}

func writeTrackCommentHeader(track *oggTrack, writePageFunc func(*oggTrack, []byte, uint8, uint64) error) error {
	return writePageFunc(track, buildCommentHeader(), pageHeaderTypeContinuationOfStream, 0)
}

func buildIDHeader(track *oggTrack) []byte {
	oggIDHeader := make([]byte, 19)

	copy(oggIDHeader[0:], idPageSignature)                         // Magic Signature 'OpusHead'
	oggIDHeader[8] = 1                                             // Version
	oggIDHeader[9] = track.channelCount                            // Channel count
	binary.LittleEndian.PutUint16(oggIDHeader[10:], track.preSkip) // pre-skip
	binary.LittleEndian.PutUint32(oggIDHeader[12:], track.sampleRate)
	binary.LittleEndian.PutUint16(oggIDHeader[16:], 0) // output gain
	oggIDHeader[18] = 0                                // channel map 0 = one stream: mono or stereo

	return oggIDHeader
}

func buildCommentHeader() []byte {
	oggCommentHeader := make([]byte, 21)
	copy(oggCommentHeader[0:], commentPageSignature)        // Magic Signature 'OpusTags'
	binary.LittleEndian.PutUint32(oggCommentHeader[8:], 5)  // Vendor Length
	copy(oggCommentHeader[12:], "pion")                     // Vendor name 'pion'
	binary.LittleEndian.PutUint32(oggCommentHeader[17:], 0) // User Comment List Length

	return oggCommentHeader
}

func writePage(
	stream io.Writer,
	rewriter pageRewriter,
	checksumTable *[256]uint32,
	track *oggTrack,
	payload []byte,
	headerType uint8,
	granulePos uint64,
) error {
	pages := createPagesForSerial(checksumTable, payload, headerType, granulePos, track.serial, track.pageIndex)

	var offset int64
	if rewriter != nil {
		var err error
		offset, err = rewriter.Seek(0, io.SeekCurrent)
		if err != nil {
			return err
		}
	}

	for i, page := range pages {
		pageOffset := offset
		if err := writeToStream(stream, page.data); err != nil {
			return err
		}

		if rewriter != nil {
			if i == len(pages)-1 {
				track.lastPageOffset = pageOffset
				track.lastPayload = append(track.lastPayload[:0], page.payload...)
				track.lastGranulePosition = page.granulePos
				track.lastPageIndex = page.pageIndex
				track.lastPageHeaderType = page.headerType
				track.lastPageWritten = true
			}
			offset += int64(len(page.data))
		}
	}
	track.pageIndex += uint32(len(pages)) //nolint:gosec // Page counts are bounded by the packet size in memory.

	return nil
}

func createPagesForSerial(
	checksumTable *[256]uint32,
	payload []byte,
	headerType uint8,
	granulePos uint64,
	serial uint32,
	pageIndex uint32,
) []oggPage {
	pages := []oggPage{}
	payloadOffset := 0
	remainingPayload := len(payload)
	firstPage := true

	for {
		segmentTable := make([]byte, 0, maxOggPageSegments)
		pagePayloadSize := 0
		packetComplete := false
		for len(segmentTable) < maxOggPageSegments {
			if remainingPayload >= 255 {
				segmentTable = append(segmentTable, 255)
				pagePayloadSize += 255
				remainingPayload -= 255

				continue
			}

			segmentTable = append(segmentTable, byte(remainingPayload)) //nolint:gosec // remainingPayload is < 255 here.
			pagePayloadSize += remainingPayload
			remainingPayload = 0
			packetComplete = true

			break
		}

		pagePayload := payload[payloadOffset : payloadOffset+pagePayloadSize]
		pageHeaderType := packetPageHeaderType(headerType, firstPage, packetComplete)
		pageGranulePos := noGranulePosition
		if packetComplete {
			pageGranulePos = granulePos
		}

		pages = append(pages, oggPage{
			data: createPageForSerialWithSegments(
				checksumTable,
				pagePayload,
				segmentTable,
				pageHeaderType,
				pageGranulePos,
				serial,
				pageIndex,
			),
			payload:    pagePayload,
			headerType: pageHeaderType,
			granulePos: pageGranulePos,
			pageIndex:  pageIndex,
		})

		payloadOffset += pagePayloadSize
		pageIndex++
		firstPage = false
		if packetComplete {
			break
		}
	}

	return pages
}

func packetPageHeaderType(headerType uint8, firstPage, packetComplete bool) uint8 {
	if firstPage {
		if packetComplete {
			return headerType
		}

		return headerType &^ pageHeaderTypeEndOfStream
	}

	pageHeaderType := uint8(pageHeaderTypeContinuationOfPacket)
	if packetComplete {
		pageHeaderType |= headerType & pageHeaderTypeEndOfStream
	}

	return pageHeaderType
}

func createPageForSerial(
	checksumTable *[256]uint32,
	payload []byte,
	headerType uint8,
	granulePos uint64,
	serial uint32,
	pageIndex uint32,
) []byte {
	pages := createPagesForSerial(checksumTable, payload, headerType, granulePos, serial, pageIndex)
	pageSize := 0
	for _, page := range pages {
		pageSize += len(page.data)
	}

	data := make([]byte, 0, pageSize)
	for _, page := range pages {
		data = append(data, page.data...)
	}

	return data
}

func createPageForSerialWithSegments(
	checksumTable *[256]uint32,
	payload []byte,
	segmentTable []byte,
	headerType uint8,
	granulePos uint64,
	serial uint32,
	pageIndex uint32,
) []byte {
	page := make([]byte, pageHeaderSize+len(segmentTable)+len(payload))

	copy(page[0:], pageHeaderSignature)                 // page headers starts with 'OggS'
	page[4] = 0                                         // Version
	page[5] = headerType                                // 1 = continuation, 2 = beginning of stream, 4 = end of stream
	binary.LittleEndian.PutUint64(page[6:], granulePos) // granule position
	binary.LittleEndian.PutUint32(page[14:], serial)    // Bitstream serial number
	binary.LittleEndian.PutUint32(page[18:], pageIndex) // Page sequence number
	page[26] = uint8(len(segmentTable))                 //nolint:gosec // segmentTable is capped at maxOggPageSegments.

	copy(page[pageHeaderSize:], segmentTable)
	copy(page[pageHeaderSize+len(segmentTable):], payload)

	var checksum uint32
	for index := range page {
		checksum = (checksum << 8) ^ checksumTable[byte(checksum>>24)^page[index]]
	}

	binary.LittleEndian.PutUint32(page[22:], checksum)

	return page
}

func (w *OggWriter) writeRTP(track *oggTrack, packet *rtp.Packet) error {
	return writeRTP(w.stream, pageRewriterForFile(w.fd), w.checksumTable, track, packet)
}

func (w *Writer) writeOpusPayload(track *oggTrack, payload []byte) error {
	return writeOpusPayload(w.stream, w.pageRewriter, w.checksumTable, track, payload)
}

func writeRTP(
	stream io.Writer,
	rewriter pageRewriter,
	checksumTable *[256]uint32,
	track *oggTrack,
	packet *rtp.Packet,
) error {
	payload, shouldWrite, err := opusPayloadFromPacket(packet)
	if err != nil || !shouldWrite {
		return err
	}

	return writeOpusPayload(stream, rewriter, checksumTable, track, payload)
}

func opusPayloadFromPacket(packet *rtp.Packet) ([]byte, bool, error) {
	if packet == nil {
		return nil, false, errInvalidNilPacket
	}
	if len(packet.Payload) == 0 {
		return nil, false, nil
	}

	opusPacket := codecs.OpusPacket{}
	if _, err := opusPacket.Unmarshal(packet.Payload); err != nil {
		return nil, false, err
	}

	return opusPacket.Payload, true, nil
}

func writeOpusPayload(
	stream io.Writer,
	rewriter pageRewriter,
	checksumTable *[256]uint32,
	track *oggTrack,
	payload []byte,
) error {
	sampleCount, err := opusPacketSampleCount(payload)
	if err != nil {
		return err
	}
	track.previousGranulePosition += sampleCount

	return writePage(
		stream, rewriter, checksumTable, track, payload, pageHeaderTypeContinuationOfStream, track.previousGranulePosition,
	)
}

func opusPacketSampleCount(payload []byte) (uint64, error) {
	if len(payload) == 0 {
		return 0, errInvalidOpusPacket
	}

	frameCount, err := opusPacketFrameCount(payload)
	if err != nil {
		return 0, err
	}

	sampleCount := uint64(opusSamplesPerFrame(payload[0])) * uint64(frameCount)
	if sampleCount > maxOpusPacketSamples {
		return 0, errInvalidOpusPacket
	}

	return sampleCount, nil
}

func opusPacketFrameCount(payload []byte) (uint8, error) {
	switch payload[0] & 0x03 {
	case 0:
		return 1, nil
	case 1, 2:
		return 2, nil
	case 3:
		if len(payload) < 2 {
			return 0, errInvalidOpusPacket
		}

		frameCount := payload[1] & 0x3f
		if frameCount == 0 {
			return 0, errInvalidOpusPacket
		}

		return frameCount, nil
	default:
		return 0, errInvalidOpusPacket
	}
}

func opusSamplesPerFrame(toc byte) uint32 {
	if toc&0x80 != 0 {
		return (opusGranuleSampleRate << ((toc >> 3) & 0x03)) / 400
	}
	if toc&0x60 == 0x60 {
		if toc&0x08 != 0 {
			return opusGranuleSampleRate / 50
		}

		return opusGranuleSampleRate / 100
	}

	frameSize := (toc >> 3) & 0x03
	if frameSize == 3 {
		return opusGranuleSampleRate * 60 / 1000
	}

	return (opusGranuleSampleRate << frameSize) / 100
}

func (w *Writer) startLocked() error {
	if w.started {
		return nil
	}

	for _, track := range w.trackOrder {
		if err := w.writeTrackIDHeader(track.track); err != nil {
			return err
		}
	}
	for _, track := range w.trackOrder {
		if err := w.writeTrackCommentHeader(track.track); err != nil {
			return err
		}
	}

	w.started = true

	return nil
}

func markTrackEndOfStream(rewriter pageRewriter, checksumTable *[256]uint32, track *oggTrack) error {
	if track == nil || !track.lastPageWritten {
		return nil
	}

	data := createPageForSerial(
		checksumTable,
		track.lastPayload,
		track.lastPageHeaderType|pageHeaderTypeEndOfStream,
		track.lastGranulePosition,
		track.serial,
		track.lastPageIndex,
	)
	_, err := rewriter.WriteAt(data, track.lastPageOffset)

	return err
}

func pageRewriterForFile(fd *os.File) pageRewriter {
	if fd == nil {
		return nil
	}

	return fd
}

func writeToStream(stream io.Writer, p []byte) error {
	if stream == nil {
		return errFileNotOpened
	}

	_, err := stream.Write(p)

	return err
}

func generateChecksumTable() *[256]uint32 {
	var table [256]uint32
	const poly = 0x04c11db7

	for i := range table {
		remainder := uint32(i) << 24 //nolint:gosec // G115
		for range 8 {
			if (remainder & 0x80000000) != 0 {
				remainder = (remainder << 1) ^ poly
			} else {
				remainder <<= 1
			}
		}
		table[i] = (remainder & 0xffffffff) //nolint:gosec // no out of bounds access here.
	}

	return &table
}
