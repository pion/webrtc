// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package signal

import "github.com/pion/randutil"

// RandSeq generates a random string to serve as dummy data
//
// It returns a deterministic sequence of values each time a program is run.
// Use rand.Seed() function in your real applications.
func RandSeq(n int) string {
	val, err := randutil.GenerateCryptoRandomString(n, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	if err != nil {
		panic(err)
	}

	return val
}
