// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js
// +build !js

// ogg-playlist-sctp streams Opus pages from single or multi-track Ogg containers,
// exposes the playlist over a DataChannel, and lets the browser switch tracks.
package main

import (
	"context"
	"embed"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"
	"github.com/pion/webrtc/v4/pkg/media/oggreader"
)

const (
	playlistFile = "playlist.ogg"
	labelAudio   = "audio"
	labelTrack   = "pion"
)

//go:embed web/*
var content embed.FS

type bufferedPage struct {
	payload  []byte
	duration time.Duration
	granule  uint64
}

type oggTrack struct {
	serial uint32
	header *oggreader.OggHeader
	tags   *oggreader.OpusTags

	title   string
	artist  string
	vendor  string
	pages   []bufferedPage
	runtime time.Duration
}

func main() { //nolint:gocognit,cyclop
	addr := flag.String("addr", "localhost:8080", "HTTP listen address")
	flag.Parse()

	tracks, err := parsePlaylist(playlistFile)
	if err != nil {
		log.Fatal(err)
	}
	if len(tracks) == 0 {
		log.Fatal("no playable Opus pages were found in playlist.ogg")
	}

	log.Printf("Loaded %d track(s) from %s", len(tracks), playlistFile)
	for i, t := range tracks {
		log.Printf("  [%d] serial=%d title=%q artist=%q pages=%d duration=%v",
			i+1, t.serial, t.title, t.artist, len(t.pages), t.runtime)
	}

	static, err := fs.Sub(content, "web")
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	fileServer := http.FileServer(http.FS(static))
	mux.Handle("/", fileServer)
	mux.HandleFunc("/whep", func(writer http.ResponseWriter, reader *http.Request) {
		if reader.Method != http.MethodPost {
			http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)

			return
		}

		body, err := io.ReadAll(reader.Body)
		if err != nil {
			http.Error(writer, "failed to read body", http.StatusBadRequest)

			return
		}
		rawSDP := string(body)
		if strings.TrimSpace(rawSDP) == "" {
			http.Error(writer, "empty SDP", http.StatusBadRequest)

			return
		}
		log.Printf("received offer (%d bytes)", len(rawSDP))

		offer := webrtc.SessionDescription{Type: webrtc.SDPTypeOffer, SDP: rawSDP}

		answer, err := handleOffer(tracks, offer) //nolint:contextcheck
		if err != nil {
			log.Printf("error handling offer: %v", err)
			http.Error(writer, err.Error(), http.StatusBadRequest)

			return
		}

		writer.Header().Set("Content-Type", "application/sdp")
		if _, err = writer.Write([]byte(answer.SDP)); err != nil {
			log.Printf("write answer failed: %v", err)
		}
	})

	log.Printf("Serving UI at http://%s ...", *addr)
	log.Fatal(http.ListenAndServe(*addr, mux)) //nolint:gosec
}

