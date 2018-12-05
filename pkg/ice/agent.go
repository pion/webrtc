// Package ice implements the Interactive Connectivity Establishment (ICE)
// protocol defined in rfc5245.
package ice

import (
	"fmt"
	"math/rand"
	"net"
	"sync"
	"time"

	"github.com/pions/pkg/stun"
	"github.com/pions/webrtc/internal/util"
	"github.com/pkg/errors"
)

const (
	// taskLoopInterval is the interval at which the agent performs checks
	taskLoopInterval = 2 * time.Second

	// keepaliveInterval used to keep candidates alive
	keepaliveInterval = 10 * time.Second

	// connectionTimeout used to declare a connection dead
	connectionTimeout = 30 * time.Second
)

// Agent represents the ICE agent
type Agent struct {
	notifier func(ConnectionState)

	// Used to block double Dial/Accept
	opened bool

	// State owned by the taskLoop
	taskChan        chan task
	onConnected     chan struct{}
	onConnectedOnce sync.Once

	connectivityTicker *time.Ticker
	connectivityChan   <-chan time.Time

	tieBreaker      uint64
	connectionState ConnectionState
	gatheringState  GatheringState

	haveStarted   bool
	isControlling bool

	localUfrag      string
	localPwd        string
	localCandidates []Candidate

	remoteUfrag      string
	remotePwd        string
	remoteCandidates map[string]Candidate

	selectedPair *candidatePair
	validPairs   []*candidatePair

	// Channel for reading
	rcvCh chan *bufIn

	// State for closing
	closeOnce sync.Once
	done      chan struct{}
	err       atomicError
}

type bufIn struct {
	buf  []byte
	size chan int
}

func (a *Agent) ok() error {
	select {
	case <-a.done:
		return a.getErr()
	default:
	}
	return nil
}

func (a *Agent) getErr() error {
	err := a.err.Load()
	if err != nil {
		return err
	}
	return ErrClosed
}

// NewAgent creates a new Agent
func NewAgent(urls []*URL, notifier func(ConnectionState)) *Agent {
	a := &Agent{
		notifier:         notifier,
		tieBreaker:       rand.New(rand.NewSource(time.Now().UnixNano())).Uint64(),
		gatheringState:   GatheringStateComplete, // TODO trickle-ice
		connectionState:  ConnectionStateNew,
		remoteCandidates: make(map[string]Candidate),

		localUfrag:  util.RandSeq(16),
		localPwd:    util.RandSeq(32),
		taskChan:    make(chan task),
		onConnected: make(chan struct{}),
		rcvCh:       make(chan *bufIn),
		done:        make(chan struct{}),
	}

	// Initialize local candidates
	a.gatherCandidatesLocal()
	a.gatherCandidatesReflective(urls)

	go a.taskLoop()
	return a
}

func (a *Agent) gatherCandidatesLocal() {
	interfaces := localInterfaces()
	networks := []string{"udp4"}
	for _, network := range networks {
		for _, i := range interfaces {

			laddr, err := net.ResolveUDPAddr(network, i+":0")
			if err != nil {
				fmt.Printf("could not resolve %s %s %v\n", network, i, err)
				continue
			}

			conn, err := net.ListenUDP(network, laddr)
			if err != nil {
				fmt.Printf("could not listen %s %s\n", network, i)
				continue
			}

			c := &CandidateHost{
				CandidateBase: CandidateBase{
					Protocol: ProtoTypeUDP,
					Address:  conn.LocalAddr().(*net.UDPAddr).IP.String(),
					Port:     conn.LocalAddr().(*net.UDPAddr).Port,
					conn:     conn,
				},
			}

			a.localCandidates = append(a.localCandidates, c)

			go a.recvLoop(c)
		}
	}
}

func (a *Agent) gatherCandidatesReflective(urls []*URL) {
	networks := []string{"udp4"}
	for _, network := range networks {
		for _, url := range urls {
			switch url.Scheme {
			case SchemeTypeSTUN:
				laddr, xoraddr, err := allocateUDP(network, url)
				if err != nil {
					fmt.Printf("could not allocate %s %s: %v\n", network, url, err)
					continue
				}
				conn, err := net.ListenUDP(network, laddr)
				if err != nil {
					fmt.Printf("could not listen %s %s: %v\n", network, laddr, err)
				}

				c := &CandidateSrflx{
					CandidateBase: CandidateBase{
						Protocol: ProtoTypeUDP,
						Address:  xoraddr.IP.String(),
						Port:     xoraddr.Port,
						conn:     conn,
					},
					RelatedAddress: laddr.IP.String(),
					RelatedPort:    laddr.Port,
				}

				a.localCandidates = append(a.localCandidates, c)

				go a.recvLoop(c)

			default:
				fmt.Printf("scheme %s is not implemented\n", url.Scheme.String())
				continue
			}
		}
	}
}

