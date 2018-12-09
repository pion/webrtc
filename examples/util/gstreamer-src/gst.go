package gst

/*
#cgo pkg-config: gstreamer-1.0 gstreamer-app-1.0

#include "gst.h"

*/
import "C"
import (
	"fmt"
	"sync"
	"unsafe"

	"github.com/pions/webrtc"
	"github.com/pions/webrtc/pkg/media"
)

func init() {
	go C.gstreamer_send_start_mainloop()
}

// Pipeline is a wrapper for a GStreamer Pipeline
type Pipeline struct {
	Pipeline  *C.GstElement
	in        chan<- media.RTCSample
	id        int
	codecName string
}

var pipelines = make(map[int]*Pipeline)
var pipelinesLock sync.Mutex

// CreatePipeline creates a GStreamer Pipeline
func CreatePipeline(codecName string, in chan<- media.RTCSample) *Pipeline {
	pipelineStr := "appsink name=appsink"
	switch codecName {
	case webrtc.VP8:
		pipelineStr = "videotestsrc ! vp8enc ! " + pipelineStr
	case webrtc.VP9:
		pipelineStr = "videotestsrc ! vp9enc ! " + pipelineStr
	case webrtc.H264:
		pipelineStr = "videotestsrc ! video/x-raw,format=I420 ! x264enc bframes=0 speed-preset=veryfast key-int-max=60 ! video/x-h264,stream-format=byte-stream ! " + pipelineStr
	case webrtc.Opus:
		pipelineStr = "audiotestsrc ! opusenc ! " + pipelineStr
	default:
		panic("Unhandled codec " + codecName)
	}

	pipelineStrUnsafe := C.CString(pipelineStr)
	defer C.free(unsafe.Pointer(pipelineStrUnsafe))

	pipelinesLock.Lock()
	defer pipelinesLock.Unlock()

	pipeline := &Pipeline{
		Pipeline:  C.gstreamer_send_create_pipeline(pipelineStrUnsafe),
		in:        in,
		id:        len(pipelines),
		codecName: codecName,
	}

	pipelines[pipeline.id] = pipeline
	return pipeline
}

// Start starts the GStreamer Pipeline
func (p *Pipeline) Start() {
	C.gstreamer_send_start_pipeline(p.Pipeline, C.int(p.id))
}

// Stop stops the GStreamer Pipeline
func (p *Pipeline) Stop() {
	C.gstreamer_send_stop_pipeline(p.Pipeline)
}

const (
	videoClockRate = 90000
	audioClockRate = 48000
)

//export goHandlePipelineBuffer
func goHandlePipelineBuffer(buffer unsafe.Pointer, bufferLen C.int, duration C.int, pipelineID C.int) {
	pipelinesLock.Lock()
	defer pipelinesLock.Unlock()

	if pipeline, ok := pipelines[int(pipelineID)]; ok {
		var samples uint32
		if pipeline.codecName == webrtc.Opus {
			samples = uint32(audioClockRate * (float32(duration) / 1000000000))
		} else {
			samples = uint32(videoClockRate * (float32(duration) / 1000000000))
		}
		pipeline.in <- media.RTCSample{Data: C.GoBytes(buffer, bufferLen), Samples: samples}
	} else {
		fmt.Printf("discarding buffer, no pipeline with id %d", int(pipelineID))
	}
	C.free(buffer)
}