//nolint:cyclop
func handleOffer(
	tracks []*oggTrack,
	offer webrtc.SessionDescription,
) (*webrtc.SessionDescription, error) {
	peerConnection, err := webrtc.NewPeerConnection(webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{{
			URLs: []string{"stun:stun.l.google.com:19302"},
		}},
	})
	if err != nil {
		return nil, fmt.Errorf("create PeerConnection: %w", err)
	}

	iceConnectedCtx, iceConnectedCtxCancel := context.WithCancel(context.Background())
	disconnectCtx, disconnectCtxCancel := context.WithCancel(context.Background())
	setupComplete := false
	defer func() {
		if !setupComplete {
			iceConnectedCtxCancel()
			disconnectCtxCancel()
		}
	}()

	audioTrack, err := webrtc.NewTrackLocalStaticSample(
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus},
		labelAudio,
		labelTrack,
	)
	if err != nil {
		return nil, fmt.Errorf("create audio track: %w", err)
	}

	rtpSender, err := peerConnection.AddTrack(audioTrack)
	if err != nil {
		return nil, fmt.Errorf("add track: %w", err)
	}

	go func() {
		rtcpBuf := make([]byte, 1500)
		for {
			if _, _, rtcpErr := rtpSender.Read(rtcpBuf); rtcpErr != nil {
				return
			}
		}
	}()

	playlistChannel, err := peerConnection.CreateDataChannel("playlist", nil)
	if err != nil {
		return nil, fmt.Errorf("create data channel: %w", err)
	}

	var currentTrack atomic.Int32
	switchTrack := make(chan int, 4)

	playlistChannel.OnOpen(func() {
		fmt.Println("playlist data channel open")
		sendPlaylistText(playlistChannel, tracks, int(currentTrack.Load()), true)
	})

	playlistChannel.OnMessage(func(msg webrtc.DataChannelMessage) {
		command := strings.TrimSpace(strings.ToLower(string(msg.Data)))
		limit := len(tracks)
		next := -1
		switch command {
		case "next", "n", "forward":
			next = wrapNext(int(currentTrack.Load()), limit)
		case "prev", "previous", "p", "back":
			next = wrapPrev(int(currentTrack.Load()), limit)
		case "list":
			sendPlaylistText(playlistChannel, tracks, int(currentTrack.Load()), true)
		default:
			if idx, convErr := strconv.Atoi(command); convErr == nil {
				next = normalizeIndex(idx-1, limit)
			}
		}

		if next < 0 || next == int(currentTrack.Load()) {
			return
		}

		currentTrack.Store(int32(next)) //nolint:gosec
		select {
		case switchTrack <- next:
		default:
		}
		sendPlaylistText(playlistChannel, tracks, next, true)
	})

	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		fmt.Printf("Connection State has changed %s\n", connectionState.String())
		if connectionState == webrtc.ICEConnectionStateConnected {
			iceConnectedCtxCancel()
		}
	})

	peerConnection.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		fmt.Printf("Peer Connection State has changed: %s\n", state.String())

		if state == webrtc.PeerConnectionStateFailed || state == webrtc.PeerConnectionStateClosed {
			disconnectCtxCancel()
		}
	})

	go func() {
		<-iceConnectedCtx.Done()
		stream(tracks, audioTrack, &currentTrack, switchTrack, playlistChannel, disconnectCtx)
	}()

	go func() {
		<-disconnectCtx.Done()
		if closeErr := peerConnection.Close(); closeErr != nil {
			fmt.Printf("cannot close peerConnection: %v\n", closeErr)
		}
	}()

	//nolint:contextcheck // webrtc API does not take context for SetRemoteDescription
	if err = peerConnection.SetRemoteDescription(offer); err != nil {
		return nil, fmt.Errorf("set remote description: %w", err)
	}

	answer, err := peerConnection.CreateAnswer(nil)
	if err != nil {
		return nil, fmt.Errorf("create answer: %w", err)
	}

	gatherComplete := webrtc.GatheringCompletePromise(peerConnection)

	if err = peerConnection.SetLocalDescription(answer); err != nil {
		return nil, fmt.Errorf("set local description: %w", err)
	}

	<-gatherComplete
	setupComplete = true

	return peerConnection.LocalDescription(), nil
}

func stream(
	tracks []*oggTrack,
	audioTrack *webrtc.TrackLocalStaticSample,
	currentTrack *atomic.Int32,
	switchTrack <-chan int,
	playlistChannel *webrtc.DataChannel,
	ctx context.Context,
) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		index := normalizeIndex(int(currentTrack.Load()), len(tracks))
		track := tracks[index]
		sendNowPlayingText(playlistChannel, track, index)

		for i := 0; i < len(track.pages); i++ {
			page := track.pages[i]
			if err := audioTrack.WriteSample(media.Sample{Data: page.payload, Duration: page.duration}); err != nil {
				if errors.Is(err, io.ErrClosedPipe) {
					return
				}
				panic(err)
			}

			wait := time.After(page.duration)
			select {
			case <-ctx.Done():
				return
			case next := <-switchTrack:
				currentTrack.Store(int32(normalizeIndex(next, len(tracks)))) //nolint:gosec

				goto nextTrack
			case <-wait:
			}
		}

	nextTrack:
	}
}

func parsePlaylist(path string) ([]*oggTrack, error) { //nolint:cyclop
	cleaned := filepath.Clean(path)
	if filepath.IsAbs(cleaned) || strings.Contains(cleaned, "..") {
		return nil, fmt.Errorf("invalid playlist path: %q", path) //nolint:err113
	}
	cleaned = filepath.Base(cleaned)

	file, err := os.Open(cleaned) //nolint:gosec // path is validated and confined to local directory
	if err != nil {
		return nil, fmt.Errorf("open playlist %q: %w", cleaned, err)
	}
	defer func() {
		if cErr := file.Close(); cErr != nil {
			fmt.Printf("cannot close ogg file: %v\n", cErr)
		}
	}()

	reader, err := oggreader.NewWithOptions(file, oggreader.WithDoChecksum(false))
	if err != nil {
		return nil, fmt.Errorf("create ogg reader: %w", err)
	}

	tracks := map[uint32]*oggTrack{}
	var order []uint32
	lastGranule := map[uint32]uint64{}

	for {
		payload, pageHeader, parseErr := reader.ParseNextPage()
		if errors.Is(parseErr, io.EOF) {
			break
		}
		if parseErr != nil {
			return nil, fmt.Errorf("parse ogg page: %w", parseErr)
		}

		track := ensureTrack(tracks, pageHeader.Serial, &order)
		if headerType, ok := pageHeader.HeaderType(payload); ok { //nolint:nestif
			switch headerType {
			case oggreader.HeaderOpusID:
				header, headerErr := oggreader.ParseOpusHead(payload)
				if headerErr != nil {
					return nil, fmt.Errorf("parse OpusHead: %w", headerErr)
				}
				track.header = header

				continue
			case oggreader.HeaderOpusTags:
				tags, tagErr := oggreader.ParseOpusTags(payload)
				if tagErr != nil {
					return nil, fmt.Errorf("parse OpusTags: %w", tagErr)
				}
				track.tags = tags
				track.title, track.artist = extractMetadata(tags)
				if track.vendor == "" {
					track.vendor = tags.Vendor
				}

				continue
			default:
			}
		}

		if track.header == nil {
			continue
		}

		duration := pageDuration(track.header, pageHeader.GranulePosition, lastGranule[track.serial])
		lastGranule[track.serial] = pageHeader.GranulePosition
		track.pages = append(track.pages, bufferedPage{
			payload:  payload,
			duration: duration,
			granule:  pageHeader.GranulePosition,
		})
		track.runtime += duration
	}

	var ordered []*oggTrack
	for _, serial := range order {
		track := tracks[serial]
		if len(track.pages) == 0 {
			continue
		}
		if track.title == "" {
			track.title = fmt.Sprintf("Track %d", len(ordered)+1)
		}
		ordered = append(ordered, track)
	}

	return ordered, nil
}

