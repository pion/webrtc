package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os/exec"
	"time"

	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"
	"github.com/pion/webrtc/v4/pkg/media/ivfreader"
	gocv "gocv.io/x/gocv"
)

func main() {
	// Serve the static/ folder on http://localhost:8080
	http.Handle("/", http.FileServer(http.Dir("static")))

	// POST /offer will handle the browser's WebRTC offer
	http.HandleFunc("/offer", handleOffer)

	fmt.Println("Listening on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func handleOffer(w http.ResponseWriter, r *http.Request) {
	// 1) Read the Offer from the browser
	var offer webrtc.SessionDescription
	if err := json.NewDecoder(r.Body).Decode(&offer); err != nil {
		http.Error(w, "invalid offer", http.StatusBadRequest)
		return
	}

	// 2) Create a new PeerConnection
	peerConnection, err := webrtc.NewPeerConnection(webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{URLs: []string{"stun:stun.l.google.com:19302"}},
		},
	})
	if err != nil {
		http.Error(w, "failed to create PeerConnection", http.StatusInternalServerError)
		return
	}

	// 3) Create a video track for VP8
	videoTrack, err := webrtc.NewTrackLocalStaticSample(
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeVP8},
		"video", // track id
		"gocv",  // stream id
	)
	if err != nil {
		http.Error(w, "failed to create video track", http.StatusInternalServerError)
		return
	}

	// Add the track to the PeerConnection
	rtpSender, err := peerConnection.AddTrack(videoTrack)
	if err != nil {
		http.Error(w, "failed to add track", http.StatusInternalServerError)
		return
	}

	// Read RTCP (for NACK, etc.) in a separate goroutine
	go func() {
		rtcpBuf := make([]byte, 1500)
		for {
			if _, _, rtcpErr := rtpSender.Read(rtcpBuf); rtcpErr != nil {
				return
			}
		}
	}()

	// 4) Watch for ICE connection state
	iceConnectedCtx, iceConnectedCancel := context.WithCancel(context.Background())
	peerConnection.OnICEConnectionStateChange(func(state webrtc.ICEConnectionState) {
		log.Printf("ICE state: %s\n", state)
		if state == webrtc.ICEConnectionStateConnected {
			iceConnectedCancel()
		}
	})

	// 5) Set the remote description (the browser's Offer)
	if err := peerConnection.SetRemoteDescription(offer); err != nil {
		http.Error(w, "failed to set remote desc", http.StatusInternalServerError)
		return
	}

	// 6) Create an Answer
	answer, err := peerConnection.CreateAnswer(nil)
	if err != nil {
		http.Error(w, "failed to create answer", http.StatusInternalServerError)
		return
	}

	// 7) Gather ICE candidates
	gatherComplete := webrtc.GatheringCompletePromise(peerConnection)
	if err := peerConnection.SetLocalDescription(answer); err != nil {
		http.Error(w, "failed to set local desc", http.StatusInternalServerError)
		return
	}
	<-gatherComplete

	// 8) Write the Answer back to the browser
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(peerConnection.LocalDescription())

	// 9) Once ICE is connected, start reading frames from the camera via GoCV,
	//    pipe them into FFmpeg for VP8 encoding, and push the IVF frames into the track.
	go func() {
		<-iceConnectedCtx.Done()

		if err := startCameraAndStream(videoTrack); err != nil {
			log.Printf("camera streaming error: %v\n", err)
		}
	}()
}

// startCameraAndStream opens the webcam with GoCV, sends raw frames to FFmpeg (via stdin),
// reads IVF from FFmpeg (via stdout), and writes them into the WebRTC video track.
func startCameraAndStream(videoTrack *webrtc.TrackLocalStaticSample) error {
	// Open default camera with GoCV
	webcam, err := gocv.OpenVideoCapture(2)
	if err != nil {
		return fmt.Errorf("cannot open camera: %w", err)
	}
	defer webcam.Close()

	// Set some camera settings if needed
	// e.g. webcam.Set(gocv.VideoCaptureFrameWidth, 640)
	//      webcam.Set(gocv.VideoCaptureFrameHeight, 480)
	// Or rely on defaults

	// Prepare FFmpeg cmd:
	//   -f rawvideo: We feed raw frames
	//   -pixel_format bgr24: Our GoCV frames come in BGR format
	//   -video_size 640x480: must match your actual capture size
	//   -i pipe:0 : read from stdin
	//   Then encode with libvpx -> IVF on stdout
	ffmpeg := exec.Command(
		"ffmpeg",
		"-y",
		"-f", "rawvideo",
		"-pixel_format", "bgr24",
		"-video_size", "640x480",
		"-framerate", "30", // assume ~30fps
		"-i", "pipe:0",
		"-c:v", "libvpx",
		"-b:v", "1M",
		"-f", "ivf",
		"pipe:1",
	)

	stdin, err := ffmpeg.StdinPipe()
	if err != nil {
		return fmt.Errorf("ffmpeg stdin error: %w", err)
	}
	stdout, err := ffmpeg.StdoutPipe()
	if err != nil {
		return fmt.Errorf("ffmpeg stdout error: %w", err)
	}

	// Start FFmpeg
	if err := ffmpeg.Start(); err != nil {
		return fmt.Errorf("failed to start ffmpeg: %w", err)
	}

	// Goroutine to write raw frames to FFmpeg stdin
	go func() {
		defer stdin.Close()

		frame := gocv.NewMat()
		defer frame.Close()

		ticker := time.NewTicker(time.Millisecond * 33) // ~30fps
		defer ticker.Stop()

		for range ticker.C {
			if ok := webcam.Read(&frame); !ok {
				log.Println("cannot read frame from camera")
				continue
			}
			if frame.Empty() {
				continue
			}

			// (Optional) do any OpenCV processing on `frame` here

			// Write raw BGR bytes to FFmpeg
			// frame.DataPtrUint8() points to the underlying byte array
			_, _ = stdin.Write(frame.ToBytes())
		}
	}()

	// Read IVF from FFmpeg stdout; parse frames with ivfreader
	ivf, _, err := ivfreader.NewWith(stdout)
	if err != nil {
		return fmt.Errorf("ivfreader init error: %w", err)
	}
	// Loop reading IVF frames; push them to the video track
	for {
		frame, _, err := ivf.ParseNextFrame()
		if errors.Is(err, io.EOF) {
			log.Println("ffmpeg ended (EOF)")
			break
		}
		if err != nil {
			return fmt.Errorf("ivf parse error: %w", err)
		}
		// Deliver the VP8 frame
		writeErr := videoTrack.WriteSample(media.Sample{
			Data:     frame,
			Duration: time.Second / 30,
		})
		if writeErr != nil {
			return fmt.Errorf("write sample error: %w", writeErr)
		}
	}

	// Wait for ffmpeg to exit
	if err := ffmpeg.Wait(); err != nil {
		return fmt.Errorf("ffmpeg wait error: %w", err)
	}

	return nil
}
