package test

import (
	"bytes"
	"fmt"
	"io"
)

// Options represents the configuration of the stress test
type Options struct {
	MsgSize  int
	MsgCount int
}

// Stress enables stress testing of a transport.Conn
func Stress(ca io.Writer, cb io.Reader, opt Options) error {
	bufs := make(chan []byte, opt.MsgCount)
	errCh := make(chan error)
	// Write
	go func() {
		err := write(ca, bufs, opt)
		close(bufs)
		errCh <- err
	}()

	// Read
	result := make([]byte, opt.MsgSize)

	for original := range bufs {
		n, err := cb.Read(result)
		if err != nil {
			return err
		}
		if !bytes.Equal(original, result) {
			return fmt.Errorf("byte sequence changed %d", n)
		}
	}

	return <-errCh
}

func write(c io.Writer, bufs chan []byte, opt Options) error {
	for i := 0; i < opt.MsgCount; i++ {
		buf, err := randBuf(opt.MsgSize)
		if err != nil {
			return err
		}
		bufs <- buf
		if _, err = c.Write(buf); err != nil {
			return err
		}
	}
	return nil
}
