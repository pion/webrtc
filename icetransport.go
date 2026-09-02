// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js

package webrtc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pion/ice/v4"
	"github.com/pion/logging"
	"github.com/pion/transport/v4/packetio"
	"github.com/pion/webrtc/v4/internal/netconn"
	"github.com/pion/webrtc/v4/internal/util"
)

const maxPendingTransportPackets = 15

type iceEndpointKind uint8

const (
	iceEndpointSRTP iceEndpointKind = iota
	iceEndpointSRTCP
)

// ICETransport allows an application access to information about the ICE
// transport over which packets are sent and received.
type ICETransport struct {
	lock sync.RWMutex

	role ICERole

	onConnectionStateChangeHandler         atomic.Value // func(ICETransportState)
	internalOnConnectionStateChangeHandler atomic.Value // func(ICETransportState)
	onSelectedCandidatePairChangeHandler   atomic.Value // func(*ICECandidatePair)

	state atomic.Value // ICETransportState

	gatherer *ICEGatherer
	conn     net.Conn

	packetLock     sync.Mutex
	dtlsHandler    func([]byte) error
	endpoints      [2]*netconn.Conn
	pendingPackets [][]byte
	readLoopDone   chan struct{}

	ctxCancel func()

	loggerFactory logging.LoggerFactory

	log logging.LeveledLogger
}

// GetSelectedCandidatePair returns the selected candidate pair on which packets are sent
// if there is no selected pair nil is returned.
func (t *ICETransport) GetSelectedCandidatePair() (*ICECandidatePair, error) {
	agent := t.gatherer.getAgent()
	if agent == nil {
		return nil, nil //nolint:nilnil
	}

	icePair, err := agent.GetSelectedCandidatePair()
	if icePair == nil || err != nil {
		return nil, err
	}

	local, err := newICECandidateFromICE(icePair.Local, "", 0)
	if err != nil {
		return nil, err
	}

	remote, err := newICECandidateFromICE(icePair.Remote, "", 0)
	if err != nil {
		return nil, err
	}

	return NewICECandidatePair(&local, &remote), nil
}

// GetSelectedCandidatePairStats returns the selected candidate pair stats on which packets are sent
// if there is no selected pair empty stats, false is returned to indicate stats not available.
func (t *ICETransport) GetSelectedCandidatePairStats() (ICECandidatePairStats, bool) {
	return t.gatherer.getSelectedCandidatePairStats()
}

// NewICETransport creates a new NewICETransport.
func NewICETransport(gatherer *ICEGatherer, loggerFactory logging.LoggerFactory) *ICETransport {
	iceTransport := &ICETransport{
		gatherer:      gatherer,
		loggerFactory: loggerFactory,
		log:           loggerFactory.NewLogger("ortc"),
	}
	iceTransport.setState(ICETransportStateNew)

	return iceTransport
}

// Start incoming connectivity checks based on its configured role.
func (t *ICETransport) Start(gatherer *ICEGatherer, params ICEParameters, role *ICERole) error {
	return t.StartContext(context.Background(), gatherer, params, role)
}

