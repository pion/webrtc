package interceptor

import (
	"sync"

	"github.com/pion/logging"
	"github.com/pion/rtcp"
	"github.com/pion/rtp"
)

type SenderNack struct {
	NoOp
	size    uint16
	streams *sync.Map
	log     logging.LeveledLogger
}

type senderNackStream struct {
	sendBuffer *SendBuffer
	rtpWriter  RTPWriter
}

// NewSenderNack returns a new ReceiverNACK interceptor
func NewSenderNack(size uint16, log logging.LeveledLogger) (*SenderNack, error) {
	_, err := NewSendBuffer(size)
	if err != nil {
		return nil, err
	}

	return &SenderNack{
		NoOp:    NoOp{},
		size:    size,
		streams: &sync.Map{},
		log:     log,
	}, nil
}

// BindRTCPReader lets you modify any incoming RTCP packets. It is called once per sender/receiver, however this might
// change in the future. The returned method will be called once per packet batch.
func (n *SenderNack) BindRTCPReader(reader RTCPReader) RTCPReader {
	return RTCPReaderFunc(func() ([]rtcp.Packet, Attributes, error) {
		pkts, attr, err := reader.Read()
		if err != nil {
			return nil, nil, err
		}

		for _, rtcpPacket := range pkts {
			nack, ok := rtcpPacket.(*rtcp.TransportLayerNack)
			if !ok {
				continue
			}

			go n.resendPackets(nack)
		}

		return pkts, attr, err
	})
}

// BindLocalStream lets you modify any outgoing RTP packets. It is called once for per LocalStream. The returned method
// will be called once per rtp packet.
func (n *SenderNack) BindLocalStream(info *StreamInfo, writer RTPWriter) RTPWriter {
	hasNack := false
	for _, fb := range info.RTCPFeedback {
		if fb.Type == "nack" && fb.Parameter == "" {
			hasNack = true
		}
	}

	if !hasNack {
		return writer
	}

	// error is already checked in NewReceiverNack
	sendBuffer, _ := NewSendBuffer(n.size)
	n.streams.Store(info.SSRC, &senderNackStream{sendBuffer: sendBuffer, rtpWriter: writer})

	return RTPWriterFunc(func(p *rtp.Packet, attributes Attributes) (int, error) {
		sendBuffer.Add(p)

		return writer.Write(p, attributes)
	})
}

// UnbindLocalStream is called when the Stream is removed. It can be used to clean up any data related to that track.
func (n *SenderNack) UnbindLocalStream(info *StreamInfo) {
	n.streams.Delete(info.SSRC)
}

func (n *SenderNack) resendPackets(nack *rtcp.TransportLayerNack) {
	v, ok := n.streams.Load(nack.MediaSSRC)
	if !ok {
		return
	}

	stream := v.(*senderNackStream)
	seqNums := nackParsToSequenceNumbers(nack.Nacks)

	for _, seq := range seqNums {
		p := stream.sendBuffer.Get(seq)
		if p == nil {
			continue
		}

		_, err := stream.rtpWriter.Write(p, Attributes{})
		if err != nil {
			n.log.Warnf("failed resending nacked packet: %+v", err)
		}
	}
}

func nackParsToSequenceNumbers(pairs []rtcp.NackPair) []uint16 {
	seqs := make([]uint16, 0)
	for _, pair := range pairs {
		startSeq := pair.PacketID
		seqs = append(seqs, startSeq)
		for i := 0; i < 16; i++ {
			if (pair.LostPackets & (1 << i)) != 0 {
				seqs = append(seqs, startSeq+uint16(i)+1)
			}
		}
	}

	return seqs
}
