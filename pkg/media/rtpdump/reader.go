package rtpdump

import (
	"bufio"
	"io"
	"regexp"
	"sync"
)

// The file starts with #!rtpplay1.0 address/port\n
var preambleRegexp = regexp.MustCompile(`#\!rtpplay1\.0 \d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\/\d{1,5}\n`)

// Reader reads the RTPDump file format
type Reader struct {
	readerMu sync.Mutex
	reader   io.Reader
}

// NewReader opens a new Reader and immediately reads the Header from the start
// of the input stream.
func NewReader(r io.Reader) (*Reader, Header, error) {
	var hdr Header

	bio := bufio.NewReader(r)

	// Look ahead to see if there's a valid preamble
	peek, err := bio.Peek(preambleLen)
	if err == io.EOF {
		return nil, hdr, errMalformed
	}
	if err != nil {
		return nil, hdr, err
	}
	if !preambleRegexp.Match(peek) {
		return nil, hdr, errMalformed
	}

	// consume the preamble
	_, _, err = bio.ReadLine()
	if err == io.EOF {
		return nil, hdr, errMalformed
	}
	if err != nil {
		return nil, hdr, err
	}

	hBuf := make([]byte, headerLen)
	_, err = io.ReadFull(bio, hBuf)
	if err == io.ErrUnexpectedEOF || err == io.EOF {
		return nil, hdr, errMalformed
	}
	if err != nil {
		return nil, hdr, err
	}

	if err := hdr.Unmarshal(hBuf); err != nil {
		return nil, hdr, err
	}

	return &Reader{
		reader: bio,
	}, hdr, nil
}

// Next returns the next Packet in the Reader input stream
func (r *Reader) Next() (Packet, error) {
	r.readerMu.Lock()
	defer r.readerMu.Unlock()

	hBuf := make([]byte, pktHeaderLen)

	_, err := io.ReadFull(r.reader, hBuf)
	if err == io.ErrUnexpectedEOF {
		return Packet{}, errMalformed
	}
	if err != nil {
		return Packet{}, err
	}

	var h packetHeader
	if err = h.Unmarshal(hBuf); err != nil {
		return Packet{}, err
	}

	if h.Length == 0 {
		return Packet{}, errMalformed
	}

	payload := make([]byte, h.Length-pktHeaderLen)
	_, err = io.ReadFull(r.reader, payload)
	if err == io.ErrUnexpectedEOF {
		return Packet{}, errMalformed
	}
	if err != nil {
		return Packet{}, err
	}

	return Packet{
		Offset:  h.Offset,
		IsRTCP:  h.PacketLength == 0,
		Payload: payload,
	}, nil
}