// StartContext incoming connectivity checks based on its configured role.
// If the context is canceled, the ICE transport will stop.
//
//nolint:cyclop
func (t *ICETransport) StartContext(
	ctx context.Context,
	gatherer *ICEGatherer,
	params ICEParameters,
	role *ICERole,
) error {
	t.lock.Lock()
	defer t.lock.Unlock()

	if t.State() != ICETransportStateNew {
		return errICETransportNotInNew
	}

	if gatherer != nil {
		t.gatherer = gatherer
	}

	if err := t.ensureGatherer(); err != nil {
		return err
	}

	agent := t.gatherer.getAgent()
	if agent == nil {
		return fmt.Errorf("%w: unable to start ICETransport", errICEAgentNotExist)
	}

	if err := agent.OnConnectionStateChange(func(iceState ice.ConnectionState) {
		state := newICETransportStateFromICE(iceState)

		t.setState(state)
		t.onConnectionStateChange(state)
	}); err != nil {
		return err
	}
	if err := agent.OnSelectedCandidatePairChange(func(local, remote ice.Candidate) {
		candidates, err := newICECandidatesFromICE([]ice.Candidate{local, remote}, "", 0)
		if err != nil {
			t.log.Warnf("%w: %s", errICECandiatesCoversionFailed, err)

			return
		}
		t.onSelectedCandidatePairChange(NewICECandidatePair(&candidates[0], &candidates[1]))
	}); err != nil {
		return err
	}
	if err := agent.SetRemoteICELite(params.ICELite); err != nil {
		return err
	}

	if role == nil {
		controlled := ICERoleControlled
		role = &controlled
	}
	t.role = *role

	callerCtx := ctx
	operationCtx, ctxCancel := context.WithCancel(callerCtx)
	t.ctxCancel = ctxCancel

	// Drop the lock here to allow ICE candidates to be
	// added so that the agent can complete a connection
	t.lock.Unlock()

	var iceConn *ice.Conn
	var err error
	switch *role {
	case ICERoleControlling:
		iceConn, err = agent.Dial(operationCtx,
			params.UsernameFragment,
			params.Password)

	case ICERoleControlled:
		iceConn, err = agent.Accept(operationCtx,
			params.UsernameFragment,
			params.Password)

	default:
		err = errICERoleUnknown
	}

	// Reacquire the lock to set the connection and start WebRTC packet dispatch.
	t.lock.Lock()
	if err != nil {
		if ctxErr := callerCtx.Err(); ctxErr != nil {
			t.lock.Unlock()
			_ = t.Stop()
			t.lock.Lock()

			return ctxErr
		}

		ctxCancel()
		t.ctxCancel = nil

		return err
	}

	if t.State() == ICETransportStateClosed {
		return errICETransportClosed
	}

	t.conn = iceConn
	t.readLoopDone = make(chan struct{})
	go t.readLoop(iceConn, t.readLoopDone)

	return nil
}

// restart is not exposed currently because ORTC has users create a whole new ICETransport
// so for now lets keep it private so we don't cause ORTC users to depend on non-standard APIs.
func (t *ICETransport) restart() error {
	t.lock.Lock()
	defer t.lock.Unlock()

	agent := t.gatherer.getAgent()
	if agent == nil {
		return fmt.Errorf("%w: unable to restart ICETransport", errICEAgentNotExist)
	}

	if err := agent.Restart(
		t.gatherer.api.settingEngine.candidates.UsernameFragment,
		t.gatherer.api.settingEngine.candidates.Password,
	); err != nil {
		return err
	}

	return t.gatherer.Gather()
}

// Stop irreversibly stops the ICETransport.
func (t *ICETransport) Stop() error {
	return t.stop(false /* shouldGracefullyClose */)
}

// GracefulStop irreversibly stops the ICETransport. It also waits
// for any goroutines it started to complete. This is only safe to call outside of
// ICETransport callbacks or if in a callback, in its own goroutine.
func (t *ICETransport) GracefulStop() error {
	return t.stop(true /* shouldGracefullyClose */)
}

func (t *ICETransport) stop(shouldGracefullyClose bool) error {
	t.lock.Lock()
	t.setState(ICETransportStateClosed)

	if t.ctxCancel != nil {
		t.ctxCancel()
	}

	conn := t.conn
	readLoopDone := t.readLoopDone
	gatherer := t.gatherer
	t.lock.Unlock()

	if conn != nil {
		var closeErrs []error
		if shouldGracefullyClose && gatherer != nil {
			closeErrs = append(closeErrs, gatherer.GracefulClose())
		}
		closeErrs = append(closeErrs, conn.Close())
		if readLoopDone != nil {
			<-readLoopDone
		}
		t.closeEndpoints()

		return util.FlattenErrs(closeErrs)
	} else if gatherer != nil {
		if shouldGracefullyClose {
			return gatherer.GracefulClose()
		}

		return gatherer.Close()
	}

	return nil
}

