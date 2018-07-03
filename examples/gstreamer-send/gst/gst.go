package gst

/*
#cgo pkg-config: gstreamer-1.0 gstreamer-app-1.0

#include "gst.h"

*/
import "C"
import (
	"fmt"
	"github.com/pions/webrtc"
	"unsafe"
)

// Pipeline is a wrapper for a GStreamer Pipeline
type Pipeline struct {
	Pipeline *C.GstElement
	in       chan<- webrtc.RTCSample
	samples  uint32
}

// CreatePipeline creates a GStreamer Pipeline
func CreatePipeline(codec webrtc.TrackType, in chan<- webrtc.RTCSample) *Pipeline {
	pipelineStr := "appsink name=appsink"
	var samples uint32
	switch codec {
	case webrtc.VP8:
		pipelineStr = "videotestsrc ! vp8enc ! " + pipelineStr
	case webrtc.VP9:
		pipelineStr = "videotestsrc ! vp9enc ! " + pipelineStr
	case webrtc.Opus:
		pipelineStr = "audiotestsrc ! opusenc ! " + pipelineStr
	default:
		panic("Unhandled codec " + codec.String())
	}

	pipelineStrUnsafe := C.CString(pipelineStr)
	defer C.free(unsafe.Pointer(pipelineStrUnsafe))
	globalPipeline = &Pipeline{
		Pipeline: C.gstreamer_send_create_pipeline(pipelineStrUnsafe),
		in:       in,
		samples:  samples,
	}

	return globalPipeline
}

// This allows cgo to access pipeline, this will not work if you want multiple
var globalPipeline *Pipeline

// Start starts the GStreamer Pipeline
func (p *Pipeline) Start() {
	C.gstreamer_send_start_pipeline(p.Pipeline)
}

// Stop stops the GStreamer Pipeline
func (p *Pipeline) Stop() {
	C.gstreamer_send_stop_pipeline(p.Pipeline)
}

//export goHandlePipelineBuffer
func goHandlePipelineBuffer(buffer unsafe.Pointer, bufferLen C.int, samples C.int) {
	if globalPipeline != nil {
		globalPipeline.in <- webrtc.RTCSample{C.GoBytes(buffer, bufferLen), samples}
	} else {
		fmt.Println("discarding buffer, globalPipeline not set")
	}
	C.free(buffer)
}
