// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js

package webrtc

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"time"
	"weak"

	"github.com/pion/datachannel"
	"github.com/pion/logging"
	"github.com/pion/sctp"
	"github.com/pion/webrtc/v4/pkg/rtcerr"
)

const sctpMaxChannels = uint16(65535)

// A reservation belongs to a channel, even before its stream has been opened.
// The captured generation distinguishes old bindings from streams opened after
// a restart's retained-stream snapshot was taken.
type dataChannelReservation struct {
	id          uint16
	association weak.Pointer[sctp.Association]
	stream      weak.Pointer[sctp.Stream]
	generation  uint64
}

func newSCTPTransportMetadata(metadata sctp.AssociationMetadata) SCTPTransportMetadata {
	partialReliabilityMode := SCTPTransportPartialReliabilityModeNone
	switch metadata.PartialReliabilityMode {
	case sctp.PartialReliabilityModeForwardTSN:
		partialReliabilityMode = SCTPTransportPartialReliabilityModeForwardTSN
	case sctp.PartialReliabilityModeIForwardTSN:
		partialReliabilityMode = SCTPTransportPartialReliabilityModeIForwardTSN
	case sctp.PartialReliabilityModeNone:
		partialReliabilityMode = SCTPTransportPartialReliabilityModeNone
	}

	return SCTPTransportMetadata{
		MessageInterleavingEnabled:   metadata.MessageInterleavingEnabled,
		PartialReliabilityMode:       partialReliabilityMode,
		ZeroChecksumSendingEnabled:   metadata.ZeroChecksumSendingEnabled,
		ZeroChecksumReceivingEnabled: metadata.ZeroChecksumReceivingEnabled,
	}
}

// SCTPTransport provides details about the SCTP transport.
type SCTPTransport struct {
	lock sync.RWMutex

	dtlsTransport *DTLSTransport

	// State represents the current state of the SCTP transport.
	state SCTPTransportState

	// SCTPTransportState doesn't have an enum to distinguish between New/Connecting
	// so we need a dedicated field
	isStarted bool

	// MaxChannels represents the maximum amount of DataChannel's that can
	// be used simultaneously.
	maxChannels *uint16

	// OnStateChange  func()

	onErrorHandler func(error)
	onCloseHandler func(error)

	sctpAssociation            *sctp.Association
	onDataChannelHandler       func(*DataChannel)
	onDataChannelOpenedHandler func(*DataChannel)

	// DataChannels
	dataChannels                   []*DataChannel
	dataChannelIDsUsed             map[uint16]uint32
	dataChannelReservations        map[weak.Pointer[DataChannel]]dataChannelReservation
	expiredDataChannelReservations map[dataChannelReservation]uint32
	associationGeneration          uint64
	dataChannelsOpened             uint32
	dataChannelsRequested          uint32
	dataChannelsAccepted           uint32

	localSctpInit []byte

	api *API
	log logging.LeveledLogger
}

// NewSCTPTransport creates a new SCTPTransport.
// This constructor is part of the ORTC API. It is not
// meant to be used together with the basic WebRTC API.
func (api *API) NewSCTPTransport(dtls *DTLSTransport) *SCTPTransport {
	res := &SCTPTransport{
		dtlsTransport:           dtls,
		state:                   SCTPTransportStateConnecting,
		api:                     api,
		log:                     api.settingEngine.LoggerFactory.NewLogger("ortc"),
		dataChannelIDsUsed:      make(map[uint16]uint32),
		dataChannelReservations: make(map[weak.Pointer[DataChannel]]dataChannelReservation),
	}

	res.updateMaxChannels()

	return res
}

// Transport returns the DTLSTransport instance the SCTPTransport is sending over.
func (r *SCTPTransport) Transport() *DTLSTransport {
	r.lock.RLock()
	defer r.lock.RUnlock()

	return r.dtlsTransport
}

