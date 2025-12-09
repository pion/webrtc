package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/pion/randutil"
	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"
)

var peerConnection *webrtc.PeerConnection

// Pipeline management structures
type MediaPipeline struct {
	Cmd      *exec.Cmd
	DataPipe io.ReadCloser
}

var (
	videoPipeline       *MediaPipeline
	audioPipeline       *MediaPipeline
	mediaProcessingDone chan bool
)

func doSignaling(res http.ResponseWriter, req *http.Request) {
	// Decode the offer from the request body
	var offer webrtc.SessionDescription
	if err := json.NewDecoder(req.Body).Decode(&offer); err != nil {
		panic(err)
	}

	if err := peerConnection.SetRemoteDescription(offer); err != nil {
		panic(err)
	}

	gatherComplete := webrtc.GatheringCompletePromise(peerConnection)

	answer, err := peerConnection.CreateAnswer(nil)
	if err != nil {
		panic(err)
	} else if err = peerConnection.SetLocalDescription(answer); err != nil {
		panic(err)
	}

	<-gatherComplete

	response, err := json.Marshal(*peerConnection.LocalDescription())
	if err != nil {
		panic(err)
	}

	res.Header().Set("Content-Type", "application/json")
	if _, err := res.Write(response); err != nil {
		panic(err)
	}
}

func createPeerConnection(res http.ResponseWriter, req *http.Request) {
	if peerConnection.ConnectionState() != webrtc.PeerConnectionStateNew {
		panic(fmt.Sprintf("createPeerConnection called in non-new state (%s)", peerConnection.ConnectionState()))
	}

	doSignaling(res, req)
	fmt.Println("PeerConnection has been created")
}

func addVideo(res http.ResponseWriter, req *http.Request) {
	var err error
	videoTrack, err := webrtc.NewTrackLocalStaticSample(
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeH264},
		fmt.Sprintf("video-%d", randutil.NewMathRandomGenerator().Uint32()),
		fmt.Sprintf("video-%d", randutil.NewMathRandomGenerator().Uint32()),
	)
	if err != nil {
		panic(err)
	}

	rtpVideoSender, err := peerConnection.AddTrack(videoTrack)
	if err != nil {
		panic(err)
	}

	go func() {
		rtcpBuf := make([]byte, 1500)
		for {
			if _, _, rtcpErr := rtpVideoSender.Read(rtcpBuf); rtcpErr != nil {
				return
			}
		}
	}()

	audioTrack, err := webrtc.NewTrackLocalStaticSample(
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus},
		fmt.Sprintf("audio-%d", randutil.NewMathRandomGenerator().Uint32()),
		fmt.Sprintf("audio-%d", randutil.NewMathRandomGenerator().Uint32()),
	)
	if err != nil {
		panic(err)
	}

	rtpAudioSender, err := peerConnection.AddTrack(audioTrack)
	if err != nil {
		panic(err)
	}

	go func() {
		rtcpBuf := make([]byte, 1500)
		for {
			if _, _, rtcpErr := rtpAudioSender.Read(rtcpBuf); rtcpErr != nil {
				return
			}
		}
	}()

	doSignaling(res, req)

	mediaProcessingDone = make(chan bool, 1)

	go func() {
		if err := startAudioProcessingLoop(audioTrack); err != nil {
			fmt.Printf("Audio processing error: %v\n", err)
		}
	}()

	go func() {
		if err := startVideoProcessingLoop(videoTrack); err != nil {
			fmt.Printf("Video processing error: %v\n", err)
		}
	}()

}