// OnSelectedCandidatePairChange sets a handler that is invoked when a new
// ICE candidate pair is selected.
func (t *ICETransport) OnSelectedCandidatePairChange(f func(*ICECandidatePair)) {
	t.onSelectedCandidatePairChangeHandler.Store(f)
}

func (t *ICETransport) onSelectedCandidatePairChange(pair *ICECandidatePair) {
	if handler, ok := t.onSelectedCandidatePairChangeHandler.Load().(func(*ICECandidatePair)); ok {
		handler(pair)
	}
}

// OnConnectionStateChange sets a handler that is fired when the ICE
// connection state changes.
func (t *ICETransport) OnConnectionStateChange(f func(ICETransportState)) {
	t.onConnectionStateChangeHandler.Store(f)
}

func (t *ICETransport) onConnectionStateChange(state ICETransportState) {
	if handler, ok := t.onConnectionStateChangeHandler.Load().(func(ICETransportState)); ok {
		handler(state)
	}
	if handler, ok := t.internalOnConnectionStateChangeHandler.Load().(func(ICETransportState)); ok {
		handler(state)
	}
}

// Role indicates the current role of the ICE transport.
func (t *ICETransport) Role() ICERole {
	t.lock.RLock()
	defer t.lock.RUnlock()

	return t.role
}

// SetRemoteCandidates sets the sequence of candidates associated with the remote ICETransport.
func (t *ICETransport) SetRemoteCandidates(remoteCandidates []ICECandidate) error {
	t.lock.RLock()
	defer t.lock.RUnlock()

	if err := t.ensureGatherer(); err != nil {
		return err
	}

	agent := t.gatherer.getAgent()
	if agent == nil {
		return fmt.Errorf("%w: unable to set remote candidates", errICEAgentNotExist)
	}

	for _, c := range remoteCandidates {
		i, err := c.ToICE()
		if err != nil {
			return err
		}

		if err = agent.AddRemoteCandidate(i); err != nil {
			return err
		}
	}

	return nil
}

// AddRemoteCandidate adds a candidate associated with the remote ICETransport.
func (t *ICETransport) AddRemoteCandidate(remoteCandidate *ICECandidate) error {
	t.lock.RLock()
	defer t.lock.RUnlock()

	var (
		candidate ice.Candidate
		err       error
	)

	if err = t.ensureGatherer(); err != nil {
		return err
	}

	if remoteCandidate != nil {
		if candidate, err = remoteCandidate.ToICE(); err != nil {
			return err
		}
	}

	agent := t.gatherer.getAgent()
	if agent == nil {
		return fmt.Errorf("%w: unable to add remote candidates", errICEAgentNotExist)
	}

	return agent.AddRemoteCandidate(candidate)
}

// State returns the current ice transport state.
func (t *ICETransport) State() ICETransportState {
	if v, ok := t.state.Load().(ICETransportState); ok {
		return v
	}

	return ICETransportState(0)
}

// GetLocalParameters returns an IceParameters object which provides information
// uniquely identifying the local peer for the duration of the ICE session.
func (t *ICETransport) GetLocalParameters() (ICEParameters, error) {
	if err := t.ensureGatherer(); err != nil {
		return ICEParameters{}, err
	}

	return t.gatherer.GetLocalParameters()
}

// GetRemoteParameters returns an IceParameters object which provides information
// uniquely identifying the remote peer for the duration of the ICE session.
func (t *ICETransport) GetRemoteParameters() (ICEParameters, error) {
	t.lock.Lock()
	defer t.lock.Unlock()

	agent := t.gatherer.getAgent()
	if agent == nil {
		return ICEParameters{}, fmt.Errorf("%w: unable to get remote parameters", errICEAgentNotExist)
	}

	uFrag, uPwd, err := agent.GetRemoteUserCredentials()
	if err != nil {
		return ICEParameters{}, fmt.Errorf("%w: unable to get remote parameters", err)
	}

	return ICEParameters{
		UsernameFragment: uFrag,
		Password:         uPwd,
	}, nil
}