// GetCapabilities returns the SCTPCapabilities of the SCTPTransport.
func (r *SCTPTransport) GetCapabilities() SCTPCapabilities {
	var maxMessageSize uint32
	if a := r.association(); a != nil {
		maxMessageSize = a.MaxMessageSize()
	}

	return SCTPCapabilities{
		MaxMessageSize: maxMessageSize,
	}
}

// Start the SCTPTransport. Since both local and remote parties must mutually
// create an SCTPTransport, SCTP SO (Simultaneous Open) is used to establish
// a connection over SCTP.
func (r *SCTPTransport) Start(capabilities SCTPCapabilities) error {
	return r.StartContext(context.Background(), capabilities)
}

// StartContext starts the SCTP transport using the remote capabilities.
// The context controls SCTP association establishment only. Canceling it after
// establishment does not close the association. Cancellation returns the context
// error. If an association is being established, its underlying DTLS connection is
// closed in the background. A failed association setup leaves the transport closed.
// An already-canceled context leaves the transport and DTLS connection unchanged.
//
//nolint:cyclop
func (r *SCTPTransport) StartContext(ctx context.Context, capabilities SCTPCapabilities) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if r.isStarted {
		return nil
	}
	r.isStarted = true

	maxMessageSize := capabilities.MaxMessageSize
	if maxMessageSize == 0 {
		maxMessageSize = sctpMaxMessageSizeUnsetValue
	}
	remoteSctpInit := []byte(capabilities.sctpInit)

	dtlsTransport := r.Transport()
	if dtlsTransport == nil || dtlsTransport.conn == nil {
		return errSCTPTransportDTLS
	}
	opts := r.sctpClientOptions(dtlsTransport.conn, maxMessageSize)
	if len(r.localSctpInit) > 0 && len(remoteSctpInit) > 0 {
		opts = append(
			opts,
			sctp.WithSNAP(r.localSctpInit, remoteSctpInit),
		)
	}
	sctpAssociation, err := sctp.ClientContext(ctx, opts...)
	if err != nil {
		r.lock.Lock()
		r.state = SCTPTransportStateClosed
		r.lock.Unlock()

		return err
	}

	r.setAssociation(sctpAssociation)
	r.lock.RLock()
	dataChannels := append([]*DataChannel{}, r.dataChannels...)
	r.lock.RUnlock()

	var openedDCCount uint32
	for _, d := range dataChannels {
		if d.ReadyState() == DataChannelStateConnecting {
			err := d.open(r)
			if err != nil {
				r.log.Warnf("failed to open data channel: %s", err)

				continue
			}
			openedDCCount++
		}
	}

	r.lock.Lock()
	r.dataChannelsOpened += openedDCCount
	r.lock.Unlock()

	go r.acceptDataChannels(sctpAssociation, dataChannels)

	return nil
}

func (r *SCTPTransport) sctpClientOptions(netConn net.Conn, maxMessageSize uint32) []sctp.ClientOption {
	opts := []sctp.ClientOption{
		sctp.WithNetConn(netConn),
		sctp.WithLoggerFactory(r.api.settingEngine.LoggerFactory),
		sctp.WithMTU(sctpOutboundMTU),
		sctp.WithMaxMessageSize(maxMessageSize),
	}

	return append(opts, r.optionalSCTPClientOptions()...)
}

