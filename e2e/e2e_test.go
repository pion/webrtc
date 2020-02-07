// +build e2e

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/sclevine/agouti"

	"github.com/pion/webrtc/v2"
	"github.com/pion/webrtc/v2/pkg/media"
)

var silentOpusFrame = []byte{0xf8, 0xff, 0xfe} // 20ms, 8kHz, mono

func TestE2E(t *testing.T) {
	drivers := map[string]*agouti.WebDriver{
		"Chrome": agouti.ChromeDriver(
			agouti.ChromeOptions(
				"args", []string{
					"--headless",
					"--disable-gpu",
					"--no-sandbox",
				},
			),
			agouti.Desired(
				agouti.Capabilities{
					"loggingPrefs": map[string]string{
						"browser": "INFO",
					},
				},
			),
		),
	}
	for name, d := range drivers {
		driver := d
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			if err := driver.Start(); err != nil {
				t.Fatalf("Failed to start WebDriver: %v", err)
			}
			page, errPage := driver.NewPage()
			if errPage != nil {
				driver.Stop()
				t.Fatalf("Failed to open page: %v", errPage)
			}
			if err := page.SetPageLoad(1000); err != nil {
				t.Fatalf("Failed to load page: %v", err)
			}
			if err := page.SetImplicitWait(1000); err != nil {
				t.Fatalf("Failed to set wait: %v", err)
			}

			type stats []struct {
				Kind            string `json:"kind"`
				Type            string `json:"type"`
				PacketsReceived int    `json:"packetsReceived"`
			}

			chStarted := make(chan struct{})
			chSDP := make(chan *webrtc.SessionDescription)
			chStats := make(chan stats)
			go func() {
				for {
					time.Sleep(time.Second)
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
							s := &stats{}
							if err := json.Unmarshal([]byte(v), &s); err != nil {
								t.Fatal(err)
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
			}()

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
			defer pc.Close()

			answerBytes, errAnsSDP := json.Marshal(answer)
			if errAnsSDP != nil {
				t.Fatalf("Failed to marshal SDP: %v", errAnsSDP)
			}
			var result string
			if err := page.RunScript(
				"pc.setRemoteDescription(new RTCSessionDescription(JSON.parse(answer)))",
				map[string]interface{}{"answer": string(answerBytes)},
				&result,
			); err != nil {
				t.Fatalf("Failed to run script to set SDP: %v", err)
			}

			go func() {
				for {
					if err := track.WriteSample(
						media.Sample{Data: silentOpusFrame, Samples: 960},
					); err != nil {
						t.Fatalf("Failed to WriteSample: %v", err)
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

func createTrack(offer webrtc.SessionDescription) (*webrtc.PeerConnection, *webrtc.SessionDescription, *webrtc.Track, error) {
	mediaEngine := webrtc.MediaEngine{}
	if err := mediaEngine.PopulateFromSDP(offer); err != nil {
		return nil, nil, nil, err
	}
	var payloadType uint8
	for _, videoCodec := range mediaEngine.GetCodecsByKind(webrtc.RTPCodecTypeAudio) {
		if videoCodec.Name == "OPUS" {
			payloadType = videoCodec.PayloadType
			break
		}
	}
	if payloadType == 0 {
		return nil, nil, nil, errors.New("Remote peer does not support VP8")
	}
	api := webrtc.NewAPI(webrtc.WithMediaEngine(mediaEngine))
	pc, errPc := api.NewPeerConnection(webrtc.Configuration{})
	if errPc != nil {
		return nil, nil, nil, errPc
	}

	track, errTrack := pc.NewTrack(payloadType, rand.Uint32(), "video", "pion")
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
