package signal

import (
	"math/rand"
)

// RandSeq generates a random string to serve as dummy data
//
// It returns a deterministic sequence of values each time a program is run.
// Use rand.Seed() function in your real applications.
func RandSeq(n int) string {
	letters := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}
