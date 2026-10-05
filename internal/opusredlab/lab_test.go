// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build opusred && !js

package opusredlab

import (
	"fmt"
	"runtime/debug"
	"testing"

	"github.com/pion/rtp"
	"github.com/pion/webrtc/v4/internal/opusredmedia"
	"github.com/pion/webrtc/v4/internal/opusredtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func oracleFixture(mode string) (*Lab, *Result) {
	fixture := &opusredmedia.Fixture{Packets: make([][]byte, packetCount)}
	result := &Result{Mode: mode, Mapping: opusredtest.Mapping{OpusPT: 109, REDPT: 127, ClockRate: 48000, Channels: 2, HasRED: mode == "red"}}
	for index := range fixture.Packets {
		fixture.Packets[index] = []byte{byte(index), byte(index >> 8), 0xff}
		header := rtp.Header{Version: 2, PayloadType: 109, SSRC: 555, SequenceNumber: uint16(1000 + index), Timestamp: uint32(96000 + 960*index)}
		record := opusredtest.NewPacketRecord(&header, fixture.Packets[index])
		result.SenderEvidence.Original = append(result.SenderEvidence.Original, record)
		wire := opusredtest.WireRecord{PacketRecord: record, Primary: record}
		if mode == "red" && index > 0 {
			wire.PayloadType = 127
			wire.PayloadSHA256 = fmt.Sprintf("outer-%d", index)
			depth := min(index, 2)
			for distance := depth; distance > 0; distance-- {
				prior := result.SenderEvidence.Original[index-distance]
				wire.Blocks = append(wire.Blocks, opusredtest.BlockRecord{PayloadType: 109, TimestampOffset: uint16(distance * 960), PayloadLength: prior.PayloadLength, PayloadSHA256: prior.PayloadSHA256})
			}
			wire.RTPSize += 1 + depth*7
		}
		result.SenderEvidence.Sent = append(result.SenderEvidence.Sent, wire)
		received := wire
		received.Blocks = append([]opusredtest.BlockRecord(nil), wire.Blocks...)
		result.ReceiverEvidence.Received = append(result.ReceiverEvidence.Received, received)
		result.Output = append(result.Output, record)
	}
	return &Lab{fixture: fixture}, result
}

func TestPacketOracleAcceptsCompleteNoLossEvidence(t *testing.T) {
	for _, mode := range []string{"plain", "red"} {
		t.Run(mode, func(t *testing.T) {
			lab, result := oracleFixture(mode)
			lab.checkPackets(result)
			for _, assertion := range result.Assertions {
				assert.True(t, assertion.Passed, "%s: %s", assertion.Name, assertion.Detail)
			}
		})
	}
}

func TestPacketOracleRejectsIncorrectEvidence(t *testing.T) {
	cases := []struct {
		name, assertion string
		corrupt         func(*Result)
	}{
		{"wrong_redundant_copy", "redundancy_depth", func(result *Result) { result.SenderEvidence.Sent[2].Blocks[0].PayloadSHA256 = "incorrect" }},
		{"plain_bypass_in_red_mode", "redundancy_depth", func(result *Result) {
			for index, original := range result.SenderEvidence.Original {
				plain := opusredtest.WireRecord{PacketRecord: original, Primary: original}
				result.SenderEvidence.Sent[index], result.ReceiverEvidence.Received[index] = plain, plain
			}
		}},
		{"missing_output", "packet_counts", func(result *Result) { result.Output = result.Output[:packetCount-1] }},
		{"duplicate_replaces_original", "delivered_identity", func(result *Result) { result.Output[42] = result.Output[41] }},
		{"unobserved_primary", "no_recovery", func(result *Result) {
			result.ReceiverEvidence.Received[42].Primary.PayloadSHA256 = "not-the-delivered-packet"
		}},
		{"capture_differs_from_fixture", "original_source_fixture", func(result *Result) { result.SenderEvidence.Original[42].PayloadSHA256 = "not-the-generated-packet" }},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			lab, result := oracleFixture("red")
			testCase.corrupt(result)
			lab.checkPackets(result)
			for _, assertion := range result.Assertions {
				if assertion.Name == testCase.assertion {
					assert.False(t, assertion.Passed, "incorrect evidence passed %s", assertion.Name)
					return
				}
			}
			require.FailNow(t, "expected assertion was not evaluated", "%s", testCase.assertion)
		})
	}
}

func TestBinaryProvenance(t *testing.T) {
	cases := []struct {
		name     string
		info     *debug.BuildInfo
		status   string
		accepted bool
	}{
		{name: "missing_metadata", status: "unavailable"},
		{name: "test_binary_without_dependencies", info: &debug.BuildInfo{}, status: "unavailable"},
		{name: "released_interceptor", info: &debug.BuildInfo{Deps: []*debug.Module{{Path: "github.com/pion/interceptor", Version: "v0.1.49"}}}, status: "mismatch"},
		{name: "different_pr_commit", info: &debug.BuildInfo{Deps: []*debug.Module{{Path: "github.com/pion/interceptor", Replace: &debug.Module{Path: "github.com/gokuljs/interceptor", Version: "v0.0.0-wrong"}}}}, status: "mismatch"},
		{name: "qualified_binary", info: &debug.BuildInfo{Deps: []*debug.Module{{Path: "github.com/pion/interceptor", Replace: &debug.Module{Path: "github.com/gokuljs/interceptor", Version: expectedInterceptorVersion}}}}, status: "verified", accepted: true},
	}
	for _, candidate := range cases {
		t.Run(candidate.name, func(t *testing.T) {
			_, _, status, err := validateBinaryInterceptor(candidate.info)
			assert.Equal(t, candidate.status, status)
			assert.Equal(t, candidate.accepted, err == nil)
		})
	}
}
