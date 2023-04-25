// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build e2e
// +build e2e

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/pion/webrtc/v3"
	"github.com/pion/webrtc/v3/pkg/media"
	"github.com/sclevine/agouti"
)

var silentOpusFrame = []byte{0xf8, 0xff, 0xfe} // 20ms, 8kHz, mono

var drivers = map[string]func() *agouti.WebDriver{
	"Chrome": func() *agouti.WebDriver {
		return agouti.ChromeDriver(
			agouti.ChromeOptions("args", []string{
				"--headless",
				"--disable-gpu",
				"--no-sandbox",
			}),
			agouti.Desired(agouti.Capabilities{
				"loggingPrefs": map[string]string{
					"browser": "INFO",
				},
			}),
		)
	},
}

func TestE2E_Audio(t *testing.T) {
	for name, d := range drivers {
		driver := d()
		t.Run(name, func(t *testing.T) {
			if err := driver.Start(); err != nil {
				t.Fatalf("Failed to start WebDriver: %v", err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer func() {
				cancel()
				time.Sleep(50 * time.Millisecond)
				_ = driver.Stop()
			}()
			page, errPage := driver.NewPage()
			if errPage != nil {
				t.Fatalf("Failed to open page: %v", errPage)
			}
			if err := page.SetPageLoad(1000); err != nil {
				t.Fatalf("Failed to load page: %v", err)
			}
			if err := page.SetImplicitWait(1000); err != nil {
				t.Fatalf("Failed to set wait: %v", err)
			}

			chStarted := make(chan struct{})
			chSDP := make(chan *webrtc.SessionDescription)
			chStats := make(chan stats)
			go logParseLoop(ctx, t, page, chStarted, chSDP, chStats)

			pwd, errPwd := os.Getwd()
			if errPwd != nil {
				t.Fatalf("Failed to get working directory: %v", errPwd)
			}
			if err := page.Navigate(
				fmt.Sprintf("file://%s/test.html", pwd),
			); err != nil {
				t.Fatalf("Failed to navigate: %v", err)
			}

			sdp := <-chSDP
			pc, answer, track, errTrack := createTrack(*sdp)
			if errTrack != nil {
				t.Fatalf("Failed to create track: %v", errTrack)
			}
			defer func() {
				_ = pc.Close()
			}()

			answerBytes, errAnsSDP := json.Marshal(answer)
			if errAnsSDP != nil {
				t.Fatalf("Failed to marshal SDP: %v", errAnsSDP)
			}
			var result string
			if err := page.RunScript(
				"pc.setRemoteDescription(JSON.parse(answer))",
				map[string]interface{}{"answer": string(answerBytes)},
				&result,
			); err != nil {
				t.Fatalf("Failed to run script to set SDP: %v", err)
			}

			go func() {
				for {
					if err := track.WriteSample(
						media.Sample{Data: silentOpusFrame, Duration: time.Millisecond * 20},
					); err != nil {
						t.Errorf("Failed to WriteSample: %v", err)
						return
					}
					select {
					case <-time.After(20 * time.Millisecond):
					case <-ctx.Done():
						return
					}
				}
			}()

			select {
			case <-chStarted:
			case <-time.After(5 * time.Second):
				t.Fatal("Timeout")
			}

			<-chStats
			var packetReceived [2]int
			for i := 0; i < 2; i++ {
				select {
				case stat := <-chStats:
					for _, s := range stat {
						if s.Type != "inbound-rtp" {
							continue
						}
						if s.Kind != "audio" {
							t.Errorf("Unused track stat received: %+v", s)
							continue
						}
						packetReceived[i] = s.PacketsReceived
					}
				case <-time.After(5 * time.Second):
					t.Fatal("Timeout")
				}
			}

			packetsPerSecond := packetReceived[1] - packetReceived[0]
			if packetsPerSecond < 45 || 55 < packetsPerSecond {
				t.Errorf("Number of OPUS packets is expected to be: 50/second, got: %d/second", packetsPerSecond)
			}
		})
	}
}

func TestE2E_DataChannel(t *testing.T) {
	for name, d := range drivers {
		driver := d()
		t.Run(name, func(t *testing.T) {
			if err := driver.Start(); err != nil {
				t.Fatalf("Failed to start WebDriver: %v", err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer func() {
				cancel()
				time.Sleep(50 * time.Millisecond)
				_ = driver.Stop()
			}()

			page, errPage := driver.NewPage()
			if errPage != nil {
				t.Fatalf("Failed to open page: %v", errPage)
			}
			if err := page.SetPageLoad(1000); err != nil {
				t.Fatalf("Failed to load page: %v", err)
			}
			if err := page.SetImplicitWait(1000); err != nil {
				t.Fatalf("Failed to set wait: %v", err)
			}

			chStarted := make(chan struct{})
			chSDP := make(chan *webrtc.SessionDescription)
			go logParseLoop(ctx, t, page, chStarted, chSDP, nil)

			pwd, errPwd := os.Getwd()
			if errPwd != nil {
				t.Fatalf("Failed to get working directory: %v", errPwd)
			}
			if err := page.Navigate(
				fmt.Sprintf("file://%s/test.html", pwd),
			); err != nil {
				t.Fatalf("Failed to navigate: %v", err)
			}

			sdp := <-chSDP
			pc, errPc := webrtc.NewPeerConnection(webrtc.Configuration{})
			if errPc != nil {
				t.Fatalf("Failed to create peer connection: %v", errPc)
			}
			defer func() {
				_ = pc.Close()
			}()

			chValid := make(chan struct{})
			pc.OnDataChannel(func(dc *webrtc.DataChannel) {
				dc.OnOpen(func() {
					// Ping
					if err := dc.SendText("hello world"); err != nil {
						t.Errorf("Failed to send data: %v", err)
					}
				})
				dc.OnMessage(func(msg webrtc.DataChannelMessage) {
					// Pong
					if string(msg.Data) != "HELLO WORLD" {
						t.Errorf("expected message from browser: HELLO WORLD, got: %s", string(msg.Data))
					} else {
						chValid <- struct{}{}
					}
				})
			})

			if err := pc.SetRemoteDescription(*sdp); err != nil {
				t.Fatalf("Failed to set remote description: %v", err)
			}
			answer, errAns := pc.CreateAnswer(nil)
			if errAns != nil {
				t.Fatalf("Failed to create answer: %v", errAns)
			}
			if err := pc.SetLocalDescription(answer); err != nil {
				t.Fatalf("Failed to set local description: %v", err)
			}

			answerBytes, errAnsSDP := json.Marshal(answer)
			if errAnsSDP != nil {
				t.Fatalf("Failed to marshal SDP: %v", errAnsSDP)
			}
			var result string
			if err := page.RunScript(
				"pc.setRemoteDescription(JSON.parse(answer))",
				map[string]interface{}{"answer": string(answerBytes)},
				&result,
			); err != nil {
				t.Fatalf("Failed to run script to set SDP: %v", err)
			}

			select {
			case <-chStarted:
			case <-time.After(5 * time.Second):
				t.Fatal("Timeout")
			}
			select {
			case <-chValid:
			case <-time.After(5 * time.Second):
				t.Fatal("Timeout")
			}
		})
	}
}

type stats []struct {
	Kind            string `json:"kind"`
	Type            string `json:"type"`
	PacketsReceived int    `json:"packetsReceived"`
}

func logParseLoop(ctx context.Context, t *testing.T, page *agouti.Page, chStarted chan struct{}, chSDP chan *webrtc.SessionDescription, chStats chan stats) {
	for {
		select {
		case <-time.After(time.Second):
		case <-ctx.Done():
			return
		}
		logs, errLog := page.ReadNewLogs("browser")
		if errLog != nil {
			t.Errorf("Failed to read log: %v", errLog)
			return
		}
		for _, log := range logs {
			k, v, ok := parseLog(log)
			if !ok {
				t.Log(log.Message)
				continue
			}
			switch k {
			case "connection":
				switch v {
				case "connected":
					close(chStarted)
				case "failed":
					t.Error("Browser reported connection failed")
					return
				}
			case "sdp":
				sdp := &webrtc.SessionDescription{}
				if err := json.Unmarshal([]byte(v), sdp); err != nil {
					t.Errorf("Failed to unmarshal SDP: %v", err)
					return
				}
				chSDP <- sdp
			case "stats":
				if chStats == nil {
					break
				}
				s := &stats{}
				if err := json.Unmarshal([]byte(v), &s); err != nil {
					t.Errorf("Failed to parse log: %v", err)
					break
				}
				select {
				case chStats <- *s:
				case <-time.After(10 * time.Millisecond):
				}
			default:
				t.Log(log.Message)
			}
		}
	}
}

func parseLog(log agouti.Log) (string, string, bool) {
	l := strings.SplitN(log.Message, " ", 4)
	if len(l) != 4 {
		return "", "", false
	}
	k, err1 := strconv.Unquote(l[2])
	if err1 != nil {
		return "", "", false
	}
	v, err2 := strconv.Unquote(l[3])
	if err2 != nil {
		return "", "", false
	}
	return k, v, true
}

func createTrack(offer webrtc.SessionDescription) (*webrtc.PeerConnection, *webrtc.SessionDescription, *webrtc.TrackLocalStaticSample, error) {
	pc, errPc := webrtc.NewPeerConnection(webrtc.Configuration{})
	if errPc != nil {
		return nil, nil, nil, errPc
	}

	track, errTrack := webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus}, "audio", "pion")
	if errTrack != nil {
		return nil, nil, nil, errTrack
	}
	if _, err := pc.AddTrack(track); err != nil {
		return nil, nil, nil, err
	}
	if err := pc.SetRemoteDescription(offer); err != nil {
		return nil, nil, nil, err
	}
	answer, errAns := pc.CreateAnswer(nil)
	if errAns != nil {
		return nil, nil, nil, errAns
	}
	if err := pc.SetLocalDescription(answer); err != nil {
		return nil, nil, nil, err
	}
	return pc, &answer, track, nil
}