func (r *SCTPTransport) optionalSCTPClientOptions() []sctp.ClientOption {
	opts := make([]sctp.ClientOption, 0, 7)

	if r.api.settingEngine.sctp.maxReceiveBufferSize != 0 {
		opts = append(opts, sctp.WithMaxReceiveBufferSize(r.api.settingEngine.sctp.maxReceiveBufferSize))
	}

	if r.api.settingEngine.sctp.enableZeroChecksum {
		opts = append(opts, sctp.WithEnableZeroChecksum(true))
	}

	if r.api.settingEngine.detach.DataChannels && r.api.settingEngine.dataChannelBlockWrite {
		opts = append(opts, sctp.WithBlockWrite(true))
	}

	if r.api.settingEngine.sctp.rtoMax > 0 {
		opts = append(
			opts,
			sctp.WithRTOMax(float64(r.api.settingEngine.sctp.rtoMax)/float64(time.Millisecond)),
		)
	}

	if r.api.settingEngine.sctp.minCwnd != 0 {
		opts = append(opts, sctp.WithMinCwnd(r.api.settingEngine.sctp.minCwnd))
	}

	if r.api.settingEngine.sctp.fastRtxWnd != 0 {
		opts = append(opts, sctp.WithFastRtxWnd(r.api.settingEngine.sctp.fastRtxWnd))
	}

	if r.api.settingEngine.sctp.cwndCAStep != 0 {
		opts = append(opts, sctp.WithCwndCAStep(r.api.settingEngine.sctp.cwndCAStep))
	}

	return opts
}

// Stop stops the SCTPTransport.
func (r *SCTPTransport) Stop() error {
	r.lock.Lock()
	association := r.sctpAssociation
	if association == nil {
		r.lock.Unlock()
		return nil
	}
	r.sctpAssociation = nil
	r.state = SCTPTransportStateClosed
	r.lock.Unlock()

	// Abort waits for the association read loop, which may be notifying a restart.
	association.OnAssociationRestart(nil)
	association.Abort("")
	return nil
}

//nolint:cyclop
func (r *SCTPTransport) acceptDataChannels(
	assoc *sctp.Association,
	existingDataChannels []*DataChannel,
) {
	dataChannels := make([]*datachannel.DataChannel, 0, len(existingDataChannels))
	for _, dc := range existingDataChannels {
		dc.mu.Lock()
		isNil := dc.dataChannel == nil
		dc.mu.Unlock()
		if isNil {
			continue
		}
		dataChannels = append(dataChannels, dc.dataChannel)
	}
ACCEPT:
	for {
		// check if the association has been stopped before calling accept.
		r.lock.RLock()
		currentAssoc := r.sctpAssociation
		shouldStop := currentAssoc == nil || currentAssoc != assoc
		r.lock.RUnlock()
		if shouldStop {
			r.onClose(nil)

			return
		}

		stream, err := assoc.AcceptStream()
		var dc *datachannel.DataChannel
		if err == nil {
			stream.SetDefaultPayloadType(sctp.PayloadTypeWebRTCBinary)
			for _, ch := range dataChannels {
				if ch.StreamIdentifier() == stream.StreamIdentifier() {
					continue ACCEPT
				}
			}
			dc, err = datachannel.Server(stream, &datachannel.Config{
				LoggerFactory: r.api.settingEngine.LoggerFactory,
			})
		}
		if err != nil {
			if !errors.Is(err, io.EOF) {
				r.log.Errorf("Failed to accept data channel: %v", err)
				r.onError(err)
				r.onClose(err)
			} else {
				r.onClose(nil)
			}
			return
		}

		var (
			maxRetransmits    *uint16
			maxPacketLifeTime *uint16
		)
		val := uint16(dc.Config.ReliabilityParameter) //nolint:gosec //G115
		ordered := true

		switch dc.Config.ChannelType {
		case datachannel.ChannelTypeReliable:
			ordered = true
		case datachannel.ChannelTypeReliableUnordered:
			ordered = false
		case datachannel.ChannelTypePartialReliableRexmit:
			ordered = true
			maxRetransmits = &val
		case datachannel.ChannelTypePartialReliableRexmitUnordered:
			ordered = false
			maxRetransmits = &val
		case datachannel.ChannelTypePartialReliableTimed:
			ordered = true
			maxPacketLifeTime = &val
		case datachannel.ChannelTypePartialReliableTimedUnordered:
			ordered = false
			maxPacketLifeTime = &val
		default:
		}

		sid := dc.StreamIdentifier()
		rtcDC, err := r.api.newDataChannel(&DataChannelParameters{
			ID:                &sid,
			Label:             dc.Config.Label,
			Protocol:          dc.Config.Protocol,
			Negotiated:        dc.Config.Negotiated,
			Ordered:           ordered,
			MaxPacketLifeTime: maxPacketLifeTime,
			MaxRetransmits:    maxRetransmits,
		}, r, r.api.settingEngine.LoggerFactory.NewLogger("ortc"))
		if err != nil {
			// This data channel is invalid. Close it and log an error.
			if err1 := dc.Close(); err1 != nil {
				r.log.Errorf("Failed to close invalid data channel: %v", err1)
			}
			r.log.Errorf("Failed to accept data channel: %v", err)
			r.onError(err)
			// We've received a datachannel with invalid configuration. We can still receive other datachannels.
			continue ACCEPT
		}

		accepted, err := r.onDataChannel(rtcDC, assoc, stream)
		if err != nil {
			if closeErr := dc.Close(); closeErr != nil {
				r.log.Errorf("Failed to close obsolete data channel: %v", closeErr)
			}

			continue ACCEPT
		}
		<-accepted
		if !r.isDataChannelBound(rtcDC, assoc, stream) {
			continue ACCEPT
		}
		rtcDC.handleOpen(dc, true, dc.Config.Negotiated)

		r.lock.Lock()
		r.dataChannelsOpened++
		handler := r.onDataChannelOpenedHandler
		r.lock.Unlock()

		if handler != nil {
			handler(rtcDC)
		}
	}
}

