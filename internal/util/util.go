// Package util provides auxiliary functions internally used in webrtc package
package util

import (
	"math/rand"
	"strings"
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

// FlattenErrs flattens multiple errors into one
func FlattenErrs(errs []error) error {
	errs2 := []error{}
	for _, e := range errs {
		if e != nil {
			errs2 = append(errs2, e)
		}
	}
	if len(errs2) == 0 {
		return nil
	}
	return multiError(errs2)
}

type multiError []error

func (me multiError) Error() string {
	var errstrings []string

	for _, err := range me {
		if err != nil {
			errstrings = append(errstrings, err.Error())
		}
	}

	if len(errstrings) == 0 {
		return "multiError must contain multiple error but is empty"
	}

	return strings.Join(errstrings, "\n")
}

func (me multiError) Is(err error) bool {
	for _, e := range me {
		if e == err {
			return true
		}
		if me2, ok := e.(multiError); ok {
			if me2.Is(err) {
				return true
			}
		}
	}
	return false
}
