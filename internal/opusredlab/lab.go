// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build opusred && !js

// Package opusredlab runs isolated, opt-in Opus RED qualification trials.
package opusredlab

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/pion/interceptor"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/internal/opusredmedia"
	"github.com/pion/webrtc/v4/internal/opusredtest"
)

const (
	packetCount                = 300
	packetInterval             = 20 * time.Millisecond
	maximumRTPSize             = 1200
	expectedInterceptorSHA     = "ac462482153f668aacc5ed9ae2e04560bc9fe517"
	expectedInterceptorVersion = "v0.0.0-20261005182723-ac462482153f"
)

// Assertion is a directly checked property of one trial.
type Assertion struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
	Detail string `json:"detail"`
}

// Environment records measured versions and the separately identified source pin.
type Environment struct {
	GoVersion                string            `json:"goVersion"`
	CheckoutSHA              string            `json:"checkoutSHA"`
	BuildVCSRevision         string            `json:"buildVCSRevision,omitempty"`
	BuildVCSModified         string            `json:"buildVCSModified,omitempty"`
	BinaryDependencyStatus   string            `json:"binaryDependencyStatus"`
	BinaryInterceptorPath    string            `json:"binaryInterceptorPath,omitempty"`
	BinaryInterceptorVersion string            `json:"binaryInterceptorVersion,omitempty"`
	InterceptorPath          string            `json:"interceptorPath"`
	InterceptorVersion       string            `json:"interceptorVersion"`
	InterceptorOriginSHA     string            `json:"interceptorOriginSHA,omitempty"`
	ExpectedInterceptorSHA   string            `json:"expectedInterceptorSHA"`
	FFmpegVersion            string            `json:"ffmpegVersion"`
	Modules                  map[string]string `json:"modules"`
	Error                    string            `json:"error,omitempty"`
}

// Audio holds independently decoded reference and received corpora.
type Audio struct {
	Reference opusredmedia.AudioEvidence `json:"reference"`
	Received  opusredmedia.AudioEvidence `json:"received"`
}

// Teardown records closure and reader completion before a result is published.
type Teardown struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

// Result is the evidence for an individually requested trial.
type Result struct {
	ID               string                     `json:"id"`
	Status           string                     `json:"status"`
	Mode             string                     `json:"mode"`
	StartedAt        time.Time                  `json:"startedAt"`
	FixtureSHA       string                     `json:"fixtureSHA"`
	Environment      Environment                `json:"environment"`
	OfferSDP         string                     `json:"offerSDP"`
	AnswerSDP        string                     `json:"answerSDP"`
	Mapping          opusredtest.Mapping        `json:"mapping"`
	SenderEvidence   opusredtest.Evidence       `json:"senderEvidence"`
	ReceiverEvidence opusredtest.Evidence       `json:"receiverEvidence"`
	Output           []opusredtest.PacketRecord `json:"output"`
	Assertions       []Assertion                `json:"assertions"`
	Audio            Audio                      `json:"audio"`
	Teardown         Teardown                   `json:"teardown"`
	ArtifactDir      string                     `json:"artifactDir"`
	Errors           []string                   `json:"errors,omitempty"`
}

// Lab owns one reusable fixture. Trials themselves never share PeerConnections.
type Lab struct {
	fixture     *opusredmedia.Fixture
	artifactDir string
	environment Environment
	mutex       sync.Mutex
}

// New generates the real Opus fixture and checks the codec prerequisite once.
func New(ctx context.Context, artifactDir string) (*Lab, error) {
	absolute, err := filepath.Abs(artifactDir)
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(absolute, 0o755); err != nil {
		return nil, err
	}
	fixture, err := opusredmedia.Generate(ctx)
	if err != nil {
		return nil, err
	}
	if len(fixture.Packets) != packetCount {
		return nil, fmt.Errorf("fixture has %d packets, expected %d", len(fixture.Packets), packetCount)
	}
	return &Lab{fixture: fixture, artifactDir: absolute, environment: provenance(ctx, fixture.FFmpegVersion)}, nil
}