func (t *ICETransport) setState(i ICETransportState) {
	t.state.Store(i)
}

func (t *ICETransport) newEndpoint(kind iceEndpointKind) *netconn.Conn {
	endpoint := netconn.New(netconn.Config{
		Write:            t.write,
		LocalAddr:        t.localAddr,
		RemoteAddr:       t.remoteAddr,
		SetWriteDeadline: t.setWriteDeadline,
		OnClose:          t.removeEndpoint,
	})

	t.packetLock.Lock()
	t.endpoints[kind] = endpoint
	pending := t.takePendingPacketsLocked(endpointMatcher(kind))
	t.packetLock.Unlock()

	for _, packet := range pending {
		t.handlePacket(endpoint.Push, packet)
	}

	return endpoint
}

func endpointMatcher(kind iceEndpointKind) func([]byte) bool {
	if kind == iceEndpointSRTP {
		return matchSRTP
	}

	return matchSRTCP
}

func (t *ICETransport) setDTLSHandler(handler func([]byte) error) {
	t.packetLock.Lock()
	t.dtlsHandler = handler
	var pending [][]byte
	if handler != nil {
		pending = t.takePendingPacketsLocked(matchDTLS)
	}
	t.packetLock.Unlock()

	for _, packet := range pending {
		t.handlePacket(handler, packet)
	}
}

func (t *ICETransport) removeEndpoint(endpoint *netconn.Conn) {
	t.packetLock.Lock()
	defer t.packetLock.Unlock()

	for kind, current := range t.endpoints {
		if current == endpoint {
			t.endpoints[kind] = nil
		}
	}
}

func (t *ICETransport) readLoop(conn net.Conn, done chan<- struct{}) {
	defer close(done)

	buffer := make([]byte, t.gatherer.api.settingEngine.getReceiveMTU())
	for {
		n, err := conn.Read(buffer)
		switch {
		case errors.Is(err, io.EOF), errors.Is(err, ice.ErrClosed):
			return
		case errors.Is(err, io.ErrShortBuffer), errors.Is(err, packetio.ErrTimeout):
			t.log.Errorf("failed to read WebRTC packet: %v", err)

			continue
		case err != nil:
			t.log.Errorf("WebRTC packet read loop ended: %v", err)

			return
		}

		t.dispatchPacket(buffer[:n])
	}
}

func (t *ICETransport) dispatchPacket(packet []byte) {
	if len(packet) == 0 {
		t.log.Warnf("unable to dispatch zero length packet")

		return
	}

	t.packetLock.Lock()
	var handler func([]byte) error
	switch {
	case matchDTLS(packet):
		handler = t.dtlsHandler
	case matchSRTP(packet):
		if endpoint := t.endpoints[iceEndpointSRTP]; endpoint != nil {
			handler = endpoint.Push
		}
	case matchSRTCP(packet):
		if endpoint := t.endpoints[iceEndpointSRTCP]; endpoint != nil {
			handler = endpoint.Push
		}
	}
	if handler == nil {
		if len(t.pendingPackets) < maxPendingTransportPackets {
			t.pendingPackets = append(t.pendingPackets, append([]byte(nil), packet...))
		} else {
			t.log.Warnf("no handler for packet starting with %d; pending queue is full", packet[0])
		}
		t.packetLock.Unlock()

		return
	}
	t.packetLock.Unlock()

	t.handlePacket(handler, packet)
}

func (t *ICETransport) handlePacket(handler func([]byte) error, packet []byte) {
	if err := handler(packet); err != nil && !errors.Is(err, packetio.ErrFull) {
		t.log.Warnf("failed to dispatch WebRTC packet: %v", err)
	}
}

func (t *ICETransport) takePendingPacketsLocked(match func([]byte) bool) [][]byte {
	matched := make([][]byte, 0, len(t.pendingPackets))
	remaining := make([][]byte, 0, len(t.pendingPackets))
	for _, packet := range t.pendingPackets {
		if match(packet) {
			matched = append(matched, packet)
		} else {
			remaining = append(remaining, packet)
		}
	}
	t.pendingPackets = remaining

	return matched
}