func allocateUDP(network string, url *URL) (*net.UDPAddr, *stun.XorAddress, error) {
	// TODO Do we want the timeout to be configurable?
	client, err := stun.NewClient(network, fmt.Sprintf("%s:%d", url.Host, url.Port), time.Second*5)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "Failed to create STUN client")
	}
	localAddr, ok := client.LocalAddr().(*net.UDPAddr)
	if !ok {
		return nil, nil, errors.Errorf("Failed to cast STUN client to UDPAddr")
	}

	resp, err := client.Request()
	if err != nil {
		return nil, nil, errors.Wrapf(err, "Failed to make STUN request")
	}

	if err = client.Close(); err != nil {
		return nil, nil, errors.Wrapf(err, "Failed to close STUN client")
	}

	attr, ok := resp.GetOneAttribute(stun.AttrXORMappedAddress)
	if !ok {
		return nil, nil, errors.Errorf("Got respond from STUN server that did not contain XORAddress")
	}

	var addr stun.XorAddress
	if err = addr.Unpack(resp, attr); err != nil {
		return nil, nil, errors.Wrapf(err, "Failed to unpack STUN XorAddress response")
	}

	return localAddr, &addr, nil
}

func (a *Agent) startConnectivityChecks(isControlling bool, remoteUfrag, remotePwd string) error {
	if a.haveStarted {
		return errors.Errorf("Attempted to start agent twice")
	} else if remoteUfrag == "" {
		return errors.Errorf("remoteUfrag is empty")
	} else if remotePwd == "" {
		return errors.Errorf("remotePwd is empty")
	}

	return a.run(func(agent *Agent) {
		a.isControlling = isControlling
		a.remoteUfrag = remoteUfrag
		a.remotePwd = remotePwd

		// TODO this should be dynamic, and grow when the connection is stable
		t := time.NewTicker(taskLoopInterval)
		a.connectivityTicker = t
		a.connectivityChan = t.C

		agent.updateConnectionState(ConnectionStateChecking)
	})
}

func (a *Agent) pingCandidate(local, remote Candidate) {
	var msg *stun.Message
	var err error

	// The controlling agent MUST include the USE-CANDIDATE attribute in
	// order to nominate a candidate pair (Section 8.1.1).  The controlled
	// agent MUST NOT include the USE-CANDIDATE attribute in a Binding
	// request.

	if a.isControlling {
		msg, err = stun.Build(stun.ClassRequest, stun.MethodBinding, stun.GenerateTransactionId(),
			&stun.Username{Username: a.remoteUfrag + ":" + a.localUfrag},
			&stun.UseCandidate{},
			&stun.IceControlling{TieBreaker: a.tieBreaker},
			&stun.Priority{Priority: uint32(local.GetBase().Priority(HostCandidatePreference, 1))},
			&stun.MessageIntegrity{
				Key: []byte(a.remotePwd),
			},
			&stun.Fingerprint{},
		)
	} else {
		msg, err = stun.Build(stun.ClassRequest, stun.MethodBinding, stun.GenerateTransactionId(),
			&stun.Username{Username: a.remoteUfrag + ":" + a.localUfrag},
			&stun.IceControlled{TieBreaker: a.tieBreaker},
			&stun.Priority{Priority: uint32(local.GetBase().Priority(HostCandidatePreference, 1))},
			&stun.MessageIntegrity{
				Key: []byte(a.remotePwd),
			},
			&stun.Fingerprint{},
		)
	}

	if err != nil {
		fmt.Println(err)
		return
	}

	a.sendSTUN(msg, local, remote)
}

func (a *Agent) updateConnectionState(newState ConnectionState) {
	if a.connectionState != newState {
		a.connectionState = newState
		if a.notifier != nil {
			// Call handler async since we may be holding the agent lock
			// and the handler may also require it
			go a.notifier(a.connectionState)
		}
	}
}

