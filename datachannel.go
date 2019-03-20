// +build !js

package webrtc

import (
	"fmt"
	"io"
	"sync"

	"github.com/pions/datachannel"
	"github.com/pions/logging"
	"github.com/pions/webrtc/pkg/rtcerr"
)

const dataChannelBufferSize = 16384 // Lowest common denominator among browsers

// DataChannel represents a WebRTC DataChannel
// The DataChannel interface represents a network channel
// which can be used for bidirectional peer-to-peer transfers of arbitrary data
type DataChannel struct {
	mu sync.RWMutex

	label                      string
	ordered                    bool
	maxPacketLifeTime          *uint16
	maxRetransmits             *uint16
	protocol                   string
	negotiated                 bool
	id                         *uint16
	priority                   PriorityType
	readyState                 DataChannelState
	bufferedAmountLowThreshold uint64

	// The binaryType represents attribute MUST, on getting, return the value to
	// which it was last set. On setting, if the new value is either the string
	// "blob" or the string "arraybuffer", then set the IDL attribute to this
	// new value. Otherwise, throw a SyntaxError. When an DataChannel object
	// is created, the binaryType attribute MUST be initialized to the string
	// "blob". This attribute controls how binary data is exposed to scripts.
	// binaryType                 string

	// OnBufferedAmountLow func()
	// OnError             func()

	onMessageHandler func(DataChannelMessage)
	onOpenHandler    func()
	onCloseHandler   func()

	sctpTransport *SCTPTransport
	dataChannel   *datachannel.DataChannel

	// A reference to the associated api object used by this datachannel
	api *API
	log *logging.LeveledLogger
}

// NewDataChannel creates a new DataChannel.
// This constructor is part of the ORTC API. It is not
// meant to be used together with the basic WebRTC API.
func (api *API) NewDataChannel(transport *SCTPTransport, params *DataChannelParameters) (*DataChannel, error) {
	d, err := api.newDataChannel(params, logging.NewScopedLogger("ortc"))
	if err != nil {
		return nil, err
	}

	err = d.open(transport)
	if err != nil {
		return nil, err
	}

	return d, nil
}

// newDataChannel is an internal constructor for the data channel used to
// create the DataChannel object before the networking is set up.
func (api *API) newDataChannel(params *DataChannelParameters, log *logging.LeveledLogger) (*DataChannel, error) {
	// https://w3c.github.io/webrtc-pc/#peer-to-peer-data-api (Step #5)
	if len(params.Label) > 65535 {
		return nil, &rtcerr.TypeError{Err: ErrStringSizeLimit}
	}

	return &DataChannel{
		label:             params.Label,
		id:                &params.ID,
		ordered:           params.Ordered,
		maxPacketLifeTime: params.MaxPacketLifeTime,
		maxRetransmits:    params.MaxRetransmits,
		readyState:        DataChannelStateConnecting,
		api:               api,
		log:               log,
	}, nil
}

// open opens the datachannel over the sctp transport
func (d *DataChannel) open(sctpTransport *SCTPTransport) error {
	d.mu.Lock()
	d.sctpTransport = sctpTransport

	if err := d.ensureSCTP(); err != nil {
		d.mu.Unlock()
		return err
	}

	var channelType datachannel.ChannelType
	var reliabilityParameteer uint32

	switch {
	case d.maxPacketLifeTime == nil && d.maxRetransmits == nil:
		if d.ordered {
			channelType = datachannel.ChannelTypeReliable
		} else {
			channelType = datachannel.ChannelTypeReliableUnordered
		}

	case d.maxRetransmits != nil:
		reliabilityParameteer = uint32(*d.maxRetransmits)
		if d.ordered {
			channelType = datachannel.ChannelTypePartialReliableRexmit
		} else {
			channelType = datachannel.ChannelTypePartialReliableRexmitUnordered
		}
	default:
		reliabilityParameteer = uint32(*d.maxPacketLifeTime)
		if d.ordered {
			channelType = datachannel.ChannelTypePartialReliableTimed
		} else {
			channelType = datachannel.ChannelTypePartialReliableTimedUnordered
		}
	}

	cfg := &datachannel.Config{
		ChannelType:          channelType,
		Priority:             datachannel.ChannelPriorityNormal, // TODO: Wiring
		ReliabilityParameter: reliabilityParameteer,
		Label:                d.label,
	}

	dc, err := datachannel.Dial(d.sctpTransport.association, *d.id, cfg)
	if err != nil {
		d.mu.Unlock()
		return err
	}

	d.readyState = DataChannelStateOpen
	d.mu.Unlock()

	d.handleOpen(dc)
	return nil
}

