// +build !js

package interceptor

import (
	"math/rand"
	"sync"
	"time"

	"github.com/pion/logging"
	"github.com/pion/rtcp"
	"github.com/pion/rtp"
)

// ReceiverNACK interceptor generates nack messages.
type ReceiverNACK struct {
	NoOp
	size        uint16
	receiveLogs *sync.Map
	m           sync.Mutex
	wg          sync.WaitGroup
	close       chan struct{}
	log         logging.LeveledLogger
}

// NewReceiverNack returns a new ReceiverNACK interceptor
func NewReceiverNack(size uint16, log logging.LeveledLogger) (*ReceiverNACK, error) {
	_, err := NewReceiveLog(size)
	if err != nil {
		return nil, err
	}

	return &ReceiverNACK{
		NoOp:        NoOp{},
		size:        size,
		receiveLogs: &sync.Map{},
		close:       make(chan struct{}),
		log:         log,
	}, nil
}

// BindRTCPWriter lets you modify any outgoing RTCP packets. It is called once per PeerConnection. The returned method
// will be called once per packet batch.
func (n *ReceiverNACK) BindRTCPWriter(writer RTCPWriter) RTCPWriter {
	go n.loop(writer)

	return writer
}

// BindRemoteStream lets you modify any incoming RTP packets. It is called once for per RemoteStream. The returned method
// will be called once per rtp packet.
func (n *ReceiverNACK) BindRemoteStream(info *StreamInfo, reader RTPReader) RTPReader {
	hasNack := false
	for _, fb := range info.RTCPFeedback {
		if fb.Type == "nack" && fb.Parameter == "" {
			hasNack = true
		}
	}

	if !hasNack {
		return reader
	}

	// error is already checked in NewReceiverNack
	receiveLog, _ := NewReceiveLog(n.size)
	n.receiveLogs.Store(info.SSRC, receiveLog)

	return RTPReaderFunc(func() (*rtp.Packet, Attributes, error) {
		p, attr, err := reader.Read()
		if err != nil {
			return nil, nil, err
		}

		receiveLog.Add(p.SequenceNumber)

		return p, attr, nil
	})
}

// UnbindLocalStream is called when the Stream is removed. It can be used to clean up any data related to that track.
func (n *ReceiverNACK) UnbindLocalStream(info *StreamInfo) {
	n.receiveLogs.Delete(info.SSRC)
}

func (n *ReceiverNACK) Close() error {
	defer n.wg.Wait()
	n.m.Lock()
	defer n.m.Unlock()

	select {
	case <-n.close:
		// already closed
		return nil
	default:
	}

	close(n.close)

	return nil
}

func (n *ReceiverNACK) loop(rtcpWriter RTCPWriter) {
	defer n.wg.Done()

	senderSSRC := rand.Uint32()

	ticker := time.NewTicker(time.Millisecond * 100)
	for {
		select {
		case <-ticker.C:
			n.receiveLogs.Range(func(key, value interface{}) bool {
				ssrc := key.(uint32)
				receiveLog := value.(*ReceiveLog)

				missing := receiveLog.MissingSeqNumbers(10)
				if len(missing) == 0 {
					return true
				}

				nack := &rtcp.TransportLayerNack{
					SenderSSRC: senderSSRC,
					MediaSSRC:  ssrc,
					Nacks:      nackPairs(missing),
				}

				_, err := rtcpWriter.Write([]rtcp.Packet{nack}, Attributes{})
				if err != nil {
					n.log.Warnf("failed sending nack: %+v", err)
				}

				return true
			})

		case <-n.close:
			return
		}
	}
}

func nackPairs(seqNums []uint16) []rtcp.NackPair {
	pairs := make([]rtcp.NackPair, 0)
	startSeq := seqNums[0]
	nackPair := &rtcp.NackPair{PacketID: startSeq}
	for i := 1; i < len(seqNums); i++ {
		m := seqNums[i]

		if m-nackPair.PacketID > 16 {
			pairs = append(pairs, *nackPair)
			nackPair = &rtcp.NackPair{PacketID: m}
			continue
		}

		nackPair.LostPackets |= 1 << (m - nackPair.PacketID - 1)
	}

	pairs = append(pairs, *nackPair)

	return pairs
}
