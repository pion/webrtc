package webrtc

import (
	"sync"

	"container/list"

	"fmt"

	"github.com/pions/webrtc/internal/sctp"
	"github.com/pions/webrtc/pkg/dcep"
	"github.com/pions/webrtc/pkg/rtcerr"
	"github.com/pkg/errors"
)

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
	MaxPacketLifeTime *uint32

	// MaxRetransmits represents the maximum number of retransmissions that are
	// attempted in unreliable mode.
	MaxRetransmits *uint32

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

	// OnOpen designates an event handler which is invoked on data channel
	// ready state changing to "open".
	OnOpen func()

	// OnBufferedAmountLow func()
	// OnError             func(*RTCErrorEvent)
	// OnClose             func()

	// Onmessage designates an event handler which is invoked on a message
	// arrival over the sctp transport from a remote peer.
	//
	// Deprecated: use OnMessage instead.
	Onmessage func(dcep.Payload)

	// OnMessage designates an event handler which is invoked on a message
	// arrival over the sctp transport from a remote peer.
	OnMessage func(dcep.Payload)

	fromSctp chan interface{}
}

func newDataChannel(sctp *RTCSctpTransport) *RTCDataChannel {
	channel := &RTCDataChannel{
		Transport:         sctp,
		Ordered:           true,
		MaxPacketLifeTime: nil,
		MaxRetransmits:    nil,
		Protocol:          "",
		Negotiated:        false,
		ID:                nil,
		Priority:          RTCPriorityTypeLow,
		// https://w3c.github.io/webrtc-pc/#dfn-create-an-rtcdatachannel (Step #2)
		ReadyState: RTCDataChannelStateConnecting,
		// https://w3c.github.io/webrtc-pc/#dfn-create-an-rtcdatachannel (Step #3)
		BufferedAmount: 0,
		fromSctp:       make(chan interface{}, 1),
	}

	go channel.handler()
	return channel
}

func (c *RTCDataChannel) handler() {
	inbound := make(chan interface{}, 1)
	queue := list.New()
	for {
		if front := queue.Front(); front == nil {
			if c.fromSctp == nil {
				close(inbound)
				return
			}

			value, ok := <-c.fromSctp
			if !ok {
				close(inbound)
				return
			}
			queue.PushBack(value)
		} else {
			select {
			case inbound <- front.Value:
				val := <-inbound
				switch val := val.(type) {
				case sctp.ReceiveEvent:
					go c.onReceiveHandler(val)
					// case sctp.SendFailureEvent:
					// case sctp.NetworkStatusChangeEvent:
				case sctp.CommunicationUpEvent:
					go c.onCommunicationUpHandler(val)
					// case sctp.CommunicationLostEvent:
					// case sctp.CommunicationErrorEvent
					// case sctp.RestartEvent
					// case sctp.ShutdownCompleteEvent:
				}
				queue.Remove(front)
			case value, ok := <-c.fromSctp:
				if ok {
					queue.PushBack(value)
				} else {
					c.fromSctp = nil
				}
			}
		}
	}
}

func (c *RTCDataChannel) onReceiveHandler(event sctp.ReceiveEvent) {
	switch event.PayloadProtocolID {
	case sctp.PayloadTypeWebRTCDcep:
		msg, err := dcep.Parse(event.Buffer)
		if err != nil {
			fmt.Println(errors.Wrap(err, "Failed to parse DataChannel packet"))
			return
		}

		switch msg.(type) {
		case *dcep.ChannelOpen:
			if c.Transport.conn.isClosed {
				return
			}

			if c.ReadyState == RTCDataChannelStateClosing ||
				c.ReadyState == RTCDataChannelStateClosed {
				return
			}

			c.ReadyState = RTCDataChannelStateOpen
			go c.OnOpen()
		}
	case sctp.PayloadTypeWebRTCString:
		fallthrough
	case sctp.PayloadTypeWebRTCStringEmpty:
		c.RLock()
		defer c.RUnlock()

		if c.Onmessage == nil && c.OnMessage == nil {
			fmt.Printf("Onmessage has not been set for Datachannel %s %d \n", c.Label, *c.ID)
		}

		if c.Onmessage != nil {
			go c.Onmessage(dcep.PayloadString{Data: event.Buffer})
		}

		if c.OnMessage != nil {
			go c.OnMessage(dcep.PayloadString{Data: event.Buffer})
		}
	case sctp.PayloadTypeWebRTCBinary:
		fallthrough
	case sctp.PayloadTypeWebRTCBinaryEmpty:
		c.RLock()
		defer c.RUnlock()

		if c.Onmessage == nil && c.OnMessage == nil {
			fmt.Printf("Onmessage has not been set for Datachannel %s %d \n", c.Label, *c.ID)
		}

		if c.Onmessage != nil {
			go c.Onmessage(dcep.PayloadBinary{Data: event.Buffer})
		}

		if c.OnMessage != nil {
			go c.OnMessage(dcep.PayloadBinary{Data: event.Buffer})
		}
	default:
		fmt.Printf("Unhandled Payload Protocol Identifier %v \n", event.PayloadProtocolID)
	}
}