// OnError sets an event handler which is invoked when the SCTP Association errors.
func (r *SCTPTransport) OnError(f func(err error)) {
	r.lock.Lock()
	defer r.lock.Unlock()
	r.onErrorHandler = f
}

func (r *SCTPTransport) onError(err error) {
	r.lock.RLock()
	handler := r.onErrorHandler
	r.lock.RUnlock()

	if handler != nil {
		go handler(err)
	}
}

// OnClose sets an event handler which is invoked when the SCTP Association closes.
func (r *SCTPTransport) OnClose(f func(err error)) {
	r.lock.Lock()
	defer r.lock.Unlock()
	r.onCloseHandler = f
}

func (r *SCTPTransport) onClose(err error) {
	r.lock.RLock()
	handler := r.onCloseHandler
	r.lock.RUnlock()

	if handler != nil {
		go handler(err)
	}
}

// OnDataChannel sets an event handler which is invoked when a data
// channel message arrives from a remote peer.
func (r *SCTPTransport) OnDataChannel(f func(*DataChannel)) {
	r.lock.Lock()
	defer r.lock.Unlock()
	r.onDataChannelHandler = f
}

// OnDataChannelOpened sets an event handler which is invoked when a data
// channel is opened.
func (r *SCTPTransport) OnDataChannelOpened(f func(*DataChannel)) {
	r.lock.Lock()
	defer r.lock.Unlock()
	r.onDataChannelOpenedHandler = f
}

func (r *SCTPTransport) onDataChannel(dc *DataChannel, association *sctp.Association, stream *sctp.Stream) (done chan struct{}, err error) {
	r.lock.Lock()
	if err = r.bindDataChannelLocked(dc, association, stream); err != nil {
		r.lock.Unlock()

		return nil, err
	}
	r.dataChannels = append(r.dataChannels, dc)
	r.dataChannelsAccepted++
	handler := r.onDataChannelHandler
	r.lock.Unlock()

	done = make(chan struct{})
	if handler == nil || dc == nil {
		close(done)

		return
	}

	// Run this synchronously to allow setup done in onDataChannelFn()
	// to complete before datachannel event handlers might be called.
	go func() {
		handler(dc)
		close(done)
	}()

	return
}

func (r *SCTPTransport) updateMaxChannels() {
	val := sctpMaxChannels
	r.maxChannels = &val
}

