// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package rtpdump

import (
	"fmt"
	"io"
	"sync"
)

// Writer writes the RTPDump file format
type Writer struct {
	writerMu sync.Mutex
	writer   io.Writer
}

// NewWriter makes a new Writer and immediately writes the given Header
// to begin the file.
func NewWriter(w io.Writer, hdr Header) (*Writer, error) {
	preamble := fmt.Sprintf(
		"#!rtpplay1.0 %s/%d\n",
		hdr.Source.To4().String(),
		hdr.Port)
	if _, err := w.Write([]byte(preamble)); err != nil {
		return nil, err
	}

	hData, err := hdr.Marshal()
	if err != nil {
		return nil, err
	}
	if _, err := w.Write(hData); err != nil {
		return nil, err
	}

	return &Writer{writer: w}, nil
}

// WritePacket writes a Packet to the output
func (w *Writer) WritePacket(p Packet) error {
	w.writerMu.Lock()
	defer w.writerMu.Unlock()

	data, err := p.Marshal()
	if err != nil {
		return err
	}
	if _, err := w.writer.Write(data); err != nil {
		return err
	}

	return nil
}
