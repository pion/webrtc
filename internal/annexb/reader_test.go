// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package annexb

import (
	"bytes"
	"io"
	"testing"
	"testing/iotest"

	"github.com/stretchr/testify/require"
)

func TestReaderDataAndEOF(t *testing.T) {
	data := []byte{0, 0, 0, 1, 0x65, 2, 0, 0, 1, 0x41, 3}
	r := New(iotest.DataErrReader(bytes.NewReader(data)), io.ErrUnexpectedEOF)
	for _, want := range [][]byte{{0x65, 2}, {0x41, 3}} {
		got, err := r.Next()
		require.NoError(t, err)
		require.Equal(t, want, got)
	}
	_, err := r.Next()
	require.ErrorIs(t, err, io.EOF)
}

func TestReaderPreservesReadError(t *testing.T) {
	boom := io.ErrClosedPipe
	r := New(io.MultiReader(bytes.NewReader([]byte{0, 0, 0, 1, 0x65}), iotest.ErrReader(boom)), io.ErrUnexpectedEOF)
	_, err := r.Next()
	require.ErrorIs(t, err, boom)
}