func (a *Agent) setValidPair(local, remote Candidate, selected bool) {
	// TODO: avoid duplicates
	p := newCandidatePair(local, remote)

	if selected {
		a.selectedPair = p
		a.validPairs = nil
		// TODO: only set state to connected on selecting final pair?
		a.updateConnectionState(ConnectionStateConnected)
	} else {
		// keep track of pairs with succesfull bindings since any of them
		// can be used for communication until the final pair is selected:
		// https://tools.ietf.org/html/draft-ietf-ice-rfc5245bis-20#section-12
		a.validPairs = append(a.validPairs, p)
	}

	// Signal connected
	a.onConnectedOnce.Do(func() { close(a.onConnected) })
}

// A task is a
type task func(*Agent)

func (a *Agent) run(t task) error {
	err := a.ok()
	if err != nil {
		return err
	}

	select {
	case <-a.done:
		return a.getErr()
	case a.taskChan <- t:
	}
	return nil
}

func (a *Agent) taskLoop() {
	for {
		select {
		case <-a.connectivityChan:
			if a.validateSelectedPair() {
				a.checkKeepalive()
			} else {
				a.pingAllCandidates()
			}

		case t := <-a.taskChan:
			// Run the task
			t(a)

		case <-a.done:

		}
	}
}

func (a *Agent) recvLoop(c Candidate) {
	base := c.GetBase()
	buffer := make([]byte, receiveMTU)
	for {
		n, srcAddr, err := base.conn.ReadFrom(buffer)
		if err != nil {
			// TODO: handle connection close?
			break
		}

		if len(buffer) == 0 {
			fmt.Println("handleIncoming: inbound buffer is not long enough to demux")
			continue
		}

		if stun.IsSTUN(buffer) {
			m, err := stun.NewMessage(buffer[:n])
			if err != nil {
				fmt.Println(fmt.Sprintf("Failed to handle decode ICE from %s to %s: %v", base.addr(), srcAddr, err))
				continue
			}

			err = a.run(func(agent *Agent) {
				agent.handleInbound(m, c, srcAddr)
			})
			if err != nil {
				fmt.Println(fmt.Sprintf("Failed to handle message: %v", err))
			}

			continue
		}

		bufin := <-a.rcvCh
		copy(bufin.buf, buffer[:n]) // TODO: avoid copy in common case?
		bufin.size <- n
	}
}

// validateSelectedPair checks if the selected pair is (still) valid
// Note: the caller should hold the agent lock.
func (a *Agent) validateSelectedPair() bool {
	if a.selectedPair == nil {
		// Not valid since not selected
		return false
	}

	if time.Since(a.selectedPair.remote.GetBase().LastReceived) > connectionTimeout {
		a.selectedPair = nil
		a.updateConnectionState(ConnectionStateDisconnected)
		return false
	}

	return true
}

// checkKeepalive sends STUN Binding Indications to the selected pair
// if no packet has been sent on that pair in the last keepaliveInterval
// Note: the caller should hold the agent lock.
func (a *Agent) checkKeepalive() {
	if a.selectedPair == nil {
		return
	}

	if time.Since(a.selectedPair.remote.GetBase().LastSent) > keepaliveInterval {
		a.keepaliveCandidate(a.selectedPair.local, a.selectedPair.remote)
	}
}

// pingAllCandidates sends STUN Binding Requests to all candidates
// Note: the caller should hold the agent lock.
func (a *Agent) pingAllCandidates() {
	for _, localCandidate := range a.localCandidates {
		for _, remoteCandidate := range a.remoteCandidates {
			a.pingCandidate(localCandidate, remoteCandidate)
		}
	}
}

// AddRemoteCandidate adds a new remote candidate
func (a *Agent) AddRemoteCandidate(c Candidate) error {
	return a.run(func(agent *Agent) {
		if _, found := agent.remoteCandidates[c.String()]; !found {
			agent.remoteCandidates[c.String()] = c
		}
	})
}

// GetLocalCandidates returns the local candidates
func (a *Agent) GetLocalCandidates() ([]Candidate, error) {
	res := make(chan []Candidate)

	err := a.run(func(agent *Agent) {
		var candidates []Candidate
		candidates = append(candidates, agent.localCandidates...)
		res <- candidates
	})
	if err != nil {
		return nil, err
	}

	return <-res, nil
}

// GetLocalUserCredentials returns the local user credentials
func (a *Agent) GetLocalUserCredentials() (frag string, pwd string) {
	return a.localUfrag, a.localPwd
}

// Close cleans up the Agent
func (a *Agent) Close() error {
	err := a.ok()
	if err != nil {
		return err
	}
	a.err.Store(ErrClosed)
	a.closeOnce.Do(func() { close(a.done) })
	return nil
}

