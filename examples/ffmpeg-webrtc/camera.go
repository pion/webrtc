package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"syscall"

	"github.com/pion/webrtc/v2"
	"github.com/pion/webrtc/v2/pkg/media"
)

type Camera struct {
	Name           string `json:"name"`
	Width          int    `json:"width"`
	Height         int    `json:"height"`
	DevicePath     string `json:"device_path"`
	peerConnection *webrtc.PeerConnection
	on             bool
	off            chan bool
}

func (c *Camera) Stream(videoTrack *webrtc.Track) error {
	cmd := exec.Command("ffmpeg", "-framerate", "30", "-f", "v4l2", "-input_format", "h264", "-video_size", fmt.Sprintf("%vx%v", c.Width, c.Height), "-i", c.DevicePath, "-c", "copy", "-f", "h264", "pipe:1")
	fmt.Println(cmd.Args)

	dataPipe, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatal("could not create named pipe. ", err)
	}

	if err := execCmd(cmd); err != nil {
		return err
	}

	framebuffer := make(chan []byte, 60)

	go func() {
		for {
			select {
			case <-c.off:
				return
			default:
				framebytes := make([]byte, 600000)
				n, err := dataPipe.Read(framebytes)
				if err != nil {
					log.Println("could not read pipe. ", err)
				}

				framebuffer <- framebytes[:n]
			}
		}
	}()

	c.off = make(chan bool)

	go func() {
		for {
			select {
			case <-c.off:
				c.peerConnection.Close()
				if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
					log.Println("failed to kill camera process. ", err)
					return
				}
				return
			case f := <-framebuffer:
				sample := media.Sample{
					Data:    f,
					Samples: 90000,
				}

				if err := videoTrack.WriteSample(sample); err != nil {
					log.Fatal("could not write rtp sample. ", err)
					return
				}
			}
		}
	}()

	c.on = true
	return nil
}

func (c *Camera) Stop() {
	if c.on {
		close(c.off)
		c.on = false
	}
}

func (c *Camera) NewPeerConnection(m webrtc.MediaEngine) (*webrtc.PeerConnection, error) {
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	}

	var err error

	c.peerConnection, err = webrtc.NewAPI(webrtc.WithMediaEngine(m)).NewPeerConnection(config)
	if err != nil {
		return nil, errors.New("webrtc could not create peer connection. " + err.Error())
	}

	return c.peerConnection, nil
}

func (c *Camera) AddICECandidate(candidate webrtc.ICECandidateInit) {
	if err := c.peerConnection.AddICECandidate(candidate); err != nil {
		fmt.Println("could not add ice candidate.", err)
	}
}

func execCmd(cmd *exec.Cmd) error {
	logFile, err := os.OpenFile("ffmpeg_log.txt", os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0775)
	if err != nil {
		return errors.New("could not create ffmpeg log. " + err.Error())
	}

	cmd.Stderr = logFile

	if err := cmd.Start(); err != nil {
		return err
	}

	return nil
}