// Run returns evidence even when the trial fails. Supported modes are plain and red.
func (lab *Lab) Run(ctx context.Context, mode string) (*Result, error) {
	result := &Result{ID: fmt.Sprintf("test01-%d", time.Now().UnixNano()), Status: "FAIL", Mode: mode, StartedAt: time.Now().UTC(), FixtureSHA: lab.fixture.SHA256, Environment: lab.environment}
	if !lab.mutex.TryLock() {
		result.Status = "BLOCKED"
		result.Errors = []string{"another trial is running"}
		return result, errors.New(result.Errors[0])
	}
	defer lab.mutex.Unlock()
	result.ArtifactDir = filepath.Join(lab.artifactDir, result.ID)
	if err := os.MkdirAll(result.ArtifactDir, 0o755); err != nil {
		result.Errors = append(result.Errors, err.Error())
		return result, err
	}
	trialCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	var runErr error
	var payloads [][]byte
	if mode != "plain" && mode != "red" {
		runErr = fmt.Errorf("unsupported mode %q", mode)
	} else if lab.environment.Error != "" {
		result.Status = "BLOCKED"
		runErr = errors.New(lab.environment.Error)
	} else {
		payloads, runErr = lab.runTrial(trialCtx, result)
	}
	if runErr == nil {
		result.Audio.Reference, runErr = opusredmedia.WriteCorpus(trialCtx, result.ArtifactDir, "reference", lab.fixture, lab.fixture.Packets)
	}
	if runErr == nil {
		result.Audio.Received, runErr = opusredmedia.WriteCorpus(trialCtx, result.ArtifactDir, "received", lab.fixture, payloads)
	}
	if runErr == nil {
		result.check("decoded_pcm", result.Audio.Reference.PCMSHA256 == result.Audio.Received.PCMSHA256 && result.Audio.Reference.PCMBytes == result.Audio.Received.PCMBytes && result.Audio.Reference.PCMBytes > 0, "reference and received PCM SHA256 and byte counts match")
		result.check("decoded_duration", result.Audio.Reference.DurationSeconds > 5.9 && result.Audio.Reference.DurationSeconds < 6.1, fmt.Sprintf("reference duration %.6fs", result.Audio.Reference.DurationSeconds))
		result.check("decoded_signal", result.Audio.Reference.RMS > 0 && result.Audio.Received.RMS > 0, fmt.Sprintf("reference RMS %g, received RMS %g", result.Audio.Reference.RMS, result.Audio.Received.RMS))
		result.Status = "PASS"
		for _, assertion := range result.Assertions {
			if !assertion.Passed {
				result.Status = "FAIL"
				break
			}
		}
		if result.Status == "FAIL" {
			runErr = errors.New("one or more qualification assertions failed")
		}
	}
	if runErr != nil {
		result.Errors = append(result.Errors, runErr.Error())
	}
	return result, runErr
}

func (result *Result) check(name string, passed bool, detail string) {
	result.Assertions = append(result.Assertions, Assertion{Name: name, Passed: passed, Detail: detail})
}

func newPeer(session *opusredtest.Session, redEnabled bool) (*webrtc.PeerConnection, error) {
	engine := &webrtc.MediaEngine{}
	if err := engine.RegisterCodec(webrtc.RTPCodecParameters{RTPCodecCapability: webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus, ClockRate: 48000, Channels: 2, SDPFmtpLine: "minptime=10;useinbandfec=0"}, PayloadType: 109}, webrtc.RTPCodecTypeAudio); err != nil {
		return nil, err
	}
	if redEnabled {
		if err := engine.RegisterCodec(webrtc.RTPCodecParameters{RTPCodecCapability: webrtc.RTPCodecCapability{MimeType: "audio/red", ClockRate: 48000, Channels: 2, SDPFmtpLine: "109/109/109"}, PayloadType: 127}, webrtc.RTPCodecTypeAudio); err != nil {
			return nil, err
		}
	}
	registry := &interceptor.Registry{}
	registry.Add(session.WireFactory())
	if err := webrtc.RegisterDefaultInterceptors(engine, registry); err != nil {
		return nil, err
	}
	registry.Add(session.SenderFactory())
	registry.Add(session.ReceiverFactory())
	if err := webrtc.ConfigureTWCCHeaderExtensionSender(engine, registry); err != nil {
		return nil, err
	}
	registry.Add(session.SourceFactory())
	settings := webrtc.SettingEngine{}
	settings.SetIncludeLoopbackCandidate(true)
	settings.SetIPFilter(func(ip net.IP) bool { return ip.IsLoopback() })
	settings.SetNetworkTypes([]webrtc.NetworkType{webrtc.NetworkTypeUDP4})
	settings.SetICETimeouts(5*time.Second, 5*time.Second, time.Second)
	return webrtc.NewAPI(webrtc.WithMediaEngine(engine), webrtc.WithInterceptorRegistry(registry), webrtc.WithSettingEngine(settings)).NewPeerConnection(webrtc.Configuration{})
}