// MaxChannels is the maximum number of RTCDataChannels that can be open simultaneously.
func (r *SCTPTransport) MaxChannels() uint16 {
	r.lock.Lock()
	defer r.lock.Unlock()

	if r.maxChannels == nil {
		return sctpMaxChannels
	}

	return *r.maxChannels
}

// State returns the current state of the SCTPTransport.
func (r *SCTPTransport) State() SCTPTransportState {
	r.lock.RLock()
	defer r.lock.RUnlock()

	return r.state
}

// Metadata returns negotiated SCTP association metadata. The ok return value is
// false until the SCTP association has been established.
func (r *SCTPTransport) Metadata() (SCTPTransportMetadata, bool) {
	association := r.association()
	if association == nil {
		return SCTPTransportMetadata{}, false
	}

	metadata, ok := association.Metadata()
	if !ok {
		return SCTPTransportMetadata{}, false
	}

	return newSCTPTransportMetadata(metadata), true
}

// Stats reports the current statistics of the SCTPTransport.
func (r *SCTPTransport) Stats() SCTPTransportStats {
	stats := SCTPTransportStats{
		Timestamp: statsTimestampFrom(time.Now()),
		Type:      StatsTypeSCTPTransport,
		ID:        "sctpTransport",
	}

	association := r.association()
	if association != nil {
		stats.BytesSent = association.BytesSent()
		stats.BytesReceived = association.BytesReceived()
		stats.SmoothedRoundTripTime = association.SRTT() * 0.001 // convert milliseconds to seconds
		stats.CongestionWindow = association.CWND()
		stats.ReceiverWindow = association.RWND()
		stats.MTU = association.MTU()
		if metadata, ok := association.Metadata(); ok {
			transportMetadata := newSCTPTransportMetadata(metadata)
			stats.Metadata = &transportMetadata
		}
	}

	return stats
}

func (r *SCTPTransport) collectStats(collector *statsReportCollector) {
	collector.Collecting()
	stats := r.Stats()
	collector.Collect(stats.ID, stats)
}

// setAssociation installs the observer before opening any data channels.
func (r *SCTPTransport) setAssociation(association *sctp.Association) {
	r.lock.Lock()
	defer r.lock.Unlock()
	r.sctpAssociation = association
	r.state = SCTPTransportStateConnected
	r.associationGeneration = 0
	association.OnAssociationRestart(func(event sctp.AssociationRestartEvent) {
		r.onAssociationRestart(association, event)
	})
}

// The caller must hold r.lock.
func (r *SCTPTransport) setMaxChannels(inbound, outbound uint16) {
	value := min(inbound, outbound)
	r.maxChannels = &value
}

func (r *SCTPTransport) onAssociationRestart(association *sctp.Association, event sctp.AssociationRestartEvent) {
	r.lock.Lock()
	defer r.lock.Unlock()
	if r.sctpAssociation != association || r.state != SCTPTransportStateConnected ||
		event.Generation <= r.associationGeneration {
		return
	}
	r.pruneDataChannelReservations()
	r.associationGeneration = event.Generation
	r.setMaxChannels(event.NumInboundStreams, event.NumOutboundStreams)
	retained := make(map[*sctp.Stream]struct{}, len(event.RetainedStreams))
	for _, stream := range event.RetainedStreams {
		retained[stream] = struct{}{}
	}
	for channel, reservation := range r.dataChannelReservations {
		if reservation.association.Value() != association || reservation.generation >= event.Generation {
			continue
		}
		if _, ok := retained[reservation.stream.Value()]; ok {
			reservation.generation = event.Generation
			r.dataChannelReservations[channel] = reservation

			continue
		}
		r.releaseDataChannelReservation(channel)
	}
	r.releaseExpiredDataChannelReservations(association, event.Generation)
}

// The caller must hold r.lock.
func (r *SCTPTransport) releaseExpiredDataChannelReservations(association *sctp.Association, generation uint64) {
	for reservation, count := range r.expiredDataChannelReservations {
		if reservation.association.Value() == association && reservation.generation < generation {
			r.releaseDataChannelIDCount(reservation.id, count)
			delete(r.expiredDataChannelReservations, reservation)
		}
	}
}

