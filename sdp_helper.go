package main

import (
	"fmt"
	"math/rand"
	"net"
	"strconv"

	"github.com/pions/webrtc/internal/sdp"
)

// VP8, recvonly SDP
// TODO RTCPeerConnection.localDescription()
func generateVP8OnlyAnswer() *sdp.SessionDescription {

	videoMediaDescription := &sdp.MediaDescription{
		MediaName:      "video 9 UDP/TLS/RTP/SAVPF 96 97",
		ConnectionData: "IN IP4 0.0.0.0",
		Attributes: []string{
			"rtcp:9 IN IP4 0.0.0.0",
			// TODO kc5nra proper fingerprint
			"fingerprint:sha-256 26:12:82:57:36:FC:E3:3A:51:2E:A0:A8:33:BA:CC:A1:CD:9E:0B:FD:B5:CE:00:8C:23:F8:3B:A8:C4:B0:17:87",
			"setup:active",
			"mid:video",
			"recvonly",
			"rtcp-mux",
			"rtcp-rsize",
			"rtpmap:96 VP8/90000",
			"rtcp-fb:96 goog-remb",
			"rtcp-fb:96 transport-cc",
			"rtcp-fb:96 ccm fir",
			"rtcp-fb:96 nack",
			"rtcp-fb:96 nack pli",
			"rtpmap:97 rtx/90000",
			"fmtp:97 apt=96",
			"ice-ufrag:" + randSeq(4),
			"ice-pwd:" + randSeq(24),
		},
	}

	// Generate only UDP host candidates for ICE
	basePriority := rand.Int()
	for _, c := range hostCandidates() {
		id := rand.Int()
		videoMediaDescription.Attributes = append(videoMediaDescription.Attributes, fmt.Sprintf("candidate:%d 1 UDP %d %s 1816 typ host", id, basePriority, c))
		videoMediaDescription.Attributes = append(videoMediaDescription.Attributes, fmt.Sprintf("candidate:%d 2 UDP %d %s 1816 typ host", id, basePriority, c))
		basePriority = basePriority + 1
	}

	sessionId := strconv.FormatUint(uint64(rand.Uint32())<<32+uint64(rand.Uint32()), 10)
	return &sdp.SessionDescription{
		ProtocolVersion: 0,
		Origin:          "- " + sessionId + " 2 IN IP4 127.0.0.1",
		SessionName:     "-",
		Timing:          []string{"0 0"},
		Attributes: []string{
			"group:BUNDLE audio video",
			"msid-semantic: WMS",
		},
		MediaDescriptions: []*sdp.MediaDescription{
			videoMediaDescription,
		},
	}
}

//TODO Sean-Der temporary
func randSeq(n int) string {
	letters := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

//TODO Sean-Der temporary
func hostCandidates() (ips []string) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ips
	}

	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 {
			continue // interface down
		}
		if iface.Flags&net.FlagLoopback != 0 {
			continue // loopback interface
		}
		addrs, err := iface.Addrs()
		if err != nil {
			return ips
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() {
				continue
			}
			ip = ip.To4()
			if ip == nil {
				continue // not an ipv4 address
			}
			ips = append(ips, ip.String())
		}
	}
	return ips
}
