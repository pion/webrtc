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

// RTCSctpTransport provides details about the SCTP transport.
type RTCSctpTransport struct {
	lock sync.RWMutex

	dtlsTransport *RTCDtlsTransport

	// State represents the current state of the SCTP transport.
	State RTCSctpTransportState

	port uint16

	// MaxMessageSize represents the maximum size of data that can be passed to
	// RTCDataChannel's send() method.
	MaxMessageSize float64

	// MaxChannels represents the maximum amount of RTCDataChannel's that can
	// be used simultaneously.
	MaxChannels *uint16

	// OnStateChange  func()

	association          *sctp.Association
	onDataChannelHandler func(*RTCDataChannel)
}

// NewRTCSctpTransport creates a new RTCSctpTransport.
// This constructor is part of the ORTC API. It is not
// meant to be used together with the basic WebRTC API.
func NewRTCSctpTransport(dtls *RTCDtlsTransport) *RTCSctpTransport {
	res := &RTCSctpTransport{
		dtlsTransport: dtls,
		State:         RTCSctpTransportStateConnecting,
		port:          5000, // TODO
	}

	res.updateMessageSize()
	res.updateMaxChannels()

	return res
}

// Transport returns the RTCDtlsTransport instance the RTCSctpTransport is sending over.
func (r *RTCSctpTransport) Transport() *RTCDtlsTransport {
	r.lock.RLock()
	defer r.lock.RUnlock()

	return r.dtlsTransport
}

// GetCapabilities returns the RTCSctpCapabilities of the RTCSctpTransport.
func (r *RTCSctpTransport) GetCapabilities() RTCSctpCapabilities {
	return RTCSctpCapabilities{
		MaxMessageSize: 0,
	}
}

// Start the RTCSctpTransport. Since both local and remote parties must mutually
// create an RTCSctpTransport, SCTP SO (Simultaneous Open) is used to establish
// a connection over SCTP.
func (r *RTCSctpTransport) Start(remoteCaps RTCSctpCapabilities) error {
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

// Stop stops the RTCSctpTransport
func (r *RTCSctpTransport) Stop() error {
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
	r.State = RTCSctpTransportStateClosed

	return nil
}

func (r *RTCSctpTransport) ensureDTLS() error {
	if r.dtlsTransport == nil ||
		r.dtlsTransport.conn == nil {
		return errors.New("DTLS not establisched")
	}

	return nil
}

func (r *RTCSctpTransport) acceptDataChannels() {
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

		sid := dc.StreamIdentifier()
		rtcDC := &RTCDataChannel{
			ID:            &sid,
			Label:         dc.Config.Label,
			ReadyState:    RTCDataChannelStateOpen,
			settingEngine: defaultSettingEngine,
		}

		<-r.onDataChannel(rtcDC)
		rtcDC.handleOpen(dc)
	}
}

// OnDataChannel sets an event handler which is invoked when a data
// channel message arrives from a remote peer.
func (r *RTCSctpTransport) OnDataChannel(f func(*RTCDataChannel)) {
	r.lock.Lock()
	defer r.lock.Unlock()
	r.onDataChannelHandler = f
}

func (r *RTCSctpTransport) onDataChannel(dc *RTCDataChannel) (done chan struct{}) {
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

func (r *RTCSctpTransport) updateMessageSize() {
	var remoteMaxMessageSize float64 = 65536 // TODO: get from SDP
	var canSendSize float64 = 65536          // TODO: Get from SCTP implementation

	r.MaxMessageSize = r.calcMessageSize(remoteMaxMessageSize, canSendSize)
}

func (r *RTCSctpTransport) calcMessageSize(remoteMaxMessageSize, canSendSize float64) float64 {
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

func (r *RTCSctpTransport) updateMaxChannels() {
	val := sctpMaxChannels
	r.MaxChannels = &val
}