// The caller must hold r.lock. Coalesce expired bindings so the transport does
// not accumulate one entry per collected detached channel between restarts.
// Counts remain reserved until a restart; normal reset reuse is separate.
func (r *SCTPTransport) pruneDataChannelReservations() {
	for channel, reservation := range r.dataChannelReservations {
		if reservation.stream.Value() != nil || channel.Value() != nil {
			continue
		}
		if r.expiredDataChannelReservations == nil {
			r.expiredDataChannelReservations = make(map[dataChannelReservation]uint32)
		}
		reservation.stream = weak.Pointer[sctp.Stream]{}
		r.expiredDataChannelReservations[reservation]++
		delete(r.dataChannelReservations, channel)
	}
}

// The caller must hold r.lock. Pending opens keep their reservations across a restart.
func (r *SCTPTransport) reserveDataChannelID(channel *DataChannel, id uint16) {
	if r.dataChannelReservations == nil {
		r.dataChannelReservations = make(map[weak.Pointer[DataChannel]]dataChannelReservation)
	}
	r.pruneDataChannelReservations()
	key := weak.Make(channel)
	if _, ok := r.dataChannelReservations[key]; ok {
		return
	}
	r.dataChannelReservations[key] = dataChannelReservation{id: id}
	r.dataChannelIDsUsed[id]++
}

// The caller must hold r.lock. Release this owner only, including when IDs overlap.
func (r *SCTPTransport) releaseDataChannelReservation(channel weak.Pointer[DataChannel]) {
	reservation, ok := r.dataChannelReservations[channel]
	if !ok {
		return
	}
	delete(r.dataChannelReservations, channel)
	r.releaseDataChannelIDCount(reservation.id, 1)
}

// The caller must hold r.lock.
func (r *SCTPTransport) releaseDataChannelIDCount(id uint16, count uint32) {
	if r.dataChannelIDsUsed[id] <= count {
		delete(r.dataChannelIDsUsed, id)
	} else {
		r.dataChannelIDsUsed[id] -= count
	}
}

func (r *SCTPTransport) bindDataChannel(channel *DataChannel, association *sctp.Association, stream *sctp.Stream) error {
	r.lock.Lock()
	defer r.lock.Unlock()

	return r.bindDataChannelLocked(channel, association, stream)
}

func (r *SCTPTransport) isDataChannelBound(channel *DataChannel, association *sctp.Association, stream *sctp.Stream) bool {
	r.lock.RLock()
	defer r.lock.RUnlock()
	reservation, ok := r.dataChannelReservations[weak.Make(channel)]

	return ok && r.sctpAssociation == association && r.state == SCTPTransportStateConnected &&
		reservation.association.Value() == association && reservation.stream.Value() == stream && stream.State() == sctp.StreamStateOpen
}

// The caller must hold r.lock.
func (r *SCTPTransport) bindDataChannelLocked(channel *DataChannel, association *sctp.Association, stream *sctp.Stream) error {
	if stream == nil {
		r.releaseDataChannelReservation(weak.Make(channel))

		return io.ErrClosedPipe
	}
	// Capture the generation before checking state. A discarded stream also
	// advances its generation during restart, but is no longer open.
	generation := stream.AssociationGeneration()
	if r.sctpAssociation != association || r.state != SCTPTransportStateConnected || stream.State() != sctp.StreamStateOpen {
		r.releaseDataChannelReservation(weak.Make(channel))

		return io.ErrClosedPipe
	}
	if !r.isDataChannelBindingWithinLimit(channel, association, stream.StreamIdentifier(), stream) {
		r.releaseDataChannelReservation(weak.Make(channel))

		return &rtcerr.OperationError{Err: ErrMaxDataChannelID}
	}
	r.reserveDataChannelID(channel, stream.StreamIdentifier())
	key := weak.Make(channel)
	reservation := r.dataChannelReservations[key]
	reservation.association = weak.Make(association)
	reservation.stream = weak.Make(stream)
	reservation.generation = generation
	r.dataChannelReservations[key] = reservation

	return nil
}