func (c *RTCDataChannel) onCommunicationUpHandler(event sctp.CommunicationUpEvent) {

}

// Close closes the RTCDataChannel. It may be called regardless of whether the
// RTCDataChannel object was created by this peer or the remote peer.
func (c *RTCDataChannel) Close() error {
	if c.ReadyState == RTCDataChannelStateClosing || c.ReadyState == RTCDataChannelStateClosed {
		return nil
	}

	c.ReadyState = RTCDataChannelStateClosing

	// if err := d.rtcPeerConnection.networkManager.SendOpenChannelMessage(d.ID, d.Label); err != nil {
	// 	return &rtcerr.UnknownError{Err: err}
	// }
	return nil

}

// Send sends the passed message to the DataChannel peer
func (c *RTCDataChannel) Send(p dcep.Payload) error {
	return c.Transport.send(p, *c.ID)

	// if err := d.rtcPeerConnection.networkManager.SendDataChannelMessage(p, *d.ID); err != nil {
	// 	return &rtcerr.UnknownError{Err: err}
	// }
	// return nil
}

// // Send sends the passed message to the DataChannel peer
// func (d *RTCDataChannel) Send(msg interface{}) error {
// 	switch msg := msg.(type) {
// 	// FIXME: THIS USECASE IS BEING DEPRECATED
// 	case datachannel.Payload:
// 		if err := d.rtcPeerConnection.networkManager.SendDataChannelMessage(msg, *d.ID); err != nil {
// 			return &rtcerr.UnknownError{Err: err}
// 		}
// 	case string:
// 	case []byte:
// 	}
// 	return nil
// }

func (c *RTCDataChannel) generateID() (int, error) {
	// TODO: base on DTLS role, currently static at "true".
	client := true

	var id uint16
	if !client {
		id++
	}

	c.Transport.Lock()
	defer c.Transport.Unlock()
	count := len(c.Transport.channels)
	for ; id < *c.Transport.MaxChannels-1; id += 2 {
		_, ok := c.Transport.channels[id]
		if !ok {
			// // https://w3c.github.io/webrtc-pc/#peer-to-peer-data-api (Step #18)
			if id > 65534 {
				return count, &rtcerr.TypeError{Err: ErrMaxDataChannelID}
			}

			// // https://w3c.github.io/webrtc-pc/#peer-to-peer-data-api (Step #20)
			if c.Transport.State == RTCSctpTransportStateConnected && id >= *c.Transport.MaxChannels {
				return count, &rtcerr.OperationError{Err: ErrMaxDataChannelID}
			}

			c.ID = &id
			c.Transport.channels[id] = c.fromSctp
			return count, nil
		}
	}
	return count, &rtcerr.OperationError{Err: ErrMaxDataChannelID}
}

// // SendOpenChannelMessage is a test to send OpenChannel manually
// //
// // Deprecated: Function discontinued in favor of spec compliance.
// func (c *RTCDataChannel) SendOpenChannelMessage() error {
// 	if err := c.rtcPeerConnection.networkManager.SendOpenChannelMessage(*c.ID, c.Label); err != nil {
// 		return &rtcerr.UnknownError{Err: err}
// 	}
// 	return nil
//
// }
