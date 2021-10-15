package webrtc

import (
	"context"
	"errors"
	"io"
	"math"
	"time"

	"github.com/pion/interceptor/pkg/cc"
	"github.com/pion/logging"
	"github.com/pion/webrtc/v3/pkg/media"
)

type track struct {
	trackConfig
	log       logging.LeveledLogger
	writer    *TrackLocalStaticSample
	rtpSender *RTPSender
}

func (t *track) start(ctx context.Context, metrics chan<- int) {
	frame := t.codec.nextPacketOrFrame()
	if err := t.writer.WriteSample(media.Sample{
		Data:     frame.content,
		Duration: frame.secondsToNextFrame,
	}); err != nil {
		panic(err)
	}

	sendTimer := time.NewTimer(frame.secondsToNextFrame)
	defer sendTimer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-sendTimer.C:
			frame = t.codec.nextPacketOrFrame()
			if err := t.writer.WriteSample(media.Sample{
				Data:     frame.content,
				Duration: frame.secondsToNextFrame,
			}); err != nil {
				panic(err)
			}
			metrics <- len(frame.content)
			sendTimer.Reset(frame.secondsToNextFrame)
		}
	}
}

type mediaSender struct {
	log    logging.LeveledLogger
	pc     *PeerConnection
	tracks []*track
	bwe    cc.BandwidthEstimator

	cbrSum    int // sum of bitrates of all CBR tracks
	vbrCodecs int // count of VBR tracks
}

func newMediaSender(log logging.LeveledLogger, pc *PeerConnection, bwe cc.BandwidthEstimator) *mediaSender {
	return &mediaSender{
		log:    log,
		pc:     pc,
		tracks: []*track{},
		bwe:    bwe,
	}
}

func (s *mediaSender) addTrack(c trackConfig) error {
	trackLocalStaticSample, err := NewTrackLocalStaticSample(c.capability, c.id, c.streamID)
	if err != nil {
		return err
	}
	rtpSender, err := s.pc.AddTrack(trackLocalStaticSample)
	if err != nil {
		return err
	}
	s.tracks = append(s.tracks, &track{
		log:         s.log,
		writer:      trackLocalStaticSample,
		rtpSender:   rtpSender,
		trackConfig: c,
	})
	if c.vbr {
		s.vbrCodecs++
	} else {
		s.cbrSum += int(c.codec.getTargetBitrate())
	}
	return nil
}

func (s *mediaSender) start(ctx context.Context) {
	metrics := make(chan int)
	for _, t := range s.tracks {
		go func() {
			for {
				if _, _, err := t.rtpSender.ReadRTCP(); err != nil {
					if errors.Is(io.EOF, err) {
						s.log.Tracef("rtpSender.ReadRTCP got EOF")
						return
					}
					s.log.Errorf("rtpSender.ReadRTCP returned error: %v", err)
					return
				}
			}
		}()
		go t.start(ctx, metrics)
	}
	// TODO(mathis): use bwe to set VBR tracks bitrate

	go func() {
		ticker := time.NewTicker(1 * time.Second)
		bytesSent := 0
		for {
			select {
			case <-ctx.Done():
				return

			case b := <-metrics:
				bytesSent += b

			case <-ticker.C:
				s.log.Tracef("sent %v bit/s", bytesSent*8)
				bytesSent = 0

				estimate := s.bwe.GetBandwidthEstimation()
				share := float64(int(estimate)-s.cbrSum) / float64(s.vbrCodecs)
				share = math.Max(0, share)

				s.log.Tracef("got new estimate: %v\n", estimate)
				for _, track := range s.tracks {
					if track.vbr {
						s.log.Tracef("set track %v to bitrate %v\n", track.id, share)
						track.codec.setTargetBitrate(share)
					}
				}
			}
		}
	}()
}
