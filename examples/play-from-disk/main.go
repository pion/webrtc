//go:build !js
// +build !js

package main

/*
#cgo pkg-config: libavformat
#include <libavformat/avformat.h>
*/
import "C"
import (
	"fmt"
	"os"
	"time"
	"unsafe"

	"github.com/pion/webrtc/v3"
	"github.com/pion/webrtc/v3/examples/internal/signal"
	"github.com/pion/webrtc/v3/pkg/media"
)

const (
	audioFileName   = "output.ogg"
	videoFileName   = "output.ivf"
	oggPageDuration = time.Millisecond * 20
)

func main() {
	// Create a new RTCPeerConnection
	peerConnection, err := webrtc.NewPeerConnection(webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	})
	if err != nil {
		panic(err)
	}
	defer func() {
		if cErr := peerConnection.Close(); cErr != nil {
			fmt.Printf("cannot close peerConnection: %v\n", cErr)
		}
	}()

	// Create a video track
	videoTrack, videoTrackErr := webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeH264}, "video", "pion")
	if videoTrackErr != nil {
		panic(videoTrackErr)
	}

	videoRTPSender, videoTrackErr := peerConnection.AddTrack(videoTrack)
	if videoTrackErr != nil {
		panic(videoTrackErr)
	}

	// Read incoming RTCP packets
	// Before these packets are returned they are processed by interceptors. For things
	// like NACK this needs to be called.
	go func() {
		rtcpBuf := make([]byte, 1500)
		for {
			if _, _, rtcpErr := videoRTPSender.Read(rtcpBuf); rtcpErr != nil {
				return
			}
		}
	}()

	// Create a audio track
	audioTrack, audioTrackErr := webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus}, "audio", "pion")
	if audioTrackErr != nil {
		panic(audioTrackErr)
	}

	audioRTPSender, audioTrackErr := peerConnection.AddTrack(audioTrack)
	if audioTrackErr != nil {
		panic(audioTrackErr)
	}

	// Read incoming RTCP packets
	// Before these packets are returned they are processed by interceptors. For things
	// like NACK this needs to be called.
	go func() {
		rtcpBuf := make([]byte, 1500)
		for {
			if _, _, rtcpErr := audioRTPSender.Read(rtcpBuf); rtcpErr != nil {
				return
			}
		}
	}()

	completeCh := make(chan bool)

	// Set the handler for ICE connection state
	// This will notify you when the peer has connected/disconnected
	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		fmt.Printf("Connection State has changed %s \n", connectionState.String())
		if connectionState == webrtc.ICEConnectionStateConnected {
			completeCh <- true
		}
	})

	// Set the handler for Peer connection state
	// This will notify you when the peer has connected/disconnected
	peerConnection.OnConnectionStateChange(func(s webrtc.PeerConnectionState) {
		fmt.Printf("Peer Connection State has changed: %s\n", s.String())

		if s == webrtc.PeerConnectionStateFailed {
			// Wait until PeerConnection has had no network activity for 30 seconds or another failure. It may be reconnected using an ICE Restart.
			// Use webrtc.PeerConnectionStateDisconnected if you are interested in detecting faster timeout.
			// Note that the PeerConnection may come back from PeerConnectionStateDisconnected.
			fmt.Println("Peer Connection has gone to failed exiting")
			os.Exit(0)
		}
	})

	// Wait for the offer to be pasted
	offer := webrtc.SessionDescription{}
	signal.Decode(signal.MustReadStdin(), &offer)

	// Set the remote SessionDescription
	if err = peerConnection.SetRemoteDescription(offer); err != nil {
		panic(err)
	}

	// Create answer
	answer, err := peerConnection.CreateAnswer(nil)
	if err != nil {
		panic(err)
	}

	// Create channel that is blocked until ICE Gathering is complete
	gatherComplete := webrtc.GatheringCompletePromise(peerConnection)

	// Sets the LocalDescription, and starts our UDP listeners
	if err = peerConnection.SetLocalDescription(answer); err != nil {
		panic(err)
	}

	// Block until ICE Gathering is complete, disabling trickle ICE
	// we do this because we only can exchange one signaling message
	// in a production application you should exchange ICE Candidates via OnICECandidate
	<-gatherComplete

	// Output the answer in base64 so we can paste it in browser
	fmt.Println(signal.Encode(*peerConnection.LocalDescription()))

	<-completeCh

	// create a new hls demuxer
	avformatctx := C.avformat_alloc_context()
	if avformatctx == nil {
		panic("failed")
	}

	curl := C.CString("https://devstreaming-cdn.apple.com/videos/streaming/examples/bipbop_4x3/bipbop_4x3_variant.m3u8")
	defer C.free(unsafe.Pointer(curl))

	if averr := C.avformat_open_input(&avformatctx, curl, nil, nil); averr < 0 {
		panic("failed")
	}

	if averr := C.avformat_find_stream_info(avformatctx, nil); averr < 0 {
		panic("failed")
	}

	streams := make([]*C.AVStream, avformatctx.nb_streams)
	for i, stream := range (*[1 << 30]*C.AVStream)(unsafe.Pointer(avformatctx.streams))[:avformatctx.nb_streams] {
		streams[i] = stream
	}

	for {
		p := C.av_packet_alloc()
		if averr := C.av_read_frame(avformatctx, p); averr < 0 {
			panic("failed")
		}
		data := (*[1<<30]byte)(unsafe.Pointer(p.data))[0:p.size]
		stream := streams[p.stream_index]
		duration := time.Duration(float64(p.duration) * float64(stream.time_base.num) / float64(stream.time_base.den) * float64(time.Second))
		sample := media.Sample{Data: data[:], Duration: duration}
		if p.stream_index == 0 {
			videoTrack.WriteSample(sample)
		} else {
			audioTrack.WriteSample(sample)
		}
		C.av_packet_free(&p)
	}
}
