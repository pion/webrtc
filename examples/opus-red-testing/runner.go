// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build opusred && !js

package main

import (
	"context"
	"encoding/json"

	"github.com/pion/webrtc/v4/internal/opusredlab"
)

func newTrialRunner(ctx context.Context, artifactDir string) (trialRunner, error) {
	lab, err := opusredlab.New(ctx, artifactDir)
	if err != nil {
		return nil, err
	}
	return func(ctx context.Context, mode string) (outcome, error) {
		result, runErr := lab.Run(ctx, mode)
		if result == nil {
			return outcome{}, runErr
		}
		evidence, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return outcome{}, err
		}
		return outcome{
			evidence:       evidence,
			fileName:       result.ID + "-" + mode + ".json",
			referenceAudio: result.Audio.Reference.WAVPath,
			receivedAudio:  result.Audio.Received.WAVPath,
		}, runErr
	}, nil
}
