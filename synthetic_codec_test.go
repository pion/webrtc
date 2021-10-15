package webrtc

import (
	"math"
	"time"
)

// TODO(mathis): Move synthetic codecs to separate package/module
type Frame struct {
	content            []byte
	secondsToNextFrame time.Duration
}

type syntheticCodec interface {
	getTargetBitrate() float64
	setTargetBitrate(float64) float64
	nextPacketOrFrame() Frame
}

type simpleFPSBasedCodec struct {
	targetBitrate float64 // bps
	fps           float64
}

func newSimpleFPSBasedCodec(targetBitrate float64) *simpleFPSBasedCodec {
	return &simpleFPSBasedCodec{
		targetBitrate: targetBitrate,
		fps:           30,
	}
}

func (c *simpleFPSBasedCodec) getTargetBitrate() float64 {
	return c.targetBitrate
}

func (c *simpleFPSBasedCodec) setTargetBitrate(r float64) float64 {
	c.targetBitrate = r
	return c.targetBitrate
}

func (c *simpleFPSBasedCodec) nextPacketOrFrame() Frame {
	frameBytes := int(math.Ceil(c.targetBitrate / (c.fps * 8.0)))
	msToNextFrame := time.Duration((1.0/c.fps)*1000.0) * time.Millisecond

	return Frame{
		content:            make([]byte, frameBytes),
		secondsToNextFrame: msToNextFrame,
	}
}
