// +build !js

package webrtc

import (
	"errors"
	"io"
	"math"
	"sync"
	"time"

	"github.com/pion/datachannel"
	"github.com/pion/logging"
	"github.com/pion/sctp"
)

const sctpMaxChannels = uint16(65535)

// SCTPTransport provides details about the SCTP transport.
type SCTPTransport struct {
	lock sync.RWMutex

	dtlsTransport *DTLSTransport

	// State represents the current state of the SCTP transport.
	state SCTPTransportState

	// MaxMessageSize represents the maximum size of data that can be passed to
	// DataChannel's send() method.
	maxMessageSize float64

	// MaxChannels represents the maximum amount of DataChannel's that can
	// be used simultaneously.
	maxChannels *uint16

	// OnStateChange  func()

	association                *sctp.Association
	onDataChannelHandler       func(*DataChannel)
	onDataChannelOpenedHandler func(*DataChannel)

	api *API
	log logging.LeveledLogger
}

// NewSCTPTransport creates a new SCTPTransport.
// This constructor is part of the ORTC API. It is not
// meant to be used together with the basic WebRTC API.
func (api *API) NewSCTPTransport(dtls *DTLSTransport) *SCTPTransport {
	res := &SCTPTransport{
		dtlsTransport: dtls,
		state:         SCTPTransportStateConnecting,
		api:           api,
		log:           api.settingEngine.LoggerFactory.NewLogger("ortc"),
	}

	res.updateMessageSize()
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
	return SCTPCapabilities{
		MaxMessageSize: 0,
	}
}

// Start the SCTPTransport. Since both local and remote parties must mutually
// create an SCTPTransport, SCTP SO (Simultaneous Open) is used to establish
// a connection over SCTP.
func (r *SCTPTransport) Start(remoteCaps SCTPCapabilities) error {
	r.lock.Lock()
	defer r.lock.Unlock()

	if err := r.ensureDTLS(); err != nil {
		return err
	}

	sctpAssociation, err := sctp.Client(sctp.Config{
		NetConn:       r.dtlsTransport.conn,
		LoggerFactory: r.api.settingEngine.LoggerFactory,
	})
	if err != nil {
		return err
	}
	r.association = sctpAssociation

	go r.acceptDataChannels(sctpAssociation)

	return nil
}

// Stop stops the SCTPTransport
func (r *SCTPTransport) Stop() error {
	r.lock.Lock()
	defer r.lock.Unlock()
	if r.association == nil {
		return nil
	}
	err := r.association.Close()
	if err != nil {
		return err
	}

	r.association = nil
	r.state = SCTPTransportStateClosed

	return nil
}

func (r *SCTPTransport) ensureDTLS() error {
	if r.dtlsTransport == nil ||
		r.dtlsTransport.conn == nil {
		return errors.New("DTLS not establisched")
	}

	return nil
}

func (r *SCTPTransport) acceptDataChannels(a *sctp.Association) {
	for {
		dc, err := datachannel.Accept(a, &datachannel.Config{
			LoggerFactory: r.api.settingEngine.LoggerFactory,
		})
		if err != nil {
			if err != io.EOF {
				r.log.Errorf("Failed to accept data channel: %v", err)
				// pion/webrtc#754
			}
			return
		}

		var ordered = true
		var maxRetransmits *uint16
		var maxPacketLifeTime *uint16
		var val = uint16(dc.Config.ReliabilityParameter)

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
			ID:                sid,
			Label:             dc.Config.Label,
			Protocol:          dc.Config.Protocol,
			Ordered:           ordered,
			MaxPacketLifeTime: maxPacketLifeTime,
			MaxRetransmits:    maxRetransmits,
		}, r.api.settingEngine.LoggerFactory.NewLogger("ortc"))

		if err != nil {
			r.log.Errorf("Failed to accept data channel: %v", err)
			// pion/webrtc#754
			return
		}

		<-r.onDataChannel(rtcDC)
		rtcDC.handleOpen(dc)

		r.lock.Lock()
		dcOpenedHdlr := r.onDataChannelOpenedHandler
		r.lock.Unlock()

		if dcOpenedHdlr != nil {
			dcOpenedHdlr(rtcDC)
		}
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
// channel is opened
func (r *SCTPTransport) OnDataChannelOpened(f func(*DataChannel)) {
	r.lock.Lock()
	defer r.lock.Unlock()
	r.onDataChannelOpenedHandler = f
}

func (r *SCTPTransport) onDataChannel(dc *DataChannel) (done chan struct{}) {
	r.lock.Lock()
	hdlr := r.onDataChannelHandler
	r.lock.Unlock()

	done = make(chan struct{})
	if hdlr == nil || dc == nil {
		close(done)
		return
	}

	// Run this synchronously to allow setup done in onDataChannelFn()
	// to complete before datachannel event handlers might be called.
	go func() {
		hdlr(dc)
		close(done)
	}()

	return
}

func (r *SCTPTransport) updateMessageSize() {
	r.lock.Lock()
	defer r.lock.Unlock()

	var remoteMaxMessageSize float64 = 65536 // pion/webrtc#758
	var canSendSize float64 = 65536          // pion/webrtc#758

	r.maxMessageSize = r.calcMessageSize(remoteMaxMessageSize, canSendSize)
}

func (r *SCTPTransport) calcMessageSize(remoteMaxMessageSize, canSendSize float64) float64 {
	switch {
	case remoteMaxMessageSize == 0 &&
		canSendSize == 0:
		return math.Inf(1)

	case remoteMaxMessageSize == 0:
		return canSendSize

	case canSendSize == 0:
		return remoteMaxMessageSize

	case canSendSize > remoteMaxMessageSize:
		return remoteMaxMessageSize

	default:
		return canSendSize
	}
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

// State returns the current state of the SCTPTransport
func (r *SCTPTransport) State() SCTPTransportState {
	r.lock.RLock()
	defer r.lock.RLock()
	return r.state
}

func (r *SCTPTransport) collectStats(collector *statsReportCollector) {
	r.lock.Lock()
	association := r.association
	r.lock.Unlock()

	collector.Collecting()

	stats := TransportStats{
		Timestamp: statsTimestampFrom(time.Now()),
		Type:      StatsTypeTransport,
		ID:        "sctpTransport",
	}

	if association != nil {
		stats.BytesSent = association.BytesSent()
		stats.BytesReceived = association.BytesReceived()
	}

	collector.Collect(stats.ID, stats)
}