func startVideoProcessingLoop(videoTrack *webrtc.TrackLocalStaticSample) error {
	for videoTrack == nil {
		time.Sleep(100 * time.Millisecond)
	}

	// FFmpeg version (commented out)
	cmd := exec.Command(
		"ffmpeg",
		"-hide_banner",
		"-fflags", "nobuffer",
		"-flags", "low_delay",
		"-f", "v4l2",
		"-input_format", "yuv420p",
		"-video_size", "640x480",
		"-framerate", "30",
		"-i", "/dev/video0",
		"-c:v", "libx264",
		"-preset", "veryfast",
		"-tune", "zerolatency",
		"-g", "30",
		"-crf", "23",
		"-f", "h264",
		"-bsf:v", "h264_mp4toannexb",
		"-")

	// GStreamer version (default)
	// cmd := exec.Command(
	// 	"gst-launch-1.0",
	// 	"-q",
	// 	"v4l2src",
	// 	"device=/dev/video0",
	// 	"!", "video/x-raw,format=YUY2,width=640,height=480,framerate=30/1",
	// 	"!", "videoconvert",
	// 	"!", "x264enc",
	// 	"bitrate=800", "key-int-max=30",
	// 	"speed-preset=fast", "tune=zerolatency",
	// 	"!", "h264parse",
	// 	"!", "video/x-h264,stream-format=byte-stream",
	// 	"!", "fdsink", "fd=1", "sync=false")
	cmd.Stderr = os.Stderr

	// Get stdout pipe to read H.264 data
	dataPipe, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("create stdout pipe failed: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start gstreamer failed: %w", err)
	}

	videoPipeline = &MediaPipeline{
		Cmd:      cmd,
		DataPipe: dataPipe,
	}

	buffer := make([]byte, 1024*1024)
	frameDuration := time.Millisecond * 33 // 30fps
	for {
		select {
		case <-mediaProcessingDone:
			return nil
		default:
		}
		n, err := videoPipeline.DataPipe.Read(buffer)
		if err != nil {
			if err == io.EOF {
				time.Sleep(1 * time.Second)
				continue
			}
			time.Sleep(100 * time.Millisecond)
			continue
		}
		if n > 0 {
			if err := videoTrack.WriteSample(media.Sample{Data: buffer[:n], Duration: frameDuration}); err != nil {
				if errors.Is(err, io.ErrClosedPipe) {
					return fmt.Errorf("PeerConnection closed")
				}
				time.Sleep(100 * time.Millisecond)
			}
		}
	}
}
func startAudioProcessingLoop(audioTrack *webrtc.TrackLocalStaticSample) error {
	// FFmpeg version (commented out)
	/*
		cmd := exec.Command(
			"ffmpeg",
			"-hide_banner",
			"-fflags", "nobuffer",
			"-flags", "low_delay",
			"-strict", "experimental",
			"-f", "alsa",
			"-i", "hw:2,0",
			"-acodec", "libopus",
			"-ar", "48000",
			"-ac", "2",
			"-b:a", "48k",
			"-application", "voip",
			"-frame_duration", "20",
			"-compression_level", "4",
			"-map", "0:a",
			"-f", "data",
			"-")
	*/

	// GStreamer version (default)
	cmd := exec.Command(
		"gst-launch-1.0",
		"-q",
		"alsasrc",
		"device=hw:2,0",
		"!", "audio/x-raw,rate=48000,channels=2",
		"!", "queue", "max-size-buffers=1", "leaky=1",
		"!", "opusenc", "bitrate=32000", "frame-size=20",
		"dtx=true", "inband-fec=true", "packet-loss-percentage=10",
		"!", "audio/x-opus",
		"!", "fdsink", "fd=1", "sync=false")

	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start gstreamer audio: %w", err)
	}

	// Create audio pipeline structure
	audioPipeline = &MediaPipeline{
		Cmd:      cmd,
		DataPipe: stdout,
	}
	go func() {
		defer stderr.Close()
		buf := make([]byte, 1024)
		for {
			_, err := stderr.Read(buf)
			if err != nil {
				break
			}
		}
	}()

	buf := make([]byte, 1024)
	for {
		select {
		case <-mediaProcessingDone:
			fmt.Println("Received stop signal, exiting audio processing loop")
			return nil
		default:
			n, err := audioPipeline.DataPipe.Read(buf)
			if err != nil {
				if err == io.EOF {
					time.Sleep(1 * time.Second)
					continue
				}
				time.Sleep(100 * time.Millisecond)
				continue
			}
			if n > 0 {
				if err := audioTrack.WriteSample(media.Sample{Data: buf[:n], Duration: time.Millisecond * 20}); err != nil {
					if errors.Is(err, io.ErrClosedPipe) {
						return fmt.Errorf("PeerConnection closed")
					}
					time.Sleep(100 * time.Millisecond)
				}
			}
		}
	}
}
func removeVideo(res http.ResponseWriter, req *http.Request) {
	stop()
	if senders := peerConnection.GetSenders(); len(senders) != 0 {
		for _, sender := range senders {
			if sender == nil {
				continue
			}

			track := sender.Track()
			if track == nil {
				continue
			}

			if track.Kind() == webrtc.RTPCodecTypeVideo || track.Kind() == webrtc.RTPCodecTypeAudio {
				if err := peerConnection.RemoveTrack(sender); err != nil {
					fmt.Printf("Failed to remove %s track: %v\n", track.Kind(), err)
				} else {
					fmt.Printf("Removed %s track\n", track.Kind())
				}
			}
		}
	}
	doSignaling(res, req)
	fmt.Println("Video and audio tracks have been removed")
}

func stop() {
	if mediaProcessingDone != nil {
		select {
		case mediaProcessingDone <- true:
		default:
		}
	}
	if videoPipeline != nil && videoPipeline.Cmd != nil && videoPipeline.Cmd.Process != nil {
		if err := videoPipeline.Cmd.Process.Kill(); err != nil {
			fmt.Printf("Failed to stop camera process: %v\n", err)
		}
		if videoPipeline.DataPipe != nil {
			videoPipeline.DataPipe.Close()
		}
	}
	videoPipeline = nil
	if audioPipeline != nil && audioPipeline.Cmd != nil && audioPipeline.Cmd.Process != nil {
		if err := audioPipeline.Cmd.Process.Kill(); err != nil {
			fmt.Printf("Failed to stop audio process: %v\n", err)
		}
		if audioPipeline.DataPipe != nil {
			audioPipeline.DataPipe.Close()
		}
	}
	audioPipeline = nil
}

func main() {
	var err error
	if peerConnection, err = webrtc.NewPeerConnection(webrtc.Configuration{}); err != nil {
		panic(err)
	}
	defer func() {
		if cErr := peerConnection.Close(); cErr != nil {
			fmt.Printf("cannot close peerConnection: %v\n", cErr)
		}
	}()

	peerConnection.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		fmt.Printf("Peer Connection State has changed: %s\n", state.String())

		if state == webrtc.PeerConnectionStateFailed {
			fmt.Println("Peer Connection has gone to failed exiting")
			os.Exit(0)
		}

		if state == webrtc.PeerConnectionStateClosed {
			fmt.Println("Peer Connection has gone to closed exiting")
			os.Exit(0)
		}
	})

	http.Handle("/", http.FileServer(http.Dir(".")))
	http.HandleFunc("/createPeerConnection", createPeerConnection)
	http.HandleFunc("/addVideo", addVideo)
	http.HandleFunc("/removeVideo", removeVideo)

	// Start HTTP server in a goroutine
	go func() {
		fmt.Println("Open http://localhost:8080 to access this demo")
		panic(http.ListenAndServe(":8080", nil))
	}()

	// Block forever to keep the program running
	select {}
}