func (t *ICETransport) closeEndpoints() {
	t.packetLock.Lock()
	endpoints := t.endpoints
	t.dtlsHandler = nil
	t.endpoints = [2]*netconn.Conn{}
	t.packetLock.Unlock()

	for _, endpoint := range endpoints {
		if endpoint != nil {
			_ = endpoint.Close()
		}
	}
}

func (t *ICETransport) getConn() net.Conn {
	t.lock.RLock()
	defer t.lock.RUnlock()

	return t.conn
}

func (t *ICETransport) write(packet []byte) (int, error) {
	conn := t.getConn()
	if conn == nil {
		return 0, io.ErrClosedPipe
	}

	n, err := conn.Write(packet)
	if errors.Is(err, ice.ErrNoCandidatePairs) {
		return 0, nil
	}
	if errors.Is(err, ice.ErrClosed) {
		return 0, io.ErrClosedPipe
	}

	return n, err
}

func (t *ICETransport) localAddr() net.Addr {
	conn := t.getConn()
	if conn == nil {
		return nil
	}

	return conn.LocalAddr()
}

func (t *ICETransport) remoteAddr() net.Addr {
	conn := t.getConn()
	if conn == nil {
		return nil
	}

	return conn.RemoteAddr()
}

func (t *ICETransport) setWriteDeadline(deadline time.Time) error {
	conn := t.getConn()
	if conn == nil {
		return io.ErrClosedPipe
	}

	return conn.SetWriteDeadline(deadline)
}

func matchDTLS(packet []byte) bool  { return matchRange(20, 63, packet) }
func matchSRTP(packet []byte) bool  { return matchRange(128, 191, packet) && !isRTCP(packet) }
func matchSRTCP(packet []byte) bool { return matchRange(128, 191, packet) && isRTCP(packet) }

func matchRange(lower, upper byte, packet []byte) bool {
	return len(packet) != 0 && packet[0] >= lower && packet[0] <= upper
}

func isRTCP(packet []byte) bool {
	return len(packet) >= 4 && packet[1] >= 192 && packet[1] <= 223
}

func (t *ICETransport) ensureGatherer() error {
	if t.gatherer == nil {
		return errICEGathererNotStarted
	} else if t.gatherer.getAgent() == nil {
		if err := t.gatherer.createAgent(); err != nil {
			return err
		}
	}

	return nil
}

// Stats reports the current statistics of the ICETransport.
func (t *ICETransport) Stats() TransportStats {
	conn := t.getConn()

	stats := TransportStats{
		Timestamp: statsTimestampFrom(time.Now()),
		Type:      StatsTypeTransport,
		ID:        "iceTransport",
	}
	if conn != nil {
		if connWithStats, ok := conn.(interface {
			BytesSent() uint64
			BytesReceived() uint64
		}); ok {
			stats.BytesSent = connWithStats.BytesSent()
			stats.BytesReceived = connWithStats.BytesReceived()
		}
	}

	return stats
}

func (t *ICETransport) collectStats(collector *statsReportCollector) {
	collector.Collecting()
	stats := t.Stats()
	collector.Collect(stats.ID, stats)
}

func (t *ICETransport) haveRemoteCredentialsChange(newUfrag, newPwd string) bool {
	t.lock.Lock()
	defer t.lock.Unlock()

	agent := t.gatherer.getAgent()
	if agent == nil {
		return false
	}

	uFrag, uPwd, err := agent.GetRemoteUserCredentials()
	if err != nil {
		return false
	}

	return uFrag != newUfrag || uPwd != newPwd
}

func (t *ICETransport) setRemoteCredentials(newUfrag, newPwd string) error {
	t.lock.Lock()
	defer t.lock.Unlock()

	agent := t.gatherer.getAgent()
	if agent == nil {
		return fmt.Errorf("%w: unable to SetRemoteCredentials", errICEAgentNotExist)
	}

	return agent.SetRemoteCredentials(newUfrag, newPwd)
}
