package webrtc

import (
	"fmt"
	"sync"

	"github.com/pions/datachannel"
	sugar "github.com/pions/webrtc/pkg/datachannel"
	"github.com/pkg/errors"
)

const receiveMTU = 8192

// RTCDataChannel represents a WebRTC DataChannel
// The RTCDataChannel interface represents a network channel
// which can be used for bidirectional peer-to-peer transfers of arbitrary data
type RTCDataChannel struct {
	sync.RWMutex

	// Transport represents the associated underlying data transport that is
	// used to transport actual data to the other peer.
	Transport *RTCSctpTransport

	// Label represents a label that can be used to distinguish this
	// RTCDataChannel object from other RTCDataChannel objects. Scripts are
	// allowed to create multiple RTCDataChannel objects with the same label.
	Label string

	// Ordered represents if the RTCDataChannel is ordered, and false if
	// out-of-order delivery is allowed.
	Ordered bool

	// MaxPacketLifeTime represents the length of the time window (msec) during
	// which transmissions and retransmissions may occur in unreliable mode.
	MaxPacketLifeTime *uint16

	// MaxRetransmits represents the maximum number of retransmissions that are
	// attempted in unreliable mode.
	MaxRetransmits *uint16

	// Protocol represents the name of the sub-protocol used with this
	// RTCDataChannel.
	Protocol string

	// Negotiated represents whether this RTCDataChannel was negotiated by the
	// application (true), or not (false).
	Negotiated bool

	// ID represents the ID for this RTCDataChannel. The value is initially
	// null, which is what will be returned if the ID was not provided at
	// channel creation time, and the DTLS role of the SCTP transport has not
	// yet been negotiated. Otherwise, it will return the ID that was either
	// selected by the script or generated. After the ID is set to a non-null
	// value, it will not change.
	ID *uint16

	// Priority represents the priority for this RTCDataChannel. The priority is
	// assigned at channel creation time.
	Priority RTCPriorityType

	// ReadyState represents the state of the RTCDataChannel object.
	ReadyState RTCDataChannelState

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
	BufferedAmount uint64

	// BufferedAmountLowThreshold represents the threshold at which the
	// bufferedAmount is considered to be low. When the bufferedAmount decreases
	// from above this threshold to equal or below it, the bufferedamountlow
	// event fires. BufferedAmountLowThreshold is initially zero on each new
	// RTCDataChannel, but the application may change its value at any time.
	BufferedAmountLowThreshold uint64

	// The binaryType represents attribute MUST, on getting, return the value to
	// which it was last set. On setting, if the new value is either the string
	// "blob" or the string "arraybuffer", then set the IDL attribute to this
	// new value. Otherwise, throw a SyntaxError. When an RTCDataChannel object
	// is created, the binaryType attribute MUST be initialized to the string
	// "blob". This attribute controls how binary data is exposed to scripts.
	// binaryType                 string

	// OnOpen              func()
	// OnBufferedAmountLow func()
	// OnError             func()
	// OnClose             func()

	onMessageHandler func(sugar.Payload)
	onOpenHandler    func()

	// Deprecated: Will be removed when networkManager is deprecated.
	rtcPeerConnection *RTCPeerConnection

	dataChannel *datachannel.DataChannel
}

// OnOpen sets an event handler which is invoked when
// the underlying data transport has been established (or re-established).
func (d *RTCDataChannel) OnOpen(f func()) {
	d.Lock()
	defer d.Unlock()
	d.onOpenHandler = f
}

func (d *RTCDataChannel) onOpen() (done chan struct{}) {
	d.RLock()
	hdlr := d.onOpenHandler
	d.RUnlock()

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

// OnMessage sets an event handler which is invoked on a message
// arrival over the sctp transport from a remote peer.
func (d *RTCDataChannel) OnMessage(f func(p sugar.Payload)) {
	d.Lock()
	defer d.Unlock()
	d.onMessageHandler = f
}

func (d *RTCDataChannel) onMessage(p sugar.Payload) {
	d.RLock()
	hdlr := d.onMessageHandler
	d.RUnlock()

	if hdlr == nil || p == nil {
		return
	}
	hdlr(p)
}

// Onmessage sets an event handler which is invoked on a message
// arrival over the sctp transport from a remote peer.
//
// Deprecated: use OnMessage instead.
func (d *RTCDataChannel) Onmessage(f func(p sugar.Payload)) {
	d.OnMessage(f)
}

// func (d *RTCDataChannel) generateID() error {
// 	// TODO: base on DTLS role, currently static at "true".
// 	client := true
//
// 	var id uint16
// 	if !client {
// 		id++
// 	}
//
// 	for ; id < *d.Transport.MaxChannels-1; id += 2 {
// 		_, ok := d.rtcPeerConnection.dataChannels[id]
// 		if !ok {
// 			d.ID = &id
// 			return nil
// 		}
// 	}
// 	return &rtcerr.OperationError{Err: ErrMaxDataChannelID}
// }

func (d *RTCDataChannel) handleOpen(dc *datachannel.DataChannel) {
	d.dataChannel = dc

	// Ensure on
	d.onOpen()

	d.Lock()
	defer d.Unlock()

	if !defaultSettingEngine.Detach.DataChannels {
		go d.readLoop()
	}
}

func (d *RTCDataChannel) readLoop() {
	for {
		buffer := make([]byte, receiveMTU)
		n, isString, err := d.dataChannel.ReadDataChannel(buffer)
		if err != nil {
			fmt.Println("Failed to read from data channel", err)
			// TODO: Kill DataChannel/PeerConnection?
			return
		}

		if isString {
			d.onMessage(&sugar.PayloadString{Data: buffer[:n]})
			continue
		}
		d.onMessage(&sugar.PayloadBinary{Data: buffer[:n]})
	}
}

// Send sends the passed message to the DataChannel peer
func (d *RTCDataChannel) Send(payload sugar.Payload) error {
	var data []byte
	isString := false

	switch p := payload.(type) {
	case sugar.PayloadString:
		data = p.Data
		isString = true
	case sugar.PayloadBinary:
		data = p.Data
	default:
		return errors.Errorf("unknown DataChannel Payload (%s)", payload.PayloadType())
	}

	if len(data) == 0 {
		data = []byte{0}
	}

	_, err := d.dataChannel.WriteDataChannel(data, isString)
	return err
}

// Detach allows you to detach the underlying datachannel. This provides
// an idiomatic API to work with, however it disables the OnMessage callback.
// Before calling Detach you have to enable this behavior by calling
// webrtc.DetachDataChannels(). Combining detached and normal data channels
// is not supported.
// Please reffer to the data-channels-detach example and the
// pions/datachannel documentation for the correct way to handle the
// resulting DataChannel object.
func (d *RTCDataChannel) Detach() (*datachannel.DataChannel, error) {
	d.Lock()
	defer d.Unlock()

	if !defaultSettingEngine.Detach.DataChannels {
		return nil, errors.New("enable detaching by calling webrtc.DetachDataChannels()")
	}

	if d.dataChannel == nil {
		return nil, errors.New("datachannel not opened yet, try calling Detach from OnOpen")
	}

	return d.dataChannel, nil
}
