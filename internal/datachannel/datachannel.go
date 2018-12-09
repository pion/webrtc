package datachannel

import (
	"fmt"

	"github.com/pions/webrtc/internal/sctp"
	"github.com/pkg/errors"
)

const receiveMTU = 8192

// DataChannel represents a data channel
type DataChannel struct {
	Config
	stream *sctp.Stream
}

// Config is used to configure the data channel.
type Config struct {
	ChannelType          ChannelType
	Priority             uint16
	ReliabilityParameter uint32
	Label                string
}

// Dial opens a data channels over SCTP
func Dial(a *sctp.Association, id uint16, config *Config) (*DataChannel, error) {
	stream, err := a.OpenStream(id, sctp.PayloadTypeWebRTCBinary)
	if err != nil {
		return nil, err
	}

	dc, err := Client(stream, config)
	if err != nil {
		return nil, err
	}

	return dc, nil
}

// Client opens a data channel over an SCTP stream
func Client(stream *sctp.Stream, config *Config) (*DataChannel, error) {
	msg := &ChannelOpen{
		ChannelType:          config.ChannelType,
		Priority:             config.Priority,
		ReliabilityParameter: config.ReliabilityParameter,

		Label:    []byte(config.Label),
		Protocol: []byte(""),
	}

	rawMsg, err := msg.Marshal()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal ChannelOpen %v", err)
	}

	_, err = stream.WriteSCTP(rawMsg, sctp.PayloadTypeWebRTCDCEP)
	if err != nil {
		return nil, fmt.Errorf("failed to send ChannelOpen %v", err)
	}

	dataChannel := &DataChannel{
		Config: *config,
		stream: stream,
	}

	return dataChannel, nil
}

// Accept is used to accept incoming data channels over SCTP
func Accept(a *sctp.Association) (*DataChannel, error) {
	stream, err := a.AcceptStream()
	if err != nil {
		return nil, err
	}

	stream.SetDefaultPayloadType(sctp.PayloadTypeWebRTCBinary)

	dc, err := Server(stream)
	if err != nil {
		return nil, err
	}

	return dc, nil
}

// Server accepts a data channel over an SCTP stream
func Server(stream *sctp.Stream) (*DataChannel, error) {
	buffer := make([]byte, receiveMTU) // TODO: Can probably be smaller
	n, ppi, err := stream.ReadSCTP(buffer)
	if err != nil {
		return nil, err
	}

	if ppi != sctp.PayloadTypeWebRTCDCEP {
		return nil, fmt.Errorf("unexpected packet type: %s", ppi)
	}

	openMsg, err := ParseExpectDataChannelOpen(buffer[:n])
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse DataChannelOpen packet")
	}

	dataChannel := &DataChannel{
		Config: Config{
			ChannelType:          openMsg.ChannelType,
			Priority:             openMsg.Priority,
			ReliabilityParameter: openMsg.ReliabilityParameter,
			Label:                string(openMsg.Label),
		},
		stream: stream,
	}

	err = dataChannel.writeDataChannelAck()
	if err != nil {
		return nil, err
	}
	return dataChannel, nil

}

// Read reads a packet of len(p) bytes as binary data
func (c *DataChannel) Read(p []byte) (int, error) {
	n, _, err := c.ReadDataChannel(p)
	return n, err
}

// ReadDataChannel reads a packet of len(p) bytes
func (c *DataChannel) ReadDataChannel(p []byte) (int, bool, error) {
	for {
		n, ppi, err := c.stream.ReadSCTP(p)

		var isString bool
		switch ppi {
		case sctp.PayloadTypeWebRTCDCEP:
			err = c.handleDCEP(p[:n])
			if err != nil {
				fmt.Println("Failed to handle DCEP:", err)
				continue
			}
			continue
		case sctp.PayloadTypeWebRTCString, sctp.PayloadTypeWebRTCStringEmpty:
			isString = true
		}
		return n, isString, err
	}
}

// StreamIdentifier returns the Stream identifier associated to the stream.
func (c *DataChannel) StreamIdentifier() uint16 {
	return c.stream.StreamIdentifier()
}

func (c *DataChannel) handleDCEP(data []byte) error {
	msg, err := Parse(data)
	if err != nil {
		return errors.Wrap(err, "Failed to parse DataChannel packet")
	}

	switch msg := msg.(type) {
	case *ChannelOpen:
		err = c.writeDataChannelAck()
		if err != nil {
			return fmt.Errorf("failed to ACK channel open: %v", err)
		}
		// TODO: Should not happen?

	case *ChannelAck:
		// TODO: handle ChannelAck (https://tools.ietf.org/html/draft-ietf-rtcweb-data-protocol-09#section-5.2)
		// TODO: handle?

	default:
		return fmt.Errorf("unhandled DataChannel message %v", msg)
	}

	return nil
}

// Write writes len(p) bytes from p as binary data
func (c *DataChannel) Write(p []byte) (n int, err error) {
	return c.WriteDataChannel(p, false)
}

// WriteDataChannel writes len(p) bytes from p
func (c *DataChannel) WriteDataChannel(p []byte, isString bool) (n int, err error) {
	// https://tools.ietf.org/html/draft-ietf-rtcweb-data-channel-12#section-6.6
	// SCTP does not support the sending of empty user messages.  Therefore,
	// if an empty message has to be sent, the appropriate PPID (WebRTC
	// String Empty or WebRTC Binary Empty) is used and the SCTP user
	// message of one zero byte is sent.  When receiving an SCTP user
	// message with one of these PPIDs, the receiver MUST ignore the SCTP
	// user message and process it as an empty message.
	var ppi sctp.PayloadProtocolIdentifier
	switch {
	case !isString && len(p) > 0:
		ppi = sctp.PayloadTypeWebRTCBinary
	case !isString && len(p) == 0:
		ppi = sctp.PayloadTypeWebRTCBinaryEmpty
	case isString && len(p) > 0:
		ppi = sctp.PayloadTypeWebRTCString
	case isString && len(p) == 0:
		ppi = sctp.PayloadTypeWebRTCStringEmpty
	}

	return c.stream.WriteSCTP(p, ppi)
}

func (c *DataChannel) writeDataChannelAck() error {
	ack := ChannelAck{}
	ackMsg, err := ack.Marshal()
	if err != nil {
		return fmt.Errorf("failed to marshal ChannelOpen ACK: %v", err)
	}

	_, err = c.stream.WriteSCTP(ackMsg, sctp.PayloadTypeWebRTCDCEP)
	if err != nil {
		return fmt.Errorf("failed to send ChannelOpen ACK: %v", err)
	}

	return err
}

// Close closes the DataChannel and the underlying SCTP stream.
func (c *DataChannel) Close() error {
	return c.stream.Close()
}