func isCandidateMatch(c Candidate, testAddress string, testPort int) bool {
	if c.GetBase().Address == testAddress && c.GetBase().Port == testPort {
		return true
	}

	switch c := c.(type) {
	case *CandidateSrflx:
		if c.RelatedAddress == testAddress && c.RelatedPort == testPort {
			return true
		}
	}

	return false
}

func getAddrCandidate(candidates map[string]Candidate, addr net.Addr) Candidate {
	var ip string
	var port int

	switch a := addr.(type) {
	case *net.UDPAddr:
		ip = a.IP.String()
		port = a.Port
	default:
		fmt.Printf("unsupported address type %T", a)
		return nil
	}

	for _, c := range candidates {
		if isCandidateMatch(c, ip, port) {
			return c
		}
	}
	return nil
}

func (a *Agent) sendBindingSuccess(m *stun.Message, local, remote Candidate) {
	base := remote.GetBase()
	if out, err := stun.Build(stun.ClassSuccessResponse, stun.MethodBinding, m.TransactionID,
		&stun.XorMappedAddress{
			XorAddress: stun.XorAddress{
				IP:   net.ParseIP(base.Address),
				Port: base.Port,
			},
		},
		&stun.MessageIntegrity{
			Key: []byte(a.localPwd),
		},
		&stun.Fingerprint{},
	); err != nil {
		fmt.Printf("Failed to handle inbound ICE from: %s to: %s error: %s", local.String(), remote.String(), err.Error())
	} else {
		a.sendSTUN(out, local, remote)
	}
}

func (a *Agent) handleInboundControlled(m *stun.Message, localCandidate, remoteCandidate Candidate) {
	if _, isControlled := m.GetOneAttribute(stun.AttrIceControlled); isControlled && !a.isControlling {
		fmt.Println("inbound isControlled && a.isControlling == false")
		return
	}

	successResponse := m.Method == stun.MethodBinding && m.Class == stun.ClassSuccessResponse
	_, usepair := m.GetOneAttribute(stun.AttrUseCandidate)
	// Remember the working pair and select it when marked with usepair
	a.setValidPair(localCandidate, remoteCandidate, usepair)

	if !successResponse {
		// Send success response
		a.sendBindingSuccess(m, localCandidate, remoteCandidate)
	}
}

func (a *Agent) handleInboundControlling(m *stun.Message, localCandidate, remoteCandidate Candidate) {
	if _, isControlling := m.GetOneAttribute(stun.AttrIceControlling); isControlling && a.isControlling {
		fmt.Println("inbound isControlling && a.isControlling == true")
		return
	} else if _, useCandidate := m.GetOneAttribute(stun.AttrUseCandidate); useCandidate && a.isControlling {
		fmt.Println("useCandidate && a.isControlling == true")
		return
	}

	successResponse := m.Method == stun.MethodBinding && m.Class == stun.ClassSuccessResponse
	// Remember the working pair and select it when receiving a success response
	a.setValidPair(localCandidate, remoteCandidate, successResponse)

	if !successResponse {
		// Send success response
		a.sendBindingSuccess(m, localCandidate, remoteCandidate)

		// We received a ping from the controlled agent. We know the pair works so now we ping with use-candidate set:
		a.pingCandidate(localCandidate, remoteCandidate)
	}
}

// handleInbound processes traffic from a remote candidate
func (a *Agent) handleInbound(m *stun.Message, local Candidate, remote net.Addr) {
	remoteCandidate := getAddrCandidate(a.remoteCandidates, remote)
	if remoteCandidate == nil {
		// TODO debug
		// fmt.Printf("Could not find remote candidate for %s:%d ", remote.IP.String(), remote.Port)
		return
	}

	remoteCandidate.GetBase().seen(false)

	if m.Class == stun.ClassIndication {
		return
	}

	if a.isControlling {
		a.handleInboundControlling(m, local, remoteCandidate)
	} else {
		a.handleInboundControlled(m, local, remoteCandidate)
	}

}

func (a *Agent) getBestPair() (*candidatePair, error) {
	res := make(chan *candidatePair)

	err := a.run(func(agent *Agent) {
		if agent.selectedPair != nil {
			res <- agent.selectedPair
			return
		}
		for _, p := range agent.validPairs {
			res <- p
			return
		}
	})

	if err != nil {
		return nil, err
	}

	return <-res, nil
}
