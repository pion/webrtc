package sfu

import (
	"context"

	"github.com/pions/webrtc/pkg/rtp"
)

type (
	sourcePacketStream <-chan *rtp.Packet
	sinkPacketStream   chan<- *rtp.Packet

	sfuSourceReader struct {
		cancel context.CancelFunc
	}

	// SFU represents an implementation of a WebRTC Selective Forwarding Unit
	// It is responsible for forwarding source packets to all interested recipients
	SFU struct {
		cancel        context.CancelFunc
		sourcePackets chan *rtp.Packet
		sources       map[sourcePacketStream]*sfuSourceReader
		addSource     chan sourcePacketStream
		removeSource  chan sourcePacketStream
		sinks         map[sinkPacketStream]bool
		addSink       chan sinkPacketStream
		removeSink    chan sinkPacketStream
	}
)

func newSourceReader(parent context.Context, input sourcePacketStream, output sinkPacketStream) *sfuSourceReader {
	ctx, cancel := context.WithCancel(parent)

	rdr := &sfuSourceReader{
		cancel: cancel,
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case pkt := <-input:
				select {
				case <-ctx.Done():
					return
				case output <- pkt:
				}
			}
		}
	}()

	return rdr
}

func (rdr *sfuSourceReader) Shutdown() {
	if rdr.cancel != nil {
		rdr.cancel()
	}
}

// New returns an initialized *SFU
func New() *SFU {
	ctx, cancel := context.WithCancel(context.Background())

	sfu := &SFU{
		cancel:        cancel,
		sourcePackets: make(chan *rtp.Packet),
		sources:       make(map[sourcePacketStream]*sfuSourceReader),
		addSource:     make(chan sourcePacketStream),
		removeSource:  make(chan sourcePacketStream),
		sinks:         make(map[sinkPacketStream]bool),
		addSink:       make(chan sinkPacketStream),
		removeSink:    make(chan sinkPacketStream),
	}

	go sfu.runLoop(ctx)

	return sfu
}

func (sfu *SFU) sendPacket(ctx context.Context, sourcePacket *rtp.Packet, recipients []sinkPacketStream) {
	for _, recipient := range recipients {
		// FIXME: This is terrible for GC pressure -- need some way to know when the
		// packet has been sent and can be returned to a pool
		var sinkPacket rtp.Packet
		rawBuf := make([]byte, len(sourcePacket.Raw))
		copy(rawBuf, sourcePacket.Raw)

		if err := sinkPacket.Unmarshal(rawBuf); err != nil {
			return
		}
		select {
		case <-ctx.Done():
			return
		case recipient <- &sinkPacket:
		}
	}
}

func (sfu *SFU) runLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case source := <-sfu.addSource:
			if _, found := sfu.sources[source]; !found {
				sfu.sources[source] = newSourceReader(ctx, source, sfu.sourcePackets)
			}
		case source := <-sfu.removeSource:
			if rdr, found := sfu.sources[source]; found {
				rdr.Shutdown()
				delete(sfu.sources, source)
			}
		case sink := <-sfu.addSink:
			if _, found := sfu.sinks[sink]; !found {
				sfu.sinks[sink] = true
			}
		case sink := <-sfu.removeSink:
			delete(sfu.sinks, sink)
		case sourcePacket := <-sfu.sourcePackets:
			recipients := make([]sinkPacketStream, 0, len(sfu.sinks))
			for sink := range sfu.sinks {
				recipients = append(recipients, sink)
			}
			go sfu.sendPacket(ctx, sourcePacket, recipients)
		}
	}
}

// Shutdown stops the SFU and all source readers
func (sfu *SFU) Shutdown() {
	if sfu.cancel != nil {
		sfu.cancel()
	}
}

// AddSource adds the supplied source packet stream to the set of
// inputs which will be forwarded
func (sfu *SFU) AddSource(source sourcePacketStream) {
	sfu.addSource <- source
}

// RemoveSource removes the supplied source packet stream from the set of
// inputs which will be forwarded
func (sfu *SFU) RemoveSource(source sourcePacketStream) {
	sfu.removeSource <- source
}

// AddSink adds the supplied sink packet stream to the set of recipients
// to which inputs will be forwarded
func (sfu *SFU) AddSink(sink sinkPacketStream) {
	sfu.addSink <- sink
}

// RemoveSink removes the supplied sink packet stream from the set of recipients
// to which inputs will be forwarded
func (sfu *SFU) RemoveSink(sink sinkPacketStream) {
	sfu.removeSink <- sink
}

// OneToMany is a convenience initializer that creates an SFU set up
// to forward packets from a single source to N recipients
func OneToMany(source sourcePacketStream, sinks ...sinkPacketStream) *SFU {
	sfu := New()

	sfu.AddSource(source)
	for _, sink := range sinks {
		sfu.AddSink(sink)
	}

	return sfu
}
