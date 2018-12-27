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

	portmin uint16
	portmax uint16

	localUfrag      string
	localPwd        string
	localCandidates map[NetworkType][]*Candidate

	remoteUfrag      string
	remotePwd        string
	remoteCandidates map[NetworkType][]*Candidate

	selectedPair *candidatePair
	validPairs   []*candidatePair

	// Channel for reading
	rcvCh chan *bufIn

	// State for closing
	done chan struct{}
	err  atomicError
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

// AgentConfig collects the arguments to ice.Agent construction into
// a single structure, for future-proofness of the interface
type AgentConfig struct {
	Urls     []*URL
	Notifier func(ConnectionState)

	// PortMin and PortMax are optional. Leave them 0 for the default UDP port allocation strategy.
	PortMin uint16
	PortMax uint16
}

// NewAgent creates a new Agent
func NewAgent(config *AgentConfig) (*Agent, error) {
	if config.PortMax < config.PortMin {
		return nil, ErrPort
	}

	a := &Agent{
		notifier:         config.Notifier,
		tieBreaker:       rand.New(rand.NewSource(time.Now().UnixNano())).Uint64(),
		gatheringState:   GatheringStateComplete, // TODO trickle-ice
		connectionState:  ConnectionStateNew,
		localCandidates:  make(map[NetworkType][]*Candidate),
		remoteCandidates: make(map[NetworkType][]*Candidate),

		localUfrag:  util.RandSeq(16),
		localPwd:    util.RandSeq(32),
		taskChan:    make(chan task),
		onConnected: make(chan struct{}),
		rcvCh:       make(chan *bufIn),
		done:        make(chan struct{}),
		portmin:     config.PortMin,
		portmax:     config.PortMax,
	}

	// Initialize local candidates
	a.gatherCandidatesLocal()
	a.gatherCandidatesReflective(config.Urls)

	go a.taskLoop()
	return a, nil
}

func (a *Agent) listenUDP(network string, laddr *net.UDPAddr) (*net.UDPConn, error) {
	if (laddr.Port != 0) || ((a.portmin == 0) && (a.portmax == 0)) {
		return net.ListenUDP(network, laddr)
	}
	var i, j int
	i = int(a.portmin)
	if i == 0 {
		i = 1
	}
	j = int(a.portmax)
	if j == 0 {
		j = 0xFFFF
	}
	for i <= j {
		c, e := net.ListenUDP(network, &net.UDPAddr{IP: laddr.IP, Port: i})
		if e == nil {
			return c, e
		}
		i++
	}
	return nil, ErrPort
}

func (a *Agent) gatherCandidatesLocal() {
	localIPs := localInterfaces()
	for _, ip := range localIPs {
		for _, network := range supportedNetworks {
			conn, err := a.listenUDP(network, &net.UDPAddr{IP: ip, Port: 0})
			if err != nil {
				fmt.Printf("could not listen %s %s\n", network, ip)
				continue
			}

			port := conn.LocalAddr().(*net.UDPAddr).Port
			c, err := NewCandidateHost(network, ip, port)
			if err != nil {
				fmt.Printf("Failed to create host candidate: %s %s %d: %v\n", network, ip, port, err)
				continue
			}

			networkType := c.NetworkType
			set := a.localCandidates[networkType]
			set = append(set, c)
			a.localCandidates[networkType] = set

			c.start(a, conn)
		}
	}
}

func (a *Agent) gatherCandidatesReflective(urls []*URL) {
	for _, networkType := range supportedNetworkTypes {
		network := networkType.String()
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

				ip := xoraddr.IP
				port := xoraddr.Port
				relIP := laddr.IP.String()
				relPort := laddr.Port
				c, err := NewCandidateServerReflexive(network, ip, port, relIP, relPort)
				if err != nil {
					fmt.Printf("Failed to create server reflexive candidate: %s %s %d: %v\n", network, ip, port, err)
					continue
				}

				networkType := c.NetworkType
				set := a.localCandidates[networkType]
				set = append(set, c)
				a.localCandidates[networkType] = set

				c.start(a, conn)

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
		agent.isControlling = isControlling
		agent.remoteUfrag = remoteUfrag
		agent.remotePwd = remotePwd

		// TODO this should be dynamic, and grow when the connection is stable
		t := time.NewTicker(taskLoopInterval)
		agent.connectivityTicker = t
		agent.connectivityChan = t.C

		agent.updateConnectionState(ConnectionStateChecking)
	})
}