func wait(ctx context.Context, done <-chan struct{}) error {
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (lab *Lab) runTrial(ctx context.Context, result *Result) (payloads [][]byte, trialErr error) {
	senderSession, err := opusredtest.NewSession(maximumRTPSize)
	if err != nil {
		return nil, err
	}
	receiverSession, err := opusredtest.NewSession(maximumRTPSize)
	if err != nil {
		return nil, err
	}
	sender, err := newPeer(senderSession, result.Mode == "red")
	if err != nil {
		return nil, err
	}
	receiver, err := newPeer(receiverSession, result.Mode == "red")
	if err != nil {
		_ = sender.Close()
		return nil, err
	}
	var readerMutex sync.Mutex
	readerStarted, readerDone := make(chan struct{}), make(chan struct{})
	var readerErr error
	var receivedPayloads [][]byte
	var receivedPackets []opusredtest.PacketRecord
	var onTrackCount int
	var codecOK bool
	var closing bool
	rtcpDone := make(chan struct{})
	rtcpStarted := false
	defer func() {
		readerMutex.Lock()
		closing = true
		readerMutex.Unlock()
		closeErr := errors.Join(sender.Close(), receiver.Close())
		joinCtx, joinCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer joinCancel()
		select {
		case <-readerStarted:
			closeErr = errors.Join(closeErr, wait(joinCtx, readerDone))
		default:
		}
		if rtcpStarted {
			closeErr = errors.Join(closeErr, wait(joinCtx, rtcpDone))
		}
		readerMutex.Lock()
		output := append([]opusredtest.PacketRecord(nil), receivedPackets...)
		payloads = append([][]byte(nil), receivedPayloads...)
		failure := readerErr
		tracks, codec := onTrackCount, codecOK
		readerMutex.Unlock()
		result.check("on_track", tracks == 1 && codec, fmt.Sprintf("OnTrack count %d; decoded track reports Opus: %t", tracks, codec))
		result.Output = output
		result.SenderEvidence, result.ReceiverEvidence = senderSession.Snapshot(), receiverSession.Snapshot()
		result.Teardown.Success = closeErr == nil
		if closeErr != nil {
			result.Teardown.Error = closeErr.Error()
		}
		result.check("teardown", closeErr == nil, "both PeerConnections closed and all started application readers joined")
		if trialErr == nil && failure != nil {
			trialErr = failure
		}
		if trialErr == nil && closeErr != nil {
			trialErr = closeErr
		}
		lab.checkPackets(result)
	}()
	receiver.OnTrack(func(track *webrtc.TrackRemote, _ *webrtc.RTPReceiver) {
		readerMutex.Lock()
		if closing {
			readerMutex.Unlock()
			return
		}
		onTrackCount++
		if onTrackCount != 1 {
			readerMutex.Unlock()
			return
		}
		codecOK = strings.EqualFold(track.Codec().MimeType, webrtc.MimeTypeOpus)
		close(readerStarted)
		readerMutex.Unlock()
		defer close(readerDone)
		if deadline, ok := ctx.Deadline(); ok {
			_ = track.SetReadDeadline(deadline)
		}
		for {
			packet, _, readErr := track.ReadRTP()
			if readErr != nil {
				readerMutex.Lock()
				if len(receivedPackets) < packetCount {
					readerErr = readErr
				}
				readerMutex.Unlock()
				return
			}
			readerMutex.Lock()
			receivedPackets = append(receivedPackets, opusredtest.NewPacketRecord(&packet.Header, packet.Payload))
			receivedPayloads = append(receivedPayloads, append([]byte(nil), packet.Payload...))
			count := len(receivedPackets)
			readerMutex.Unlock()
			if count == packetCount {
				_ = track.SetReadDeadline(time.Now().Add(250 * time.Millisecond))
			}
		}
	})
	senderConnected, receiverConnected := make(chan struct{}), make(chan struct{})
	var senderOnce, receiverOnce sync.Once
	sender.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		if state == webrtc.PeerConnectionStateConnected {
			senderOnce.Do(func() { close(senderConnected) })
		}
	})
	receiver.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		if state == webrtc.PeerConnectionStateConnected {
			receiverOnce.Do(func() { close(receiverConnected) })
		}
	})
	track, err := webrtc.NewTrackLocalStaticRTP(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus, ClockRate: 48000, Channels: 2}, "test01-audio", result.ID)
	if err != nil {
		return nil, err
	}
	rtpSender, err := sender.AddTrack(track)
	if err != nil {
		return nil, err
	}
	rtcpStarted = true
	go func() {
		defer close(rtcpDone)
		buffer := make([]byte, 1500)
		for {
			if _, _, readErr := rtpSender.Read(buffer); readErr != nil {
				return
			}
		}
	}()
	offer, err := sender.CreateOffer(nil)
	if err != nil {
		return nil, err
	}
	offerGathered := webrtc.GatheringCompletePromise(sender)
	if err = sender.SetLocalDescription(offer); err != nil {
		return nil, err
	}
	if err = wait(ctx, offerGathered); err != nil {
		return nil, err
	}
	result.OfferSDP = sender.LocalDescription().SDP
	if err = receiver.SetRemoteDescription(*sender.LocalDescription()); err != nil {
		return nil, err
	}
	answer, err := receiver.CreateAnswer(nil)
	if err != nil {
		return nil, err
	}
	mapping, err := opusredtest.ValidateAnswer(result.OfferSDP, answer.SDP)
	if err != nil {
		return nil, err
	}
	result.Mapping = mapping
	if err = senderSession.Arm(mapping); err != nil {
		return nil, err
	}
	if err = receiverSession.Arm(mapping); err != nil {
		return nil, err
	}
	answerGathered := webrtc.GatheringCompletePromise(receiver)
	if err = receiver.SetLocalDescription(answer); err != nil {
		return nil, err
	}
	if err = wait(ctx, answerGathered); err != nil {
		return nil, err
	}
	result.AnswerSDP = receiver.LocalDescription().SDP
	if _, err = opusredtest.ValidateAnswer(result.OfferSDP, result.AnswerSDP); err != nil {
		return nil, err
	}
	if err = sender.SetRemoteDescription(*receiver.LocalDescription()); err != nil {
		return nil, err
	}
	if err = wait(ctx, senderConnected); err != nil {
		return nil, err
	}
	if err = wait(ctx, receiverConnected); err != nil {
		return nil, err
	}
	result.check("sdp_mapping", mapping.OpusPT == 109 && mapping.HasRED == (result.Mode == "red") && (!mapping.HasRED || mapping.REDPT == 127), fmt.Sprintf("accepted Opus PT %d, RED available %t, RED PT %d", mapping.OpusPT, mapping.HasRED, mapping.REDPT))
	ticker := time.NewTicker(packetInterval)
	defer ticker.Stop()
	for index, payload := range lab.fixture.Packets {
		if index != 0 {
			select {
			case <-ticker.C:
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		packet := &rtp.Packet{Header: rtp.Header{Version: 2, SequenceNumber: uint16(1000 + index), Timestamp: uint32(96000 + 960*index), Marker: index == 0}, Payload: append([]byte(nil), payload...)}
		if err = track.WriteRTP(packet); err != nil {
			return nil, err
		}
	}
	if err = wait(ctx, readerStarted); err != nil {
		return nil, err
	}
	if err = wait(ctx, readerDone); err != nil {
		return nil, err
	}
	return nil, nil
}

func (lab *Lab) checkPackets(result *Result) {
	originals, sent, received := result.SenderEvidence.Original, result.SenderEvidence.Sent, result.ReceiverEvidence.Received
	result.check("packet_counts", len(originals) == packetCount && len(sent) == packetCount && len(received) == packetCount && len(result.Output) == packetCount, fmt.Sprintf("source %d, sent %d, received %d, delivered %d, expected %d each", len(originals), len(sent), len(received), len(result.Output), packetCount))
	result.check("adapter_faults", len(result.SenderEvidence.Faults) == 0 && len(result.ReceiverEvidence.Faults) == 0, fmt.Sprintf("sender faults %v, receiver faults %v", result.SenderEvidence.Faults, result.ReceiverEvidence.Faults))
	identity, wirePrimary, depths, sizeLimit, sourceFixture := true, true, true, true, len(originals) == packetCount
	for index, original := range originals {
		if index >= len(lab.fixture.Packets) {
			sourceFixture = false
			continue
		}
		expected := opusredtest.NewPacketRecord(&rtp.Header{Version: 2, PayloadType: result.Mapping.OpusPT, SSRC: original.SSRC, SequenceNumber: uint16(1000 + index), Timestamp: uint32(96000 + 960*index)}, lab.fixture.Packets[index])
		if !sameMedia(original, expected) {
			sourceFixture = false
		}
	}
	result.check("original_source_fixture", sourceFixture, "the source catalog matches the independently generated fixture and scheduled RTP identities")
	if len(originals) != packetCount || len(sent) != packetCount || len(received) != packetCount || len(result.Output) != packetCount {
		identity, wirePrimary, depths = false, false, false
	}
	for index, wire := range sent {
		if wire.RTPSize > maximumRTPSize {
			sizeLimit = false
		}
		expectedDepth := 0
		if result.Mode == "red" && index > 0 {
			expectedDepth = 1
			if index > 1 {
				expectedDepth = 2
			}
		}
		expectedPT := result.Mapping.OpusPT
		if expectedDepth > 0 {
			expectedPT = result.Mapping.REDPT
		}
		if wire.PayloadType != expectedPT || len(wire.Blocks) != expectedDepth {
			depths = false
		}
		if index < len(originals) && !sameMedia(originals[index], wire.Primary) {
			wirePrimary = false
		}
		for blockIndex, block := range wire.Blocks {
			sourceIndex := index - len(wire.Blocks) + blockIndex
			if sourceIndex < 0 || sourceIndex >= len(originals) {
				depths = false
				continue
			}
			original := originals[sourceIndex]
			if block.PayloadType != result.Mapping.OpusPT || block.PayloadSHA256 != original.PayloadSHA256 || block.PayloadLength != original.PayloadLength || uint32(block.TimestampOffset) != wire.Timestamp-original.Timestamp {
				depths = false
			}
		}
	}
	for index, output := range result.Output {
		if index >= len(originals) || !sameMedia(originals[index], output) || output.Sequence != uint16(1000+index) || output.Timestamp != uint32(96000+960*index) || output.PayloadType != result.Mapping.OpusPT {
			identity = false
		}
	}
	noRecovery := len(received) == packetCount && len(result.Output) == packetCount
	for index, output := range result.Output {
		if index >= len(received) || !sameMedia(output, received[index].Primary) {
			noRecovery = false
		}
	}
	result.check("no_recovery", noRecovery, "every delivered packet matches a physically received primary; no missing originals or synthesized recovery")
	for index, wire := range received {
		if index >= len(sent) || wire.PacketRecord != sent[index].PacketRecord {
			wirePrimary = false
		}
	}
	result.check("delivered_identity", identity, "all 300 source payload hashes, lengths, SSRCs, PTs, sequences and timestamps preserved in order without extras")
	result.check("wire_primary_identity", wirePrimary, "independent parser primary matches source; received wire packet identity matches sent wire")
	result.check("redundancy_depth", depths, "plain stays plain; RED starts plain, then one and two exact previous payloads")
	result.check("rtp_size_limit", sizeLimit, fmt.Sprintf("every sent RTP packet is at most %d bytes including header/extensions/padding", maximumRTPSize))
}

func sameMedia(a, b opusredtest.PacketRecord) bool {
	return a.Sequence == b.Sequence && a.Timestamp == b.Timestamp && a.SSRC == b.SSRC && a.PayloadType == b.PayloadType && a.PayloadLength == b.PayloadLength && a.PayloadSHA256 == b.PayloadSHA256
}

func provenance(ctx context.Context, ffmpegVersion string) Environment {
	environment := Environment{GoVersion: runtime.Version(), ExpectedInterceptorSHA: expectedInterceptorSHA, FFmpegVersion: ffmpegVersion, Modules: map[string]string{}}
	info, buildInfoAvailable := debug.ReadBuildInfo()
	var binaryErr error
	if !buildInfoAvailable {
		info = nil
	}
	environment.BinaryInterceptorPath, environment.BinaryInterceptorVersion, environment.BinaryDependencyStatus, binaryErr = validateBinaryInterceptor(info)
	if info != nil {
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" {
				environment.BuildVCSRevision = setting.Value
			}
			if setting.Key == "vcs.modified" {
				environment.BuildVCSModified = setting.Value
			}
		}
		for _, module := range info.Deps {
			value := module.Version
			if module.Replace != nil {
				value = module.Replace.Path + "@" + module.Replace.Version
			}
			environment.Modules[module.Path] = value
		}
	}
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		environment.Error = "cannot locate qualification checkout for provenance"
		return environment
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(source), "../.."))
	git := exec.CommandContext(ctx, "git", "-C", repoRoot, "rev-parse", "HEAD")
	raw, err := git.Output()
	if err != nil {
		environment.Error = fmt.Sprintf("WebRTC git provenance: %v", err)
		return environment
	}
	environment.CheckoutSHA = strings.TrimSpace(string(raw))
	command := exec.CommandContext(ctx, "go", "list", "-modfile=go.red.mod", "-m", "-json", "github.com/pion/interceptor")
	command.Dir = repoRoot
	raw, err = command.Output()
	if err != nil {
		environment.Error = fmt.Sprintf("interceptor module provenance: %v", err)
		return environment
	}
	var module struct {
		Path    string
		Version string
		Replace *struct {
			Path    string
			Version string
			GoMod   string
			Origin  *struct{ Hash string }
		}
	}
	if err = json.Unmarshal(raw, &module); err != nil {
		environment.Error = err.Error()
		return environment
	}
	if module.Replace == nil {
		environment.Error = "qualification requires the pinned interceptor replacement"
		return environment
	}
	environment.InterceptorPath, environment.InterceptorVersion = module.Replace.Path, module.Replace.Version
	if module.Replace.Origin != nil {
		environment.InterceptorOriginSHA = module.Replace.Origin.Hash
	}
	if environment.InterceptorOriginSHA == "" && module.Replace.GoMod != "" {
		infoPath := strings.TrimSuffix(module.Replace.GoMod, ".mod") + ".info"
		if infoRaw, readErr := os.ReadFile(infoPath); readErr == nil {
			var info struct{ Origin *struct{ Hash string } }
			if json.Unmarshal(infoRaw, &info) == nil && info.Origin != nil {
				environment.InterceptorOriginSHA = info.Origin.Hash
			}
		}
	}
	if module.Replace.Path != "github.com/gokuljs/interceptor" || module.Replace.Version != expectedInterceptorVersion || (environment.InterceptorOriginSHA != "" && environment.InterceptorOriginSHA != expectedInterceptorSHA) {
		environment.Error = "interceptor module does not match the qualification source pin"
	}
	if binaryErr != nil {
		environment.Error = errors.Join(binaryErr, errorOrNil(environment.Error)).Error()
	}
	return environment
}

func errorOrNil(message string) error {
	if message == "" {
		return nil
	}
	return errors.New(message)
}

// validateBinaryInterceptor checks compiled dependency metadata, independently
// from the checkout's module files. Go test binaries may omit dependencies.
func validateBinaryInterceptor(info *debug.BuildInfo) (path, version, status string, err error) {
	if info != nil {
		for _, module := range info.Deps {
			if module.Path != "github.com/pion/interceptor" {
				continue
			}
			path, version = module.Path, module.Version
			if module.Replace != nil {
				path, version = module.Replace.Path, module.Replace.Version
			}
			if path != "github.com/gokuljs/interceptor" || version != expectedInterceptorVersion {
				return path, version, "mismatch", fmt.Errorf("compiled interceptor dependency does not match the qualification pin: %s@%s", path, version)
			}
			return path, version, "verified", nil
		}
	}
	return "", "", "unavailable", errors.New("compiled interceptor dependency metadata is unavailable; qualification requires a standalone built lab binary")
}
