package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"time"
)

type IVFWriter struct {
	fd    *os.File
	time  time.Time
	count uint64
}

func panicWrite(fd *os.File, data []byte) {
	if _, err := fd.Write(data); err != nil {
		panic(err)
	}
}

func NewIVFWriter(fileName string) (*IVFWriter, error) {
	f, err := os.Create(fileName)
	if err != nil {
		return nil, err
	}

	panicWrite(f, []byte("DKIF"))       // DKIF
	panicWrite(f, []byte{0, 0})         // Version
	panicWrite(f, []byte{32, 0})        // Header Size
	panicWrite(f, []byte("VP80"))       // FOURCC
	panicWrite(f, []byte{128, 2})       // Width  (640)
	panicWrite(f, []byte{224, 1})       // Height (480)
	panicWrite(f, []byte{232, 3, 0, 0}) // Framerate numerator
	panicWrite(f, []byte{1, 0, 0, 0})   // Framerate denominator
	panicWrite(f, []byte{132, 3, 0, 0}) // Frame count
	panicWrite(f, []byte{0, 0, 0, 0})   // Unused

	i := &IVFWriter{fd: f}
	return i, nil
}

func (i *IVFWriter) AddBuffer(buffer []byte) {
	if len(buffer) == 0 {
		fmt.Println("skipping")
		return
	}
	bufferLen := make([]byte, 4)
	fmt.Println(len(buffer))
	binary.LittleEndian.PutUint32(bufferLen, uint32(len(buffer)))

	pts := make([]byte, 8)
	binary.LittleEndian.PutUint64(pts, i.count)
	i.count += 33

	panicWrite(i.fd, bufferLen)
	panicWrite(i.fd, pts)
	panicWrite(i.fd, buffer)
}

func (i *IVFWriter) Close() error {
	return i.fd.Close()
}