func ensureTrack(tracks map[uint32]*oggTrack, serial uint32, order *[]uint32) *oggTrack {
	track, ok := tracks[serial]
	if ok {
		return track
	}

	track = &oggTrack{serial: serial, title: fmt.Sprintf("serial-%d", serial)}
	tracks[serial] = track
	*order = append(*order, serial)

	return track
}

func extractMetadata(tags *oggreader.OpusTags) (title, artist string) {
	for _, c := range tags.UserComments {
		switch strings.ToLower(c.Comment) {
		case "title":
			title = c.Value
		case "artist":
			artist = c.Value
		}
	}

	return title, artist
}

func pageDuration(header *oggreader.OggHeader, granule, last uint64) time.Duration {
	sampleRate := header.SampleRate
	if sampleRate == 0 {
		sampleRate = 48000
	}

	if granule <= last {
		return 20 * time.Millisecond
	}

	sampleCount := int64(granule - last) //nolint:gosec
	if sampleCount <= 0 {
		return 20 * time.Millisecond
	}

	ns := float64(sampleCount) / float64(sampleRate) * float64(time.Second)

	return time.Duration(ns)
}

func wrapNext(current, limit int) int {
	if limit == 0 {
		return 0
	}

	return (current + 1) % limit
}

func wrapPrev(current, limit int) int {
	if limit == 0 {
		return 0
	}
	if current == 0 {
		return limit - 1
	}

	return current - 1
}

func normalizeIndex(i, limit int) int {
	if limit == 0 {
		return 0
	}
	if i < 0 {
		return 0
	}
	if i >= limit {
		return limit - 1
	}

	return i
}

func sendPlaylistText(dc *webrtc.DataChannel, tracks []*oggTrack, current int, includeNow bool) {
	if dc == nil || dc.ReadyState() != webrtc.DataChannelStateOpen {
		return
	}

	var str strings.Builder
	fmt.Fprintf(&str, "playlist|%d\n", normalizeIndex(current, len(tracks)))
	for i, t := range tracks {
		fmt.Fprintf(
			&str, "track|%d|%d|%d|%s|%s\n", i, t.serial, t.runtime.Milliseconds(),
			cleanText(t.title),
			cleanText(t.artist),
		)
	}
	if includeNow && len(tracks) > 0 {
		next := normalizeIndex(current, len(tracks))
		str.WriteString(nowLine(tracks[next], next))
	}

	if err := dc.SendText(str.String()); err != nil {
		fmt.Printf("unable to send playlist: %v\n", err)
	}
}

func sendNowPlayingText(dc *webrtc.DataChannel, track *oggTrack, index int) {
	if dc == nil || dc.ReadyState() != webrtc.DataChannelStateOpen {
		return
	}

	line := nowLine(track, index)
	if err := dc.SendText(line); err != nil {
		fmt.Printf("unable to send now-playing: %v\n", err)
	}
}

func nowLine(track *oggTrack, index int) string {
	comments := ""
	if track.tags != nil && len(track.tags.UserComments) > 0 {
		pairs := make([]string, 0, len(track.tags.UserComments))
		for _, c := range track.tags.UserComments {
			pairs = append(pairs, cleanText(c.Comment)+"="+cleanText(c.Value))
		}
		comments = strings.Join(pairs, ",")
	}

	return fmt.Sprintf(
		"now|%d|%d|%d|%d|%d|%s|%s|%s|%s\n",
		index,
		track.serial,
		track.header.Channels,
		track.header.SampleRate,
		track.runtime.Milliseconds(),
		cleanText(track.title),
		cleanText(track.artist),
		cleanText(track.vendor),
		comments,
	)
}

func cleanText(v string) string {
	out := strings.ReplaceAll(v, "\n", " ")

	return strings.ReplaceAll(out, "|", "/")
}
