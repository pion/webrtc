package util

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"strings"
	"time"
)

// Allows compressing offer/answer to bypass terminal input limits.
const compress = false

// Check is used to panic in an error occurs.
// Don't do this! We're only using it to make the examples shorter.
func Check(err error) {
	if err != nil {
		panic(err)
	}
}

// MustReadStdin blocks until input is received from stdin
func MustReadStdin() string {
	r := bufio.NewReader(os.Stdin)

	var in string
	for {
		var err error
		in, err = r.ReadString('\n')
		if err != io.EOF {
			Check(err)
		}
		in = strings.TrimSpace(in)
		if len(in) > 0 {
			break
		}
	}

	fmt.Println("")

	return in
}

// Encode encodes the input in base64
// It can optionally zip the input before encoding
func Encode(in string) string {
	if compress {
		in = zip(in)
	}

	return base64.StdEncoding.EncodeToString([]byte(in))
}

// Decode decodes the input from base64
// It can optionally unzip the input after decoding
func Decode(in string) string {
	b, err := base64.StdEncoding.DecodeString(in)
	Check(err)

	out := string(b)

	if compress {
		out = unzip(out)
	}

	return out
}

// RandSeq generates a random string to serve as dummy data
func RandSeq(n int) string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	letters := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[r.Intn(len(letters))]
	}
	return string(b)
}

func zip(in string) string {
	var b bytes.Buffer
	gz := gzip.NewWriter(&b)
	_, err := gz.Write([]byte(in))
	Check(err)
	err = gz.Flush()
	Check(err)
	err = gz.Close()
	Check(err)
	return string(b.Bytes())
}

func unzip(in string) string {
	var b bytes.Buffer
	_, err := b.Write([]byte(in))
	Check(err)
	r, err := gzip.NewReader(&b)
	Check(err)
	res, err := ioutil.ReadAll(r)
	Check(err)
	return string(res)
}