func (a *Agent) pingCandidate(local, remote *Candidate) {
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
			&stun.Priority{Priority: uint32(local.Priority(CandidateTypeHost.Preference(), 1))},
			&stun.MessageIntegrity{
				Key: []byte(a.remotePwd),
			},
			&stun.Fingerprint{},
		)
	} else {
		msg, err = stun.Build(stun.ClassRequest, stun.MethodBinding, stun.GenerateTransactionId(),
			&stun.Username{Username: a.remoteUfrag + ":" + a.localUfrag},
			&stun.IceControlled{TieBreaker: a.tieBreaker},
			&stun.Priority{Priority: uint32(local.Priority(CandidateTypeHost.Preference(), 1))},
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

func (a *Agent) setValidPair(local, remote *Candidate, selected bool) {
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
			return
		}
	}
}

// validateSelectedPair checks if the selected pair is (still) valid
// Note: the caller should hold the agent lock.
func (a *Agent) validateSelectedPair() bool {
	if a.selectedPair == nil {
		// Not valid since not selected
		return false
	}

	if time.Since(a.selectedPair.remote.LastReceived()) > connectionTimeout {
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

	if time.Since(a.selectedPair.remote.LastSent()) > keepaliveInterval {
		a.keepaliveCandidate(a.selectedPair.local, a.selectedPair.remote)
	}
}

// pingAllCandidates sends STUN Binding Requests to all candidates
// Note: the caller should hold the agent lock.
func (a *Agent) pingAllCandidates() {
	for networkType, localCandidates := range a.localCandidates {
		if remoteCandidates, ok := a.remoteCandidates[networkType]; ok {

			for _, localCandidate := range localCandidates {
				for _, remoteCandidate := range remoteCandidates {
					a.pingCandidate(localCandidate, remoteCandidate)
				}
			}

		}
	}
}

// AddRemoteCandidate adds a new remote candidate
func (a *Agent) AddRemoteCandidate(c *Candidate) error {
	return a.run(func(agent *Agent) {
		networkType := c.NetworkType
		set := agent.remoteCandidates[networkType]

		for _, candidate := range set {
			if candidate.Equal(c) {
				return
			}
		}

		set = append(set, c)
		agent.remoteCandidates[networkType] = set
	})
}

// GetLocalCandidates returns the local candidates
func (a *Agent) GetLocalCandidates() ([]*Candidate, error) {
	res := make(chan []*Candidate)

	err := a.run(func(agent *Agent) {
		var candidates []*Candidate
		for _, set := range agent.localCandidates {
			candidates = append(candidates, set...)
		}
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
	done := make(chan struct{})
	err := a.run(func(agent *Agent) {
		defer func() {
			close(done)
		}()
		agent.err.Store(ErrClosed)
		close(agent.done)

		// Cleanup all candidates
		for net, cs := range agent.localCandidates {
			for _, c := range cs {
				err := c.close()
				if err != nil {
					fmt.Printf("Failed to close candidate %s: %v", c, err)
				}
			}
			delete(agent.localCandidates, net)
		}
		for net, cs := range agent.remoteCandidates {
			for _, c := range cs {
				err := c.close()
				if err != nil {
					fmt.Printf("Failed to close candidate %s: %v", c, err)
				}
			}
			delete(agent.remoteCandidates, net)
		}
	})
	if err != nil {
		return err
	}

	<-done

	return nil
}

func (a *Agent) findRemoteCandidate(networkType NetworkType, addr net.Addr) *Candidate {
	var ip net.IP
	var port int

	switch a := addr.(type) {
	case *net.UDPAddr:
		ip = a.IP
		port = a.Port
	case *net.TCPAddr:
		ip = a.IP
		port = a.Port
	default:
		fmt.Printf("unsupported address type %T", a)
		return nil
	}

	set := a.remoteCandidates[networkType]
	for _, c := range set {
		base := c
		if base.IP.Equal(ip) &&
			base.Port == port {
			return c
		}
	}
	return nil
}

func (a *Agent) sendBindingSuccess(m *stun.Message, local, remote *Candidate) {
	base := remote
	if out, err := stun.Build(stun.ClassSuccessResponse, stun.MethodBinding, m.TransactionID,
		&stun.XorMappedAddress{
			XorAddress: stun.XorAddress{
				IP:   base.IP,
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

func (a *Agent) handleInboundControlled(m *stun.Message, localCandidate, remoteCandidate *Candidate) {
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

func (a *Agent) handleInboundControlling(m *stun.Message, localCandidate, remoteCandidate *Candidate) {
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
func (a *Agent) handleInbound(m *stun.Message, local *Candidate, remote net.Addr) {
	remoteCandidate := a.findRemoteCandidate(local.NetworkType, remote)
	if remoteCandidate == nil {
		// TODO debug
		// fmt.Printf("Could not find remote candidate for %s:%d ", remote.IP.String(), remote.Port)
		return
	}

	remoteCandidate.seen(false)

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
		res <- nil
	})

	if err != nil {
		return nil, err
	}

	out := <-res

	if out == nil {
		return nil, errors.New("No Valid Candidate Pairs Available")
	}

	return out, nil
}
