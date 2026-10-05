// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build opusred && !js

package main

import (
	"context"
	"errors"
)

func newTrialRunner(_ context.Context, _ string) (trialRunner, error) {
	return nil, errors.New("Test 01 is not implemented yet")
}
