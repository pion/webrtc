package util

import (
	"math/rand"
	"time"
)

// RandSeq generates a random alpha numeric sequence of the requested length
func RandSeq(n int) string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	letters := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[r.Intn(len(letters))]
	}
	return string(b)
}

// GetPadding Returns the padding required to make the length a multiple of 4
func GetPadding(len int) int {
	if len%4 == 0 {
		return 0
	}
	return 4 - (len % 4)
}
