// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

// Package util provides auxiliary functions internally used in webrtc package
package util //nolint: revive

import "github.com/pion/randutil"

const (
	runesAlpha = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
)

// Use global random generator to properly seed by crypto grade random.
var globalMathRandomGenerator = randutil.NewMathRandomGenerator() // nolint:gochecknoglobals

// MathRandAlpha generates a mathematical random alphabet sequence of the requested length.
func MathRandAlpha(n int) string {
	return globalMathRandomGenerator.GenerateString(n, runesAlpha)
}

// RandUint32 generates a mathematical random uint32.
func RandUint32() uint32 {
	return globalMathRandomGenerator.Uint32()
}
