// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js

package webrtc

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pion/datachannel"
	"github.com/pion/logging"
	"github.com/pion/sctp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	sctpCommonHeaderLen = 12
	sctpChunkTypeData   = 0
	sctpChunkTypeIData  = 64
	sctpDataFlagBegin   = 0x02
)

var castagnoliTable = crc32.MakeTable(crc32.Castagnoli) //nolint:gochecknoglobals

// fragmentDroppingConn forwards SCTP packets but strips every DATA/I-DATA chunk of
// dropSID that is not the first fragment of a message. The receiver therefore
// admits the stream but never gets a complete, readable message on it.
type fragmentDroppingConn struct {
	net.Conn
	dropSID uint16

	firstFragments atomic.Int32
	dropped        atomic.Int32
}

//nolint:cyclop
func (c *fragmentDroppingConn) Write(pkt []byte) (int, error) {
	if len(pkt) < sctpCommonHeaderLen {
		return c.Conn.Write(pkt)
	}

	out := append([]byte{}, pkt[:sctpCommonHeaderLen]...)
	kept, stripped := 0, false
	for offset := sctpCommonHeaderLen; offset+4 <= len(pkt); {
		chunkLen := int(binary.BigEndian.Uint16(pkt[offset+2:]))
		if chunkLen < 4 || offset+chunkLen > len(pkt) {
			return c.Conn.Write(pkt)
		}
		next := min(offset+(chunkLen+3)&^3, len(pkt))
		chunk := pkt[offset:next]
		offset = next

		// DATA and I-DATA both carry the stream identifier at offset 8.
		isData := chunk[0] == sctpChunkTypeData || chunk[0] == sctpChunkTypeIData
		if isData && chunkLen >= 16 && binary.BigEndian.Uint16(chunk[8:]) == c.dropSID {
			if chunk[1]&sctpDataFlagBegin == 0 {
				c.dropped.Add(1)
				stripped = true

				continue
			}
			c.firstFragments.Add(1)
		}
		out = append(out, chunk...)
		kept++
	}

	if !stripped {
		return c.Conn.Write(pkt)
	}
	if kept == 0 {
		return len(pkt), nil
	}

	binary.LittleEndian.PutUint32(out[8:], 0)
	binary.LittleEndian.PutUint32(out[8:], crc32.Checksum(out, castagnoliTable))
	if _, err := c.Conn.Write(out); err != nil {
		return 0, err
	}

	return len(pkt), nil
}

type acceptTestPeer struct {
	client    *sctp.Association
	transport *SCTPTransport
	filter    *fragmentDroppingConn
	opened    chan string
	closed    chan error
}

// newAcceptTestPeer connects a raw client association to an SCTPTransport
// running acceptDataChannels, with fragments of stuckSID dropped on the way.
func newAcceptTestPeer(t *testing.T, stuckSID uint16, openTimeout time.Duration) *acceptTestPeer {
	t.Helper()

	clientConn, serverConn := net.Pipe()
	filter := &fragmentDroppingConn{Conn: clientConn, dropSID: stuckSID}
	loggerFactory := logging.NewDefaultLoggerFactory()

	type result struct {
		assoc *sctp.Association
		err   error
	}
	serverRes := make(chan result, 1)
	go func() {
		assoc, err := sctp.ServerWithOptions(sctp.WithNetConn(serverConn), sctp.WithLoggerFactory(loggerFactory))
		serverRes <- result{assoc, err}
	}()
	client, err := sctp.ClientWithOptions(sctp.WithNetConn(filter), sctp.WithLoggerFactory(loggerFactory))
	require.NoError(t, err)
	res := <-serverRes
	require.NoError(t, res.err)

	settingEngine := SettingEngine{}
	settingEngine.SetSCTPDataChannelOpenTimeout(openTimeout)
	api := NewAPI(WithSettingEngine(settingEngine))

	transport := api.NewSCTPTransport(nil)
	transport.lock.Lock()
	transport.sctpAssociation = res.assoc
	transport.state = SCTPTransportStateConnected
	transport.lock.Unlock()

	peer := &acceptTestPeer{
		client:    client,
		transport: transport,
		filter:    filter,
		opened:    make(chan string, 16),
		closed:    make(chan error, 1),
	}
	transport.OnDataChannel(func(dc *DataChannel) {
		peer.opened <- dc.Label()
	})
	transport.OnClose(func(err error) {
		peer.closed <- err
	})
	go transport.acceptDataChannels(res.assoc, nil)

	t.Cleanup(func() {
		_ = client.Close()
		_ = transport.Stop()
		_ = clientConn.Close()
		_ = serverConn.Close()
	})

	return peer
}

