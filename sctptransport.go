// +build !js

package webrtc

import (
	"errors"
	"fmt"
	"math"
	"sync"

	"github.com/pions/datachannel"
	"github.com/pions/sctp"
)

const sctpMaxChannels = uint16(65535)

// SCTPTransport provides details about the SCTP transport.
type SCTPTransport struct {
	lock sync.RWMutex

	dtlsTransport *DTLSTransport

	// State represents the current state of the SCTP transport.
	State SCTPTransportState

	port uint16

	// MaxMessageSize represents the maximum size of data that can be passed to
	// DataChannel's send() method.
	MaxMessageSize float64

	// MaxChannels represents the maximum amount of DataChannel's that can
	// be used simultaneously.
	MaxChannels *uint16

	// OnStateChange  func()

	association          *sctp.Association
	onDataChannelHandler func(*DataChannel)

	api *API
}

// NewSCTPTransport creates a new SCTPTransport.
// This constructor is part of the ORTC API. It is not
// meant to be used together with the basic WebRTC API.
func (api *API) NewSCTPTransport(dtls *DTLSTransport) *SCTPTransport {
	res := &SCTPTransport{
		dtlsTransport: dtls,
		State:         SCTPTransportStateConnecting,
		port:          5000, // TODO
		api:           api,
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

	// TODO: port
	_ = r.MaxMessageSize // TODO

	if err := r.ensureDTLS(); err != nil {
		return err
	}

	sctpAssociation, err := sctp.Client(r.dtlsTransport.conn)
	if err != nil {
		return err
	}
	r.association = sctpAssociation

	go r.acceptDataChannels()

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
	r.State = SCTPTransportStateClosed

	return nil
}

func (r *SCTPTransport) ensureDTLS() error {
	if r.dtlsTransport == nil ||
		r.dtlsTransport.conn == nil {
		return errors.New("DTLS not establisched")
	}

	return nil
}

func (r *SCTPTransport) acceptDataChannels() {
	r.lock.RLock()
	a := r.association
	r.lock.RUnlock()
	for {
		dc, err := datachannel.Accept(a)
		if err != nil {
			fmt.Println("Failed to accept data channel:", err)
			// TODO: Kill DataChannel/PeerConnection?
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
		rtcDC := &DataChannel{
			id:                &sid,
			label:             dc.Config.Label,
			ordered:           ordered,
			maxPacketLifeTime: maxPacketLifeTime,
			maxRetransmits:    maxRetransmits,
			readyState:        DataChannelStateOpen,
			api:               r.api,
		}

		<-r.onDataChannel(rtcDC)
		rtcDC.handleOpen(dc)
	}
}

// OnDataChannel sets an event handler which is invoked when a data
// channel message arrives from a remote peer.
func (r *SCTPTransport) OnDataChannel(f func(*DataChannel)) {
	r.lock.Lock()
	defer r.lock.Unlock()
	r.onDataChannelHandler = f
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
	var remoteMaxMessageSize float64 = 65536 // TODO: get from SDP
	var canSendSize float64 = 65536          // TODO: Get from SCTP implementation

	r.MaxMessageSize = r.calcMessageSize(remoteMaxMessageSize, canSendSize)
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
	r.MaxChannels = &val
}
