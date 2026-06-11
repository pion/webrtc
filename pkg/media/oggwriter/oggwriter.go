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
	pageHeaderTypeBeginningOfStream    = 0x02
	pageHeaderTypeEndOfStream          = 0x04
	defaultPreSkip                     = 3840 // 3840 recommended in the RFC
	idPageSignature                    = "OpusHead"
	commentPageSignature               = "OpusTags"
	pageHeaderSignature                = "OggS"
	pageHeaderSize                     = 27
)

var (
	errFileNotOpened        = errors.New("file not opened")
	errInvalidNilPacket     = errors.New("invalid nil packet")
	errDuplicateTrackSSRC   = errors.New("duplicate Ogg track SSRC")
	errDuplicateTrackSerial = errors.New("duplicate Ogg track serial")
	errTracksStarted        = errors.New("cannot add Ogg tracks after writing has started")
)

type trackConfig struct {
	serial uint32
}

type oggTrack struct {
	sampleRate              uint32
	channelCount            uint16
	preSkip                 uint16
	serial                  uint32
	pageIndex               uint32
	previousGranulePosition uint64
	previousTimestamp       uint32
	lastPayload             []byte
	lastGranulePosition     uint64
	lastPageIndex           uint32
	lastPageOffset          int64
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
	fd            *os.File
	sampleRate    uint32
	channelCount  uint16
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

// TrackOption configures a logical Opus stream in a multiplexed Ogg container.
type TrackOption func(*trackConfig) error

// WithSerial sets the Ogg bitstream serial number for a track.
func WithSerial(serial uint32) TrackOption {
	return func(config *trackConfig) error {
		config.serial = serial

		return nil
	}
}

// New builds a new single-track OGG Opus writer.
func New(fileName string, sampleRate uint32, channelCount uint16) (*OggWriter, error) {
	file, err := os.Create(fileName) //nolint:gosec
	if err != nil {
		return nil, err
	}

	writer, err := NewWith(file, sampleRate, channelCount)
	if err != nil {
		return nil, file.Close()
	}
	writer.fd = file

	return writer, nil
}

// NewWith initializes a new single-track OGG Opus writer with an io.Writer output.
func NewWith(out io.Writer, sampleRate uint32, channelCount uint16) (*OggWriter, error) {
	if out == nil {
		return nil, errFileNotOpened
	}

	writer := &OggWriter{
		stream:        out,
		checksumTable: generateChecksumTable(),
		track:         newTrackState(sampleRate, channelCount, util.RandUint32()),
	}

	if err := writer.writeTrackHeaders(writer.track); err != nil {
		return nil, err
	}

	return writer, nil
}

// NewWriter creates a new multi-track OGG Opus writer backed by a file.
func NewWriter(fileName string, sampleRate uint32, channelCount uint16) (*Writer, error) {
	file, err := os.Create(fileName) //nolint:gosec
	if err != nil {
		return nil, err
	}

	writer, err := NewWriterWith(file, sampleRate, channelCount)
	if err != nil {
		return nil, file.Close()
	}
	writer.fd = file

	return writer, nil
}

// NewWriterWith creates a new multi-track OGG Opus writer with an io.Writer output.
func NewWriterWith(out io.Writer, sampleRate uint32, channelCount uint16) (*Writer, error) {
	if out == nil {
		return nil, errFileNotOpened
	}

	writer := &Writer{
		stream:        out,
		sampleRate:    sampleRate,
		channelCount:  channelCount,
		checksumTable: generateChecksumTable(),
		tracks:        map[uint32]*Track{},
	}
	if file, ok := out.(*os.File); ok {
		writer.fd = file
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

	config := &trackConfig{}
	for _, opt := range opts {
		if err := opt(config); err != nil {
			return nil, err
		}
	}

	serial := config.serial
	if serial == 0 {
		serial = w.allocateSerial()
	} else if w.serialInUse(serial) {
		return nil, errDuplicateTrackSerial
	}

	track := &Track{
		parent: w,
		ssrc:   ssrc,
		track:  newTrackState(w.sampleRate, w.channelCount, serial),
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
	if err := parent.startLocked(); err != nil {
		return err
	}

	return parent.writeRTP(w.track, packet)
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
		w.fd = nil
		w.stream = nil
	}()

	if w.stream == nil {
		return nil
	}
	if err := w.startLocked(); err != nil {
		return err
	}

	if w.fd == nil {
		if closer, ok := w.stream.(io.Closer); ok {
			return closer.Close()
		}

		return nil
	}

	for _, track := range w.trackOrder {
		if err := markTrackEndOfStream(w.fd, w.checksumTable, track.track); err != nil {
			return err
		}
	}

	return w.fd.Close()
}

func newTrackState(sampleRate uint32, channelCount uint16, serial uint32) *oggTrack {
	return &oggTrack{
		sampleRate:              sampleRate,
		channelCount:            channelCount,
		preSkip:                 defaultPreSkip,
		serial:                  serial,
		previousTimestamp:       1,
		previousGranulePosition: 1,
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
		return writePage(w.stream, w.fd, w.checksumTable, track, payload, headerType, granulePos)
	})
}

func (w *Writer) writeTrackHeaders(track *oggTrack) error {
	return writeTrackHeaders(track, func(track *oggTrack, payload []byte, headerType uint8, granulePos uint64) error {
		return writePage(w.stream, w.fd, w.checksumTable, track, payload, headerType, granulePos)
	})
}

func writeTrackHeaders(track *oggTrack, writePageFunc func(*oggTrack, []byte, uint8, uint64) error) error {
	if err := writePageFunc(track, buildIDHeader(track), pageHeaderTypeBeginningOfStream, 0); err != nil {
		return err
	}

	return writePageFunc(track, buildCommentHeader(), pageHeaderTypeContinuationOfStream, 0)
}

func buildIDHeader(track *oggTrack) []byte {
	oggIDHeader := make([]byte, 19)

	copy(oggIDHeader[0:], idPageSignature) // Magic Signature 'OpusHead'
	oggIDHeader[8] = 1                     // Version
	//nolint:gosec // G115
	oggIDHeader[9] = uint8(track.channelCount)                     // Channel count
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
	fd *os.File,
	checksumTable *[256]uint32,
	track *oggTrack,
	payload []byte,
	headerType uint8,
	granulePos uint64,
) error {
	if fd != nil {
		offset, err := fd.Seek(0, io.SeekCurrent)
		if err != nil {
			return err
		}
		track.lastPageOffset = offset
	}

	pageIndex := track.pageIndex
	data := createPageForSerial(checksumTable, payload, headerType, granulePos, track.serial, pageIndex)
	if err := writeToStream(stream, data); err != nil {
		return err
	}

	track.lastPayload = append(track.lastPayload[:0], payload...)
	track.lastGranulePosition = granulePos
	track.lastPageIndex = pageIndex
	track.pageIndex++

	return nil
}

func createPageForSerial(
	checksumTable *[256]uint32,
	payload []byte,
	headerType uint8,
	granulePos uint64,
	serial uint32,
	pageIndex uint32,
) []byte {
	nSegments := (len(payload) / 255) + 1 // A segment can be at most 255 bytes long.

	page := make([]byte, pageHeaderSize+len(payload)+nSegments)

	copy(page[0:], pageHeaderSignature)                 // page headers starts with 'OggS'
	page[4] = 0                                         // Version
	page[5] = headerType                                // 1 = continuation, 2 = beginning of stream, 4 = end of stream
	binary.LittleEndian.PutUint64(page[6:], granulePos) // granule position
	binary.LittleEndian.PutUint32(page[14:], serial)    // Bitstream serial number
	binary.LittleEndian.PutUint32(page[18:], pageIndex) // Page sequence number
	//nolint:gosec // G115
	page[26] = uint8(nSegments) // Number of segments in page.

	for i := 0; i < nSegments-1; i++ {
		page[pageHeaderSize+i] = 255
	}
	page[pageHeaderSize+nSegments-1] = uint8(len(payload) % 255) //nolint:gosec // G115

	copy(page[pageHeaderSize+nSegments:], payload)

	var checksum uint32
	for index := range page {
		checksum = (checksum << 8) ^ checksumTable[byte(checksum>>24)^page[index]]
	}

	binary.LittleEndian.PutUint32(page[22:], checksum)

	return page
}

func (w *OggWriter) writeRTP(track *oggTrack, packet *rtp.Packet) error {
	return writeRTP(w.stream, w.fd, w.checksumTable, track, packet)
}

func (w *Writer) writeRTP(track *oggTrack, packet *rtp.Packet) error {
	return writeRTP(w.stream, w.fd, w.checksumTable, track, packet)
}

func writeRTP(
	stream io.Writer,
	fd *os.File,
	checksumTable *[256]uint32,
	track *oggTrack,
	packet *rtp.Packet,
) error {
	if packet == nil {
		return errInvalidNilPacket
	}
	if len(packet.Payload) == 0 {
		return nil
	}

	opusPacket := codecs.OpusPacket{}
	if _, err := opusPacket.Unmarshal(packet.Payload); err != nil {
		return err
	}

	payload := opusPacket.Payload[0:]

	if track.previousTimestamp != 1 {
		increment := packet.Timestamp - track.previousTimestamp
		track.previousGranulePosition += uint64(increment)
	}
	track.previousTimestamp = packet.Timestamp

	return writePage(
		stream, fd, checksumTable, track, payload, pageHeaderTypeContinuationOfStream, track.previousGranulePosition,
	)
}

func (w *Writer) startLocked() error {
	if w.started {
		return nil
	}

	for _, track := range w.trackOrder {
		if err := w.writeTrackHeaders(track.track); err != nil {
			return err
		}
	}

	w.started = true

	return nil
}

func markTrackEndOfStream(fd *os.File, checksumTable *[256]uint32, track *oggTrack) error {
	if track == nil || len(track.lastPayload) == 0 {
		return nil
	}

	data := createPageForSerial(
		checksumTable,
		track.lastPayload,
		pageHeaderTypeEndOfStream,
		track.lastGranulePosition,
		track.serial,
		track.lastPageIndex,
	)
	_, err := fd.WriteAt(data, track.lastPageOffset)

	return err
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
