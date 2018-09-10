package webrtc

import (
	"fmt"
	"math"
	"sync"

	"github.com/pions/webrtc/internal/sctp"
	"github.com/pions/webrtc/pkg/dcep"
	"github.com/pkg/errors"
)

// RTCSctpTransport provides details about the SCTP transport.
type RTCSctpTransport struct {
	sync.RWMutex

	// Transport represents the transport over which all SCTP packets for data
	// channels will be sent and received.
	Transport *RTCDtlsTransport

	// State represents the current state of the SCTP transport.
	State RTCSctpTransportState

	// MaxMessageSize represents the maximum size of data that can be passed to
	// RTCDataChannel's send() method.
	MaxMessageSize float64

	// MaxChannels represents the maximum amount of RTCDataChannel's that can
	// be used simultaneously.
	MaxChannels *uint16

	// OnStateChange indicates that the RTCSctpTransport state changed.
	OnStateChange func()

	conn        *RTCPeerConnection
	association *sctp.Association
	channels    map[uint16]chan interface{}
}

func newRTCSctpTransport(connection *RTCPeerConnection) (*RTCSctpTransport, error) {
	var err error
	t := &RTCSctpTransport{
		State: RTCSctpTransportStateConnecting,
		conn:  connection,
	}
	t.association = sctp.NewAssocation()
	t.association.OnReceive = t.onReceiveHandler
	// t.association.OnSendFailure = t.onSendFailureHandler
	// t.association.OnNetworkStatusChange = t.onNetworkStatusChangeHandler
	t.association.OnCommunicationUp = t.onCommunicationUpHandler
	// t.association.OnCommunicationLost = t.onCommunicationLostHandler
	// t.association.OnCommunicationError = t.onCommunicationErrorHandler
	// t.association.OnRestart = t.onRestartHandler
	// t.association.OnShutdownComplete = t.onShutdownCompleteHandler

	t.Transport, err = newRTCDtlsTransport(connection)
	if err != nil {
		return nil, err
	}

	// dtls -> sctp
	t.Transport.toSctp = t.association.Input

	// dtls <- sctp
	t.Transport.fromSctp = t.association.Output

	t.updateMessageSize()
	t.updateMaxChannels()

	// go t.handler()

	return t, nil
}

func (r *RTCSctpTransport) onReceiveHandler(event sctp.ReceiveEvent) {
	switch event.PayloadProtocolID {
	case sctp.PayloadTypeWebRTCDcep:
		msg, err := dcep.Parse(event.Buffer)
		if err != nil {
			fmt.Println(errors.Wrap(err, "Failed to parse DataChannel packet"))
			return
		}

		switch msg := msg.(type) {
		case *dcep.ChannelOpen:
			ack, err := dcep.ChannelAck{}.Marshal()
			if err != nil {
				fmt.Println("Error Marshaling ChannelOpen ACK", err)
				return
			}

			r.conn.RLock()
			defer r.conn.RUnlock()

			if r.conn.isClosed {
				return
			}

			channel := newDataChannel(r)
			channel.Label = string(msg.Label)

			if msg.ChannelType == dcep.ChannelTypeReliableUnordered ||
				msg.ChannelType == dcep.ChannelTypePartialReliableRexmitUnordered ||
				msg.ChannelType == dcep.ChannelTypePartialReliableTimedUnordered {
				channel.Ordered = false
			}

			if msg.ChannelType == dcep.ChannelTypePartialReliableTimed ||
				msg.ChannelType == dcep.ChannelTypePartialReliableTimedUnordered {
				channel.MaxPacketLifeTime = &msg.ReliabilityParameter
			}

			if msg.ChannelType == dcep.ChannelTypePartialReliableRexmit ||
				msg.ChannelType == dcep.ChannelTypePartialReliableRexmitUnordered {
				channel.MaxRetransmits = &msg.ReliabilityParameter
			}

			channel.Protocol = msg.Protocol
			channel.ID = &event.StreamID
			channel.Negotiated = false
			channel.Priority = newRTCPriorityTypeFromUint16(msg.Priority)
			channel.ReadyState = RTCDataChannelStateOpen

			r.Lock()
			defer r.Unlock()
			r.channels[event.StreamID] = channel.fromSctp

			if err = r.association.Send(ack, event.StreamID, event.PayloadProtocolID); err != nil {
				fmt.Println("Error sending ChannelOpen ACK", err)
				return
			}

			if r.conn.Ondatachannel == nil && r.conn.OnDataChannel == nil {
				fmt.Println("OnDataChannel is unset, discarding message")
			}

			if r.conn.Ondatachannel != nil {
				go r.conn.Ondatachannel(channel)
			}

			if r.conn.OnDataChannel != nil {
				go r.conn.OnDataChannel(channel)
			}

			r.channels[event.StreamID] <- event
		case *dcep.ChannelAck:
			r.Lock()
			defer r.Unlock()

			r.State = RTCSctpTransportStateConnected
			if r.OnStateChange != nil {
				go r.OnStateChange()
			}
		}
	}
}

func (r *RTCSctpTransport) onCommunicationUpHandler(event sctp.CommunicationUpEvent) {

}

func (r *RTCSctpTransport) send(payload dcep.Payload, streamID uint16) error {
	var data []byte
	var payloadProtocolID sctp.PayloadProtocolID

	/*
		https://tools.ietf.org/html/draft-ietf-rtcweb-data-channel-12#section-6.6
		SCTP does not support the sending of empty user messages.  Therefore,
		if an empty message has to be sent, the appropriate PPID (WebRTC
		String Empty or WebRTC Binary Empty) is used and the SCTP user
		message of one zero byte is sent.  When receiving an SCTP user
		message with one of these PPIDs, the receiver MUST ignore the SCTP
		user message and process it as an empty message.
	*/
	switch p := payload.(type) {
	case dcep.PayloadString:
		data = p.Data
		if len(data) == 0 {
			data = []byte{0}
			payloadProtocolID = sctp.PayloadTypeWebRTCStringEmpty
		} else {
			payloadProtocolID = sctp.PayloadTypeWebRTCString
		}
	case dcep.PayloadBinary:
		data = p.Data
		if len(data) == 0 {
			data = []byte{0}
			payloadProtocolID = sctp.PayloadTypeWebRTCBinaryEmpty
		} else {
			payloadProtocolID = sctp.PayloadTypeWebRTCBinary
		}
	default:
		return errors.Errorf("Unknown DataChannel Payload (%s)", payload.PayloadType().String())
	}

	r.association.Lock()
	defer r.association.Unlock()
	if err := r.association.Send(data, streamID, payloadProtocolID); err != nil {
		return errors.Wrap(err, "SCTP Association failed handling outbound packet")
	}

	return nil
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
	val := uint16(65535)
	r.MaxChannels = &val // TODO: Get from implementation
}