func (d *DataChannel) ensureSCTP() error {
	if d.sctpTransport == nil ||
		d.sctpTransport.association == nil {
		return fmt.Errorf("SCTP not establisched")
	}
	return nil
}

// Transport returns the SCTPTransport instance the DataChannel is sending over.
func (d *DataChannel) Transport() *SCTPTransport {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return d.sctpTransport
}

// OnOpen sets an event handler which is invoked when
// the underlying data transport has been established (or re-established).
func (d *DataChannel) OnOpen(f func()) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.onOpenHandler = f
}

func (d *DataChannel) onOpen() (done chan struct{}) {
	d.mu.RLock()
	hdlr := d.onOpenHandler
	d.mu.RUnlock()

	done = make(chan struct{})
	if hdlr == nil {
		close(done)
		return
	}

	go func() {
		hdlr()
		close(done)
	}()

	return
}

// OnClose sets an event handler which is invoked when
// the underlying data transport has been closed.
func (d *DataChannel) OnClose(f func()) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.onCloseHandler = f
}

func (d *DataChannel) onClose() (done chan struct{}) {
	d.mu.RLock()
	hdlr := d.onCloseHandler
	d.mu.RUnlock()

	done = make(chan struct{})
	if hdlr == nil {
		close(done)
		return
	}

	go func() {
		hdlr()
		close(done)
	}()

	return
}

// OnMessage sets an event handler which is invoked on a binary
// message arrival over the sctp transport from a remote peer.
// OnMessage can currently receive messages up to 16384 bytes
// in size. Check out the detach API if you want to use larger
// message sizes. Note that browser support for larger messages
// is also limited.
func (d *DataChannel) OnMessage(f func(msg DataChannelMessage)) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.onMessageHandler = f
}

func (d *DataChannel) onMessage(msg DataChannelMessage) {
	d.mu.RLock()
	hdlr := d.onMessageHandler
	d.mu.RUnlock()

	if hdlr == nil {
		return
	}
	hdlr(msg)
}

func (d *DataChannel) handleOpen(dc *datachannel.DataChannel) {
	d.mu.Lock()
	d.dataChannel = dc
	d.mu.Unlock()

	d.onOpen()

	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.api.settingEngine.detach.DataChannels {
		go d.readLoop()
	}
}

func (d *DataChannel) readLoop() {
	for {
		buffer := make([]byte, dataChannelBufferSize)
		n, isString, err := d.dataChannel.ReadDataChannel(buffer)
		if err == io.ErrShortBuffer {
			d.log.Warnf("Failed to read from data channel: The message is larger than %d bytes.\n", dataChannelBufferSize)
			continue
		}
		if err != nil {
			d.mu.Lock()
			d.readyState = DataChannelStateClosed
			d.mu.Unlock()
			if err != io.EOF {
				// TODO: Throw OnError
				fmt.Println("Failed to read from data channel", err)
			}
			d.onClose()
			return
		}

		d.onMessage(DataChannelMessage{Data: buffer[:n], IsString: isString})
	}
}

// Send sends the binary message to the DataChannel peer
func (d *DataChannel) Send(data []byte) error {
	err := d.ensureOpen()
	if err != nil {
		return err
	}

	if len(data) == 0 {
		data = []byte{0}
	}

	_, err = d.dataChannel.WriteDataChannel(data, false)
	return err
}

// SendText sends the text message to the DataChannel peer
func (d *DataChannel) SendText(s string) error {
	err := d.ensureOpen()
	if err != nil {
		return err
	}

	data := []byte(s)
	if len(data) == 0 {
		data = []byte{0}
	}

	_, err = d.dataChannel.WriteDataChannel(data, true)
	return err
}

func (d *DataChannel) ensureOpen() error {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if d.readyState != DataChannelStateOpen {
		return &rtcerr.InvalidStateError{Err: ErrDataChannelNotOpen}
	}
	return nil
}

