// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

// Package annexb reads start-code-delimited NAL units.
package annexb

import (
	"bufio"
	"errors"
	"io"
)

// Reader splits an Annex-B stream without interpreting codec headers.
type Reader struct {
	stream        *bufio.Reader
	invalidPrefix error
	started       bool
}

// New creates a reader that reports invalidPrefix for a missing start code.
func New(stream io.Reader, invalidPrefix error) *Reader {
	return &Reader{stream: bufio.NewReader(stream), invalidPrefix: invalidPrefix}
}

// Next returns the next NAL unit, or io.EOF after the final unit.
func (r *Reader) Next() ([]byte, error) { //nolint:cyclop // Keep delimiter and end-of-stream handling together.
	if !r.started {
		if err := r.readPrefix(); err != nil {
			return nil, err
		}
		r.started = true
	}
	var data []byte
	zeros := 0
	for {
		readByte, err := r.stream.ReadByte()
		if err != nil {
			if errors.Is(err, io.EOF) && len(data) > 0 {
				return data, nil
			}

			return nil, err
		}
		if readByte == 1 && zeros >= 2 {
			data = data[:len(data)-min(zeros, 3)]
			zeros = 0
			if len(data) > 0 {
				return data, nil
			}

			continue
		}
		data = append(data, readByte)
		if readByte == 0 {
			zeros++
		} else {
			zeros = 0
		}
	}
}

func (r *Reader) readPrefix() error {
	prefix, err := r.stream.Peek(4)
	if err != nil {
		return err
	}
	var size int
	switch {
	case prefix[0] == 0 && prefix[1] == 0 && prefix[2] == 1:
		size = 3
	case prefix[0] == 0 && prefix[1] == 0 && prefix[2] == 0 && prefix[3] == 1:
		size = 4
	default:
		return r.invalidPrefix
	}
	_, err = r.stream.Discard(size)

	return err
}
