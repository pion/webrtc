// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js

package webrtc

import (
	"context"
	"encoding/binary"
	"hash/crc32"
	"io"
	"net"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"weak"

	"github.com/pion/datachannel"
	"github.com/pion/sctp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// restartTestConn changes only INIT stream counts, so the test uses the public
// SCTP API and real authenticated restart cookies rather than fork options.
type restartTestConn struct {
	*net.UDPConn
	remote         *net.UDPAddr
	streams        uint16
	dropResets     atomic.Bool
	resetDrops     atomic.Uint32
	initAckStreams atomic.Uint32
}

func (c *restartTestConn) RemoteAddr() net.Addr { return c.remote }

//nolint:cyclop
func (c *restartTestConn) Write(packet []byte) (int, error) {
	packet = append([]byte(nil), packet...)
	changed := false
	for offset := 12; offset+4 <= len(packet); {
		size := int(binary.BigEndian.Uint16(packet[offset+2:]))
		if size < 4 || offset+size > len(packet) {
			break
		}
		switch packet[offset] {
		case 1: // INIT: outbound and inbound counts follow the receive window.
			if c.streams != 0 && size >= 20 {
				binary.BigEndian.PutUint16(packet[offset+12:], c.streams)
				binary.BigEndian.PutUint16(packet[offset+14:], c.streams)
				changed = true
			}
		case 2: // Observe the actual negotiated counts in INIT ACK.
			if size >= 20 {
				outbound := binary.BigEndian.Uint16(packet[offset+12:])
				inbound := binary.BigEndian.Uint16(packet[offset+14:])
				c.initAckStreams.Store(uint32(min(inbound, outbound)))
			}
		case 130: // Keep stream reset pending until the peer restarts.
			if c.dropResets.Load() {
				c.resetDrops.Add(1)

				return len(packet), nil
			}
		default:
		}
		offset += (size + 3) &^ 3
	}
	if changed {
		clear(packet[8:12])
		binary.LittleEndian.PutUint32(packet[8:12], crc32.Checksum(packet, crc32.MakeTable(crc32.Castagnoli)))
	}

	return c.WriteToUDP(packet, c.remote)
}

type restartTestPair struct {
	transport   *SCTPTransport
	association *sctp.Association
	peer        *sctp.Association
	conn        *restartTestConn
	peerAddr    *net.UDPAddr
	localAddr   *net.UDPAddr
}

func newRestartTestPair(t *testing.T) *restartTestPair {
	t.Helper()
	listen := func(address *net.UDPAddr) *net.UDPConn {
		conn, err := net.ListenUDP("udp4", address)
		require.NoError(t, err)
		t.Cleanup(func() { _ = conn.Close() })

		return conn
	}
	local := listen(&net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	peer := listen(&net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	localAddr, ok := local.LocalAddr().(*net.UDPAddr)
	require.True(t, ok)
	peerAddr, ok := peer.LocalAddr().(*net.UDPAddr)
	require.True(t, ok)
	conn := &restartTestConn{UDPConn: local, remote: peerAddr}
	type result struct {
		association *sctp.Association
		err         error
	}
	accepted := make(chan result, 1)
	go func() {
		association, err := sctp.ServerWithOptions(sctp.WithNetConn(conn))
		accepted <- result{association, err}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	peerAssociation, err := sctp.ClientContext(ctx, sctp.WithNetConn(&restartTestConn{
		UDPConn: peer, remote: localAddr, streams: 8,
	}))
	require.NoError(t, err)
	t.Cleanup(func() { _ = peerAssociation.Close() })
	var association *sctp.Association
	select {
	case incoming := <-accepted:
		require.NoError(t, incoming.err)
		association = incoming.association
	case <-ctx.Done():
		require.FailNow(t, "initial SCTP handshake timed out")
	}
	t.Cleanup(func() { _ = association.Close() })
	require.Equal(t, uint32(8), conn.initAckStreams.Load())
	var settingEngine SettingEngine
	settingEngine.DetachDataChannels()
	transport := NewAPI(WithSettingEngine(settingEngine)).NewSCTPTransport(nil)
	transport.setAssociation(association)
	// Upstream has no initial stream-count query. Seed the cache from the
	// negotiated INIT ACK; integrations may already initialize this limit.
	transport.lock.Lock()
	initialLimit := uint16(8)
	transport.maxChannels = &initialLimit
	transport.lock.Unlock()
	acceptDone := make(chan struct{})
	go func() { transport.acceptDataChannels(association, nil); close(acceptDone) }()
	t.Cleanup(func() {
		assert.NoError(t, transport.Stop())
		select {
		case <-acceptDone:
		case <-time.After(5 * time.Second):
			assert.Fail(t, "data channel accept loop did not stop")
		}
	})

	return &restartTestPair{transport: transport, association: association, peer: peerAssociation, conn: conn, peerAddr: peerAddr, localAddr: localAddr}
}

func (p *restartTestPair) restart(ctx context.Context) (*sctp.Association, error) {
	if err := p.peer.Close(); err != nil {
		return nil, err
	}
	conn, err := net.ListenUDP("udp4", p.peerAddr)
	if err != nil {
		return nil, err
	}
	association, err := sctp.ClientContext(ctx, sctp.WithNetConn(&restartTestConn{
		UDPConn: conn, remote: p.localAddr, streams: 4,
	}))
	if err != nil {
		_ = conn.Close()
	}

	return association, err
}

func (p *restartTestPair) openLocal(t *testing.T, id uint16) (*DataChannel, *datachannel.DataChannel) {
	t.Helper()
	type result struct {
		channel *datachannel.DataChannel
		err     error
	}
	accepted := make(chan result, 1)
	go func() {
		channel, err := datachannel.Accept(p.peer, &datachannel.Config{LoggerFactory: p.transport.api.settingEngine.LoggerFactory})
		accepted <- result{channel, err}
	}()
	channel, err := p.transport.api.NewDataChannel(p.transport, &DataChannelParameters{ID: &id, Label: "restart", Ordered: true})
	require.NoError(t, err)
	select {
	case incoming := <-accepted:
		require.NoError(t, incoming.err)

		return channel, incoming.channel
	case <-time.After(5 * time.Second):
		require.FailNow(t, "DCEP OPEN was not accepted")

		return nil, nil
	}
}

func restartReservation(t *testing.T, transport *SCTPTransport, channel *DataChannel) dataChannelReservation {
	t.Helper()
	transport.lock.RLock()
	defer transport.lock.RUnlock()
	reservation, ok := transport.dataChannelReservations[weak.Make(channel)]
	require.True(t, ok)

	return reservation
}

func restartIDCount(transport *SCTPTransport, id uint16) uint32 {
	transport.lock.RLock()
	defer transport.lock.RUnlock()

	return transport.dataChannelIDsUsed[id]
}

func TestSCTPTransportPeerRestart(t *testing.T) {
	for _, detachedClose := range []bool{false, true} {
		name := "Close"
		if detachedClose {
			name = "DetachedClose"
		}
		t.Run(name, func(t *testing.T) {
			pair := newRestartTestPair(t)
			transport := pair.transport
			closing, _ := pair.openLocal(t, 0)
			retained, _ := pair.openLocal(t, 2)
			old := restartReservation(t, transport, closing)
			kept := restartReservation(t, transport, retained)

			// Also retain a remotely opened DCEP channel, including its reservation.
			incoming := make(chan *DataChannel, 1)
			transport.OnDataChannel(func(channel *DataChannel) { incoming <- channel })
			_, err := datachannel.Dial(pair.peer, 1, &datachannel.Config{
				ChannelType: datachannel.ChannelTypeReliable, LoggerFactory: transport.api.settingEngine.LoggerFactory,
			})
			require.NoError(t, err)
			var remote *DataChannel
			select {
			case remote = <-incoming:
			case <-time.After(5 * time.Second):
				require.FailNow(t, "remote DCEP channel was not accepted")
			}
			require.Eventually(t, func() bool { return remote.ReadyState() == DataChannelStateOpen }, 5*time.Second, time.Millisecond)
			remoteStream := restartReservation(t, transport, remote).stream.Value()
			remoteReader, err := remote.DetachWithDeadline()
			require.NoError(t, err)

			pair.conn.dropResets.Store(true)
			if detachedClose {
				detached, detachErr := closing.Detach()
				require.NoError(t, detachErr)
				require.NoError(t, detached.Close())
			} else {
				require.NoError(t, closing.Close())
			}
			require.Eventually(t, func() bool { return pair.conn.resetDrops.Load() != 0 }, 5*time.Second, time.Millisecond)
			require.Equal(t, uint32(1), restartIDCount(transport, 0))
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			replacement, err := pair.restart(ctx)
			require.NoError(t, err)
			t.Cleanup(func() { _ = replacement.Close() })

			assert.Equal(t, uint16(4), transport.MaxChannels())
			assert.Equal(t, uint32(4), pair.conn.initAckStreams.Load())
			assert.Equal(t, sctp.StreamStateClosed, old.stream.Value().State())
			assert.Equal(t, uint32(0), restartIDCount(transport, 0), "discarded stream must release its ID")
			assert.Same(t, kept.stream.Value(), restartReservation(t, transport, retained).stream.Value())
			assert.Same(t, remoteStream, restartReservation(t, transport, remote).stream.Value())
			assert.Equal(t, uint32(1), restartIDCount(transport, 1))
			assert.Equal(t, uint32(1), restartIDCount(transport, 2))

			// The retained WebRTC objects still carry user data in both directions.
			replacementKept, err := replacement.OpenStream(2, sctp.PayloadTypeWebRTCBinary)
			require.NoError(t, err)
			require.NoError(t, replacementKept.SetReadDeadline(time.Now().Add(5*time.Second)))
			require.NoError(t, retained.SendText("retained local"))
			buffer := make([]byte, 64)
			n, _, err := replacementKept.ReadSCTP(buffer)
			require.NoError(t, err)
			assert.Equal(t, "retained local", string(buffer[:n]))
			replacementRemote, err := replacement.OpenStream(1, sctp.PayloadTypeWebRTCBinary)
			require.NoError(t, err)
			require.NoError(t, remoteReader.SetReadDeadline(time.Now().Add(5*time.Second)))
			_, err = replacementRemote.Write([]byte("retained remote"))
			require.NoError(t, err)
			n, _, err = remoteReader.ReadDataChannel(buffer)
			require.NoError(t, err)
			assert.Equal(t, "retained remote", string(buffer[:n]))

			reused, err := transport.api.newDataChannel(&DataChannelParameters{Ordered: true}, nil, transport.log)
			require.NoError(t, err)
			require.NoError(t, transport.generateAndSetDataChannelID(DTLSRoleClient, &reused.id, reused))
			require.Equal(t, uint16(0), *reused.id)
			var exhaustedID *uint16
			require.ErrorIs(t, transport.generateAndSetDataChannelID(DTLSRoleClient, &exhaustedID, &DataChannel{}), ErrMaxDataChannelID)
			// SID 3 is valid when four streams were negotiated.
			require.NoError(t, transport.generateAndSetDataChannelID(DTLSRoleServer, &exhaustedID, &DataChannel{}))
			assert.Equal(t, uint16(3), *exhaustedID)
			require.ErrorIs(t, transport.generateAndSetDataChannelID(DTLSRoleServer, &exhaustedID, &DataChannel{}), ErrMaxDataChannelID)

			// Reuse the freed ID in another real DCEP handshake.
			accepted := make(chan error, 1)
			go func() {
				_, acceptErr := datachannel.Accept(replacement, &datachannel.Config{LoggerFactory: transport.api.settingEngine.LoggerFactory})
				accepted <- acceptErr
			}()
			require.NoError(t, reused.open(transport))
			select {
			case acceptErr := <-accepted:
				require.NoError(t, acceptErr)
			case <-ctx.Done():
				require.FailNow(t, "reused ID could not complete DCEP")
			}
			assert.NotSame(t, old.stream.Value(), restartReservation(t, transport, reused).stream.Value())
			assert.Equal(t, uint32(1), restartIDCount(transport, 0))
		})
	}
}

func TestSCTPTransportPeerRestartRegistrationRace(t *testing.T) {
	pair := newRestartTestPair(t)
	transport := pair.transport
	closing, _ := pair.openLocal(t, 0)
	old := restartReservation(t, transport, closing)
	pair.conn.dropResets.Store(true)
	require.NoError(t, closing.Close())
	require.Eventually(t, func() bool { return pair.conn.resetDrops.Load() != 0 }, 5*time.Second, time.Millisecond)

	entered := make(chan sctp.AssociationRestartEvent, 1)
	released := make(chan struct{})
	var releaseOnce sync.Once
	resume := func() { releaseOnce.Do(func() { close(released) }) }
	defer resume()
	pair.association.OnAssociationRestart(func(event sctp.AssociationRestartEvent) {
		entered <- event
		<-released
		transport.onAssociationRestart(pair.association, event)
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	type result struct {
		association *sctp.Association
		err         error
	}
	restarted := make(chan result, 1)
	go func() { association, err := pair.restart(ctx); restarted <- result{association, err} }()
	var event sctp.AssociationRestartEvent
	select {
	case event = <-entered:
	case <-ctx.Done():
		require.FailNow(t, "restart observer was not called")
	}
	require.Equal(t, sctp.StreamStateClosed, old.stream.Value().State())
	require.Equal(t, event.Generation, old.stream.Value().AssociationGeneration())

	// This registration occurs after SCTP took its retained-stream snapshot,
	// but before the transport processes that snapshot. The ID overlaps the old
	// closing channel and must survive that channel's reconciliation.
	fresh, err := transport.api.newDataChannel(&DataChannelParameters{}, nil, transport.log)
	require.NoError(t, err)
	freshStream, err := pair.association.OpenStream(0, sctp.PayloadTypeWebRTCBinary)
	require.NoError(t, err)
	require.NotContains(t, event.RetainedStreams, freshStream)
	require.NoError(t, transport.bindDataChannel(fresh, pair.association, freshStream))
	require.Equal(t, uint32(2), restartIDCount(transport, 0))
	require.Equal(t, event.Generation, restartReservation(t, transport, fresh).generation)
	stale := &DataChannel{}
	transport.lock.Lock()
	transport.reserveDataChannelID(stale, 0)
	transport.lock.Unlock()
	require.ErrorIs(t, transport.bindDataChannel(stale, pair.association, old.stream.Value()), io.ErrClosedPipe)
	require.Equal(t, uint32(2), restartIDCount(transport, 0), "rejecting the stale stream must release only its owner")
	pending := &DataChannel{}
	transport.lock.Lock()
	transport.reserveDataChannelID(pending, 3)
	transport.lock.Unlock()
	resume()
	select {
	case incoming := <-restarted:
		require.NoError(t, incoming.err)
		t.Cleanup(func() { _ = incoming.association.Close() })
	case <-ctx.Done():
		require.FailNow(t, "restart did not finish")
	}
	assert.Equal(t, uint16(4), transport.MaxChannels())
	assert.Equal(t, uint32(1), restartIDCount(transport, 0))
	assert.Equal(t, uint32(1), restartIDCount(transport, 3), "unbound pending opens must survive")
	assert.Same(t, freshStream, restartReservation(t, transport, fresh).stream.Value())
	transport.onAssociationRestart(pair.association, event)
	assert.Equal(t, uint32(1), restartIDCount(transport, 0), "replayed notifications cannot release the new binding")
	// Bind a pending reservation after reconciliation to the discarded object.
	transport.lock.Lock()
	transport.reserveDataChannelID(stale, 0)
	transport.lock.Unlock()
	require.ErrorIs(t, transport.bindDataChannel(stale, pair.association, old.stream.Value()), io.ErrClosedPipe)
	assert.Equal(t, uint32(1), restartIDCount(transport, 0))
	runtime.KeepAlive(pending)
	runtime.KeepAlive(fresh)
	runtime.KeepAlive(closing)
}

func TestSCTPTransportPeerRestartStop(t *testing.T) {
	pair := newRestartTestPair(t)
	transport := pair.transport
	closing, _ := pair.openLocal(t, 0)
	pair.conn.dropResets.Store(true)
	require.NoError(t, closing.Close())
	entered := make(chan sctp.AssociationRestartEvent, 1)
	released := make(chan struct{})
	var releaseOnce sync.Once
	resume := func() { releaseOnce.Do(func() { close(released) }) }
	defer resume()
	pair.association.OnAssociationRestart(func(event sctp.AssociationRestartEvent) {
		entered <- event
		<-released
		transport.onAssociationRestart(pair.association, event)
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	restarted := make(chan struct{})
	go func() {
		association, _ := pair.restart(ctx)
		if association != nil {
			_ = association.Close()
		}
		close(restarted)
	}()
	var event sctp.AssociationRestartEvent
	select {
	case event = <-entered:
	case <-ctx.Done():
		require.FailNow(t, "restart observer was not called")
	}
	stopped := make(chan error, 1)
	go func() { stopped <- transport.Stop() }()
	require.Eventually(t, func() bool { return transport.association() == nil }, 5*time.Second, time.Millisecond)
	resume()
	select {
	case stopErr := <-stopped:
		require.NoError(t, stopErr)
	case <-ctx.Done():
		require.FailNow(t, "Stop deadlocked with the restart observer")
	}
	assert.Equal(t, SCTPTransportStateClosed, transport.State())
	assert.Equal(t, uint16(8), transport.MaxChannels(), "a stopped transport ignores the in-flight notification")
	assert.Equal(t, uint32(1), restartIDCount(transport, 0))
	event.Generation++
	transport.onAssociationRestart(pair.association, event)
	assert.Equal(t, uint16(8), transport.MaxChannels())
	assert.Equal(t, uint32(1), restartIDCount(transport, 0))
	cancel()
	select {
	case <-restarted:
	case <-time.After(5 * time.Second):
		require.FailNow(t, "replacement association did not stop")
	}
	runtime.KeepAlive(closing)
}

func TestSCTPTransportPeerRestartObsoleteAssociation(t *testing.T) {
	pair := newRestartTestPair(t)
	transport := pair.transport
	channel, _ := pair.openLocal(t, 0)
	// The transport now owns another association; the previous observer may
	// already have captured its handler before replacement.
	transport.setAssociation(pair.peer)
	transport.onAssociationRestart(pair.association, sctp.AssociationRestartEvent{
		Generation: 1, NumInboundStreams: 4, NumOutboundStreams: 4,
	})
	assert.Equal(t, uint16(8), transport.MaxChannels())
	assert.Equal(t, uint32(1), restartIDCount(transport, 0))
	assert.Same(t, pair.peer, transport.association())
	runtime.KeepAlive(channel)
}

func closeDetachedRestartChannel(t *testing.T, pair *restartTestPair) (weak.Pointer[DataChannel], weak.Pointer[sctp.Stream]) {
	t.Helper()
	channel, peerChannel := pair.openLocal(t, 0)
	stream := restartReservation(t, pair.transport, channel).stream
	owner := weak.Make(channel)
	detached, err := channel.Detach()
	require.NoError(t, err)
	require.NoError(t, detached.Close())
	require.NoError(t, peerChannel.SetReadDeadline(time.Now().Add(5*time.Second)))
	_, _, err = peerChannel.ReadDataChannel(make([]byte, 64))
	require.ErrorIs(t, err, io.EOF)
	require.Eventually(t, func() bool { return stream.Value().State() == sctp.StreamStateClosed }, 5*time.Second, time.Millisecond)
	runtime.KeepAlive(channel)

	return owner, stream
}

func TestSCTPTransportPeerRestartCollectedReservations(t *testing.T) {
	pair := newRestartTestPair(t)
	// Repeated detached closes must not leave strong references to either the
	// WebRTC channel or its closed SCTP stream in the transport's bookkeeping.
	for range 4 {
		owner, stream := closeDetachedRestartChannel(t, pair)
		require.Eventually(t, func() bool {
			runtime.GC()

			return owner.Value() == nil && stream.Value() == nil
		}, 5*time.Second, 20*time.Millisecond)
	}
	live, _ := pair.openLocal(t, 0)
	pair.transport.lock.RLock()
	assert.Len(t, pair.transport.dataChannelReservations, 1)
	assert.Len(t, pair.transport.expiredDataChannelReservations, 1, "expired bindings of one SID/generation are coalesced")
	pair.transport.lock.RUnlock()
	require.Equal(t, uint32(5), restartIDCount(pair.transport, 0))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	replacement, err := pair.restart(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = replacement.Close() })
	assert.Equal(t, uint32(1), restartIDCount(pair.transport, 0), "only the live overlapping owner is retained")
	assert.Equal(t, uint64(1), restartReservation(t, pair.transport, live).generation)
	pair.transport.lock.RLock()
	assert.Empty(t, pair.transport.expiredDataChannelReservations)
	pair.transport.lock.RUnlock()
	runtime.KeepAlive(live)
}

func TestSCTPTransportPeerRestartPendingAnnouncement(t *testing.T) {
	pair := newRestartTestPair(t)
	transport := pair.transport
	pair.conn.dropResets.Store(true)
	entered := make(chan *DataChannel, 1)
	released := make(chan struct{})
	var releaseOnce sync.Once
	resume := func() { releaseOnce.Do(func() { close(released) }) }
	defer resume()
	var staleOpened atomic.Bool
	transport.OnDataChannel(func(channel *DataChannel) {
		if *channel.ID() != 0 {
			return
		}
		channel.OnOpen(func() { staleOpened.Store(true) })
		stream := restartReservation(t, transport, channel).stream.Value()
		assert.NoError(t, stream.Close())
		entered <- channel
		<-released
	})
	_, err := datachannel.Dial(pair.peer, 0, &datachannel.Config{
		ChannelType: datachannel.ChannelTypeReliable, LoggerFactory: transport.api.settingEngine.LoggerFactory,
	})
	require.NoError(t, err)
	var stale *DataChannel
	select {
	case stale = <-entered:
	case <-time.After(5 * time.Second):
		require.FailNow(t, "announcement handler did not start")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	replacement, err := pair.restart(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = replacement.Close() })
	require.Equal(t, uint32(0), restartIDCount(transport, 0))
	fresh := &DataChannel{}
	var id *uint16
	require.NoError(t, transport.generateAndSetDataChannelID(DTLSRoleClient, &id, fresh))
	require.Equal(t, uint16(0), *id)
	accepted := make(chan uint16, 1)
	transport.OnDataChannelOpened(func(channel *DataChannel) { accepted <- *channel.ID() })
	resume()
	_, err = datachannel.Dial(replacement, 1, &datachannel.Config{
		ChannelType: datachannel.ChannelTypeReliable, LoggerFactory: transport.api.settingEngine.LoggerFactory,
	})
	require.NoError(t, err)
	select {
	case openedID := <-accepted:
		assert.Equal(t, uint16(1), openedID, "the discarded channel must never reach handleOpen")
	case <-ctx.Done():
		require.FailNow(t, "subsequent remote DCEP channel was not opened")
	}
	assert.Equal(t, DataChannelStateConnecting, stale.ReadyState())
	assert.False(t, staleOpened.Load())
	assert.Equal(t, uint32(1), restartIDCount(transport, 0), "announcement must not re-reserve the discarded channel")
	runtime.KeepAlive(fresh)
	runtime.KeepAlive(stale)
}

func TestSCTPTransportPeerRestartRejectsNewOutOfRangeBindings(t *testing.T) {
	pair := newRestartTestPair(t)
	transport := pair.transport
	retained, _ := pair.openLocal(t, 6)
	retainedStream := restartReservation(t, transport, retained).stream.Value()
	// An unbound owner can already have obtained the retained Stream pointer.
	late := &DataChannel{}
	lateStream, err := pair.association.OpenStream(6, sctp.PayloadTypeWebRTCBinary)
	require.NoError(t, err)
	require.Same(t, retainedStream, lateStream)
	transport.lock.Lock()
	transport.reserveDataChannelID(late, 6)
	transport.lock.Unlock()
	// Allocate SID 4 while the old eight-stream limit is still in effect, then
	// delay opening it until the authenticated restart shrinks the limit.
	for range 2 {
		var id *uint16
		require.NoError(t, transport.generateAndSetDataChannelID(DTLSRoleClient, &id, &DataChannel{}))
	}
	pending, err := transport.api.newDataChannel(&DataChannelParameters{Ordered: true}, nil, transport.log)
	require.NoError(t, err)
	require.NoError(t, transport.generateAndSetDataChannelID(DTLSRoleClient, &pending.id, pending))
	require.Equal(t, uint16(4), *pending.id)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	replacement, err := pair.restart(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = replacement.Close() })
	require.Equal(t, uint16(4), transport.MaxChannels())
	require.Equal(t, uint32(2), restartIDCount(transport, 6))
	require.ErrorIs(t, transport.bindDataChannel(late, pair.association, lateStream), ErrMaxDataChannelID)
	require.Equal(t, uint32(1), restartIDCount(transport, 6), "binding must reject the new owner of an existing out-of-range stream")
	require.Equal(t, uint32(1), restartIDCount(transport, 4))
	invalidID := uint16(4)
	_, err = transport.api.NewDataChannel(transport, &DataChannelParameters{ID: &invalidID})
	require.ErrorIs(t, err, ErrMaxDataChannelID)
	require.Equal(t, uint32(1), restartIDCount(transport, 4), "an explicit failed owner must not release the pending owner")
	require.ErrorIs(t, pending.open(transport), ErrMaxDataChannelID)
	require.Equal(t, uint32(0), restartIDCount(transport, 4), "a preallocated out-of-range ID must release its own reservation")
	// A new owner cannot piggyback on a retained out-of-range Stream object.
	invalidID = 6
	_, err = transport.api.NewDataChannel(transport, &DataChannelParameters{ID: &invalidID})
	require.ErrorIs(t, err, ErrMaxDataChannelID)
	require.Equal(t, uint32(1), restartIDCount(transport, 6))
	failed := &DataChannel{}
	transport.lock.Lock()
	transport.reserveDataChannelID(failed, 6)
	transport.lock.Unlock()
	require.ErrorIs(t, transport.bindDataChannel(failed, pair.association, nil), io.ErrClosedPipe)
	require.Equal(t, uint32(1), restartIDCount(transport, 6), "a nil stream must release only its failed owner")
	require.NoError(t, transport.bindDataChannel(retained, pair.association, retainedStream))
	require.Equal(t, sctp.StreamStateOpen, retainedStream.State())
	// The retained binding, despite its SID, continues carrying actual user data.
	receiver, err := replacement.OpenStream(6, sctp.PayloadTypeWebRTCBinary)
	require.NoError(t, err)
	require.NoError(t, receiver.SetReadDeadline(time.Now().Add(5*time.Second)))
	require.NoError(t, retained.SendText("retained above reduced limit"))
	buffer := make([]byte, 64)
	n, _, err := receiver.ReadSCTP(buffer)
	require.NoError(t, err)
	assert.Equal(t, "retained above reduced limit", string(buffer[:n]))
	runtime.KeepAlive(pending)
	runtime.KeepAlive(retained)
}

func TestSCTPTransportFailedOpenReleasesReservation(t *testing.T) {
	for _, existingOwner := range []bool{false, true} {
		name := "unreserved ID"
		if existingOwner {
			name = "overlapping existing owner"
		}
		t.Run(name, func(t *testing.T) {
			pair := newRestartTestPair(t)
			transport := pair.transport
			id := uint16(0)
			var retained *DataChannel
			if existingOwner {
				retained, _ = pair.openLocal(t, id)
			}
			baseline := restartIDCount(transport, id)
			_, err := transport.api.NewDataChannel(transport, &DataChannelParameters{
				ID: &id, Ordered: true, Protocol: strings.Repeat("x", 1<<16),
			})
			require.ErrorIs(t, err, datachannel.ErrTooLongProtocol)
			assert.Equal(t, baseline, restartIDCount(transport, id), "failed DCEP marshal must release only its newly bound owner")
			transport.lock.RLock()
			assert.Len(t, transport.dataChannelReservations, int(baseline))
			transport.lock.RUnlock()
			if retained != nil {
				assert.Equal(t, sctp.StreamStateOpen, restartReservation(t, transport, retained).stream.Value().State())
			}
			runtime.KeepAlive(retained)
		})
	}
}