// Detach allows you to detach the underlying datachannel. This provides
// an idiomatic API to work with, however it disables the OnMessage callback.
// Before calling Detach you have to enable this behavior by calling
// webrtc.DetachDataChannels(). Combining detached and normal data channels
// is not supported.
// Please reffer to the data-channels-detach example and the
// pions/datachannel documentation for the correct way to handle the
// resulting DataChannel object.
func (d *DataChannel) Detach() (*datachannel.DataChannel, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.api.settingEngine.detach.DataChannels {
		return nil, fmt.Errorf("enable detaching by calling webrtc.DetachDataChannels()")
	}

	if d.dataChannel == nil {
		return nil, fmt.Errorf("datachannel not opened yet, try calling Detach from OnOpen")
	}

	return d.dataChannel, nil
}

// Close Closes the DataChannel. It may be called regardless of whether
// the DataChannel object was created by this peer or the remote peer.
func (d *DataChannel) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.readyState == DataChannelStateClosing ||
		d.readyState == DataChannelStateClosed {
		return nil
	}

	d.readyState = DataChannelStateClosing

	return d.dataChannel.Close()
}

// Label represents a label that can be used to distinguish this
// DataChannel object from other DataChannel objects. Scripts are
// allowed to create multiple DataChannel objects with the same label.
func (d *DataChannel) Label() string {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return d.label
}

// Ordered represents if the DataChannel is ordered, and false if
// out-of-order delivery is allowed.
func (d *DataChannel) Ordered() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return d.ordered
}

// MaxPacketLifeTime represents the length of the time window (msec) during
// which transmissions and retransmissions may occur in unreliable mode.
func (d *DataChannel) MaxPacketLifeTime() *uint16 {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return d.maxPacketLifeTime
}

// MaxRetransmits represents the maximum number of retransmissions that are
// attempted in unreliable mode.
func (d *DataChannel) MaxRetransmits() *uint16 {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return d.maxRetransmits
}

// Protocol represents the name of the sub-protocol used with this
// DataChannel.
func (d *DataChannel) Protocol() string {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return d.protocol
}

// Negotiated represents whether this DataChannel was negotiated by the
// application (true), or not (false).
func (d *DataChannel) Negotiated() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return d.negotiated
}

// ID represents the ID for this DataChannel. The value is initially
// null, which is what will be returned if the ID was not provided at
// channel creation time, and the DTLS role of the SCTP transport has not
// yet been negotiated. Otherwise, it will return the ID that was either
// selected by the script or generated. After the ID is set to a non-null
// value, it will not change.
func (d *DataChannel) ID() *uint16 {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return d.id
}

// Priority represents the priority for this DataChannel. The priority is
// assigned at channel creation time.
func (d *DataChannel) Priority() PriorityType {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return d.priority
}

// ReadyState represents the state of the DataChannel object.
func (d *DataChannel) ReadyState() DataChannelState {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return d.readyState
}

// BufferedAmount represents the number of bytes of application data
// (UTF-8 text and binary data) that have been queued using send(). Even
// though the data transmission can occur in parallel, the returned value
// MUST NOT be decreased before the current task yielded back to the event
// loop to prevent race conditions. The value does not include framing
// overhead incurred by the protocol, or buffering done by the operating
// system or network hardware. The value of BufferedAmount slot will only
// increase with each call to the send() method as long as the ReadyState is
// open; however, BufferedAmount does not reset to zero once the channel
// closes.
func (d *DataChannel) BufferedAmount() uint64 {
	d.mu.RLock()
	defer d.mu.RUnlock()

	// TODO: wire to SCTP (pions/sctp#11)
	return 0
}

// BufferedAmountLowThreshold represents the threshold at which the
// bufferedAmount is considered to be low. When the bufferedAmount decreases
// from above this threshold to equal or below it, the bufferedamountlow
// event fires. BufferedAmountLowThreshold is initially zero on each new
// DataChannel, but the application may change its value at any time.
func (d *DataChannel) BufferedAmountLowThreshold() uint64 {
	d.mu.RLock()
	defer d.mu.RUnlock()

	// TODO: wire to SCTP (pions/sctp#11)
	return d.bufferedAmountLowThreshold
}

// SetBufferedAmountLowThreshold represents the threshold at which the
// bufferedAmount is considered to be low. When the bufferedAmount decreases
// from above this threshold to equal or below it, the bufferedamountlow
// event fires. BufferedAmountLowThreshold is initially zero on each new
// DataChannel, but the application may change its value at any time.
func (d *DataChannel) SetBufferedAmountLowThreshold(th uint64) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// TODO: wire to SCTP (pions/sctp#11)
	d.bufferedAmountLowThreshold = th
}