// openStuckStream starts a message on sid whose tail never reaches the
// server, and waits until the head has been forwarded so the server has
// admitted the stream.
func (p *acceptTestPeer) openStuckStream(
	t *testing.T,
	sid uint16,
	ppi sctp.PayloadProtocolIdentifier,
	configure func(*sctp.Stream),
) *sctp.Stream {
	t.Helper()

	stream, err := p.client.OpenStream(sid, ppi)
	require.NoError(t, err)
	if configure != nil {
		configure(stream)
	}
	_, err = stream.WriteSCTP(make([]byte, 1300), ppi)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return p.filter.firstFragments.Load() > 0 && p.filter.dropped.Load() > 0
	}, 5*time.Second, 5*time.Millisecond, "stuck stream never reached the server")

	return stream
}

func (p *acceptTestPeer) dial(t *testing.T, sid uint16) string {
	t.Helper()

	label := fmt.Sprintf("dc-%d", sid)
	_, err := datachannel.Dial(p.client, sid, &datachannel.Config{
		ChannelType:   datachannel.ChannelTypeReliable,
		Label:         label,
		LoggerFactory: logging.NewDefaultLoggerFactory(),
	})
	require.NoError(t, err)

	return label
}

func (p *acceptTestPeer) expectOpened(t *testing.T, labels []string, timeout time.Duration) {
	t.Helper()

	want := make(map[string]struct{}, len(labels))
	for _, l := range labels {
		want[l] = struct{}{}
	}
	deadline := time.After(timeout)
	for len(want) > 0 {
		select {
		case l := <-p.opened:
			delete(want, l)
		case err := <-p.closed:
			require.FailNowf(t, "accept loop exited", "err=%v, still waiting for %v", err, want)
		case <-deadline:
			require.FailNowf(t, "data channels were not accepted", "still waiting for %v", want)
		}
	}
}

// waitForReset waits until the server resets the client's stream.
func waitForReset(t *testing.T, stream *sctp.Stream, timeout time.Duration) {
	t.Helper()

	readErr := make(chan error, 1)
	go func() {
		buf := make([]byte, 2048)
		for {
			if _, _, err := stream.ReadSCTP(buf); err != nil {
				readErr <- err

				return
			}
		}
	}()

	select {
	case err := <-readErr:
		assert.ErrorIs(t, err, io.EOF)
	case <-time.After(timeout):
		require.FailNow(t, "server never closed the stuck stream")
	}
}

// A stream whose DCEP OPEN never arrives must neither block DataChannels
// accepted after it nor stop the accept loop once its open times out.
func TestSCTPTransportAcceptDataChannelsStuckOpen(t *testing.T) {
	const (
		stuckSID    = 1
		openTimeout = 2 * time.Second
	)
	peer := newAcceptTestPeer(t, stuckSID, openTimeout)

	start := time.Now()
	stuck := peer.openStuckStream(t, stuckSID, sctp.PayloadTypeWebRTCDCEP, nil)

	peer.expectOpened(t, []string{peer.dial(t, 3), peer.dial(t, 5), peer.dial(t, 7)}, openTimeout/2)
	assert.Less(t, time.Since(start), openTimeout, "later channels were blocked behind the stuck open")

	waitForReset(t, stuck, 5*openTimeout)

	peer.expectOpened(t, []string{peer.dial(t, 9)}, 5*time.Second)
}

// A per-stream io.EOF while waiting for the DCEP OPEN (the peer reset the
// stream) must close that stream only, not end the accept loop.
func TestSCTPTransportAcceptDataChannelsStreamEOF(t *testing.T) {
	const stuckSID = 1
	// Long enough that only io.EOF, not the timeout, can end the open.
	peer := newAcceptTestPeer(t, stuckSID, time.Minute)

	// Partially reliable so the dropped tail is abandoned and the peer's
	// stream reset below is not held back by the TSN gap.
	stream := peer.openStuckStream(t, stuckSID, sctp.PayloadTypeWebRTCBinary, func(s *sctp.Stream) {
		s.SetReliabilityParams(false, sctp.ReliabilityTypeRexmit, 1)
	})

	require.NoError(t, stream.Close())
	// The server answers the peer's reset by resetting its own side, which
	// only happens if it handled the io.EOF by closing the stream.
	waitForReset(t, stream, 15*time.Second)

	peer.expectOpened(t, []string{peer.dial(t, 3), peer.dial(t, 5)}, 5*time.Second)

	select {
	case err := <-peer.closed:
		assert.Fail(t, "accept loop exited", "err=%v", err)
	default:
	}
}