func (r *SCTPTransport) generateAndSetDataChannelID(dtlsRole DTLSRole, idOut **uint16, channel *DataChannel) error {
	r.lock.Lock()
	defer r.lock.Unlock()
	maxVal := sctpMaxChannels
	if r.maxChannels != nil {
		maxVal = *r.maxChannels
	}
	var firstID uint32
	if dtlsRole != DTLSRoleClient {
		firstID = 1
	}
	// Use a wider counter to avoid wraparound at the maximum stream count.
	for candidate := firstID; candidate < uint32(maxVal); candidate += 2 {
		id := uint16(candidate)
		if r.dataChannelIDsUsed[id] != 0 {
			continue
		}
		*idOut = &id
		r.reserveDataChannelID(channel, id)
		return nil
	}
	return &rtcerr.OperationError{Err: ErrMaxDataChannelID}
}

func (r *SCTPTransport) association() *sctp.Association {
	if r == nil {
		return nil
	}
	r.lock.RLock()
	association := r.sctpAssociation
	r.lock.RUnlock()

	return association
}

// BufferedAmount returns total amount (in bytes) of currently buffered user data.
func (r *SCTPTransport) BufferedAmount() int {
	r.lock.Lock()
	defer r.lock.Unlock()
	if r.sctpAssociation == nil {
		return 0
	}

	return r.sctpAssociation.BufferedAmount()
}

// GetSctpInit returns the current sctp-init attribute and caches the last created.
// The caller should hold the lock.
func (r *SCTPTransport) GetSctpInit() []byte {
	if len(r.localSctpInit) == 0 {
		var err error
		r.localSctpInit, err = sctp.GenerateOutOfBandToken(sctp.Config{
			MaxReceiveBufferSize: r.api.settingEngine.sctp.maxReceiveBufferSize,
			EnableZeroChecksum:   r.api.settingEngine.sctp.enableZeroChecksum,
		})
		if err != nil {
			r.log.Warnf("Failed to create sctp-init: %v", err)
		}
	}

	return r.localSctpInit
}

// The caller must hold r.lock. A retained binding can outlive a reduced limit,
// but a new owner, including a previously allocated unbound ID, cannot exceed it.
func (r *SCTPTransport) isDataChannelBindingWithinLimit(channel *DataChannel, association *sctp.Association, id uint16, stream *sctp.Stream) bool {
	limit := sctpMaxChannels
	if r.maxChannels != nil {
		limit = *r.maxChannels
	}
	if id < limit {
		return true
	}
	reservation, ok := r.dataChannelReservations[weak.Make(channel)]
	boundStream := reservation.stream.Value()

	return ok && reservation.id == id && reservation.association.Value() == association && boundStream != nil &&
		(stream == nil || boundStream == stream)
}

// validateDataChannelID rejects an already invalid local open before creating
// its SCTP stream. Binding rechecks the limit if a restart races OpenStream.
func (r *SCTPTransport) validateDataChannelID(channel *DataChannel, association *sctp.Association, id uint16) error {
	r.lock.Lock()
	defer r.lock.Unlock()
	if r.isDataChannelBindingWithinLimit(channel, association, id, nil) {
		return nil
	}
	r.releaseDataChannelReservation(weak.Make(channel))

	return &rtcerr.OperationError{Err: ErrMaxDataChannelID}
}

// releaseFailedDataChannelBinding releases only the owner whose open failed.
// Constructor and PeerConnection failure cleanup may safely run afterwards.
func (r *SCTPTransport) releaseFailedDataChannelBinding(channel *DataChannel) {
	r.lock.Lock()
	defer r.lock.Unlock()
	r.releaseDataChannelReservation(weak.Make(channel))
}
