package main

import (
	"crypto/md5"
	"fmt"
	"math/rand"
	"net"
	"strconv"

	"github.com/pions/webrtc/internal/sdp"
)

// VP8, recvonly SDP
// TODO RTCPeerConnection.localDescription()
func generateVP8OnlyAnswer() *sdp.SessionDescription {

	iceUsername := randSeq(16)
	icePassword := randSeq(32)

	videoMediaDescription := &sdp.MediaDescription{
		MediaName:      "video 7 RTP/SAVPF 96 97",
		ConnectionData: "IN IP4 127.0.0.1",
		Attributes: []string{
			"rtpmap:96 VP8/90000",
			"rtpmap:97 rtx/90000",
			"fmtp:97 apt=96",
			"rtcp-fb:96 goog-remb",
			"rtcp-fb:96 ccm fir",
			"rtcp-fb:96 nack",
			"rtcp-fb:96 nack pli",
			"extmap:2 urn:ietf:params:rtp-hdrext:toffset",
			"extmap:3 http://www.webrtc.org/experiments/rtp-hdrext/abs-send-time",
			"extmap:4 urn:3gpp:video-orientation",
			"setup:active",
			"mid:video",
			"recvonly",
			"ice-ufrag:" + iceUsername,
			"ice-pwd:" + icePassword,
			"ice-options:renomination",
			"rtcp-mux",
			"rtcp-rsize",
		},
	}

	// Generate only UDP host candidates for ICE
	basePriority := uint16(rand.Uint32() & (1<<16 - 1))
	remoteKey := md5.Sum([]byte(iceUsername + ":" + icePassword))
	for id, c := range hostCandidates() {
		dstPort, err := udpListener(c, remoteKey)
		if err != nil {
			panic(err)
		}

		videoMediaDescription.Attributes = append(videoMediaDescription.Attributes, fmt.Sprintf("candidate:udpcandidate %d udp %d %s %d typ host", id, basePriority, c, dstPort))

		basePriority = basePriority + 1
		dstPort = dstPort + 1
	}
	videoMediaDescription.Attributes = append(videoMediaDescription.Attributes, "end-of-candidates")

	sessionId := strconv.FormatUint(uint64(rand.Uint32())<<32+uint64(rand.Uint32()), 10)
	return &sdp.SessionDescription{
		ProtocolVersion: 0,
		Origin:          "pion-webrtc " + sessionId + " 2 IN IP4 0.0.0.0",
		SessionName:     "-",
		Timing:          []string{"0 0"},
		Attributes: []string{
			"ice-lite",
			// TODO kc5nra proper fingerprint
			"fingerprint:sha-512 4E:DD:25:41:95:51:85:B6:6A:29:42:FF:56:5B:41:47:2C:6C:67:36:7D:97:91:5A:65:C7:E1:76:1B:6E:D3:22:45:B4:9F:DF:EA:93:FF:20:F4:CB:A8:53:AF:50:DA:87:5A:C5:4C:5B:F6:4C:50:DC:D9:29:A3:C0:19:7A:17:48",
			"msid-semantic: WMS *",
			"group:BUNDLE video",
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
