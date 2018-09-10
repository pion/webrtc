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

// Unknown defines default public constant to use for "enum" like struct
// comparisons when no value was defined.
const Unknown = iota

func newCandidatePair(local, remote Candidate) CandidatePair {
	return CandidatePair{
		remote: remote,
		local:  local,
	}
}

// CandidatePair represents a combination of a local and remote candidate
type CandidatePair struct {
	// lastUpdateTime ?
	remote Candidate
	local  Candidate
}

// GetAddrs returns network addresses for the candidate pair
func (c CandidatePair) GetAddrs() (local *stun.TransportAddr, remote *net.UDPAddr) {
	return &stun.TransportAddr{
			IP:   net.ParseIP(c.local.GetBase().Address),
			Port: c.local.GetBase().Port,
		}, &net.UDPAddr{
			IP:   net.ParseIP(c.remote.GetBase().Address),
			Port: c.remote.GetBase().Port,
		}
}

// Agent represents the ICE agent
type Agent struct {
	sync.RWMutex

	// iceNotifier      func(ConnectionState)

	tieBreaker      uint64
	connectionState ConnectionState
	gatheringState  GatheringState

	haveStarted   bool
	isControlling bool
	taskLoopChan  chan bool

	LocalUfrag      string
	LocalPwd        string
	LocalCandidates []Candidate

	remoteUfrag      string
	remotePwd        string
	remoteCandidates []Candidate

	selectedPair CandidatePair
	validPairs   []CandidatePair

	transports map[string]*Transport

	Input  chan interface{}
	reader chan interface{}

	Output chan []byte
	writer chan []byte

	OnReceive func([]byte, *net.UDPAddr)
}

const (
	agentTickerBaseInterval = 3 * time.Second
	stunTimeout             = 10 * time.Second
)

// NewAgent creates a new Agent
func NewAgent() (*Agent, error) {
	agent := &Agent{
		// outboundCallback: outboundCallback,
		// iceNotifier:      iceNotifier,

		tieBreaker:      rand.New(rand.NewSource(time.Now().UnixNano())).Uint64(),
		gatheringState:  GatheringStateComplete, // TODO trickle-ice
		connectionState: ConnectionStateNew,

		LocalUfrag: util.RandSeq(16),
		LocalPwd:   util.RandSeq(32),

		Input:  make(chan interface{}, 1),
		reader: make(chan interface{}, 1),
		Output: make(chan []byte, 1),
		writer: make(chan []byte, 1),
	}

	for _, ip := range getLocalInterfaces() {
		transport, err := NewTransport(ip + ":0")
		if err != nil {
			return nil, err
		}
		transport.OnReceive = agent.onReceiveHandler

		agent.transports[transport.Addr.String()] = transport
		agent.LocalCandidates = append(agent.LocalCandidates, &CandidateHost{
			CandidateBase: CandidateBase{
				Protocol: ProtoTypeUDP,
				Address:  transport.Addr.IP.String(),
				Port:     transport.Addr.Port,
			},
		})
	}

	return agent, nil
}

func (a *Agent) onReceiveHandler(t *Transport, packet *Packet) {
	if packet.Buffer[0] < 2 {
		a.handleInbound(packet.Buffer, t.Addr, packet.Addr)
	}

	if a.OnReceive != nil {
		a.OnReceive(packet.Buffer, packet.Addr)
	}
}

func getLocalInterfaces() (IPs []string) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return IPs
	}

	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 {
			continue // interface down
		}
		if iface.Flags&net.FlagLoopback != 0 {
			continue // loopback interface
		}
		addrs, err := iface.Addrs()
		if err != nil {
			return IPs
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() {
				continue
			}
			ip = ip.To4()
			if ip == nil {
				continue // not an ipv4 address
			}
			IPs = append(IPs, ip.String())
		}
	}
	return IPs
}

func (a *Agent) Send(raw []byte, local *stun.TransportAddr, remote *net.UDPAddr) {

}

// Start starts the agent
func (a *Agent) Start(isControlling bool, remoteUfrag, remotePwd string) error {
	a.Lock()
	defer a.Unlock()

	if a.haveStarted {
		return errors.Errorf("Attempted to start agent twice")
	} else if remoteUfrag == "" {
		return errors.Errorf("remoteUfrag is empty")
	} else if remotePwd == "" {
		return errors.Errorf("remotePwd is empty")
	}

	a.isControlling = isControlling
	a.remoteUfrag = remoteUfrag
	a.remotePwd = remotePwd

	go a.agentTaskLoop()
	return nil
}

func (a *Agent) pingCandidate(local, remote Candidate) {
	var msg *stun.Message
	var err error

	if a.isControlling {
		msg, err = stun.Build(stun.ClassRequest, stun.MethodBinding, stun.GenerateTransactionId(),
			&stun.Username{Username: a.remoteUfrag + ":" + a.LocalUfrag},
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
			&stun.Username{Username: a.remoteUfrag + ":" + a.LocalUfrag},
			&stun.UseCandidate{},
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

	a.Send(msg.Pack(), &stun.TransportAddr{
		IP:   net.ParseIP(local.GetBase().Address),
		Port: local.GetBase().Port,
	}, &net.UDPAddr{
		IP:   net.ParseIP(remote.GetBase().Address),
		Port: remote.GetBase().Port,
	})

	// a.outboundCallback(msg.Pack(), &stun.TransportAddr{
	// 	IP:   net.ParseIP(local.GetBase().Address),
	// 	Port: local.GetBase().Port,
	// }, &net.UDPAddr{
	// 	IP:   net.ParseIP(remote.GetBase().Address),
	// 	Port: remote.GetBase().Port,
	// })
}

func (a *Agent) updateConnectionState(newState ConnectionState) {
	a.connectionState = newState
	// Call handler async since we may be holding the agent lock
	// and the handler may also require it
	go a.iceNotifier(a.connectionState)
}

func (a *Agent) setValidPair(local, remote Candidate, selected bool) {
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
}

func (a *Agent) agentTaskLoop() {
	// TODO this should be dynamic, and grow when the connection is stable
	t := time.NewTicker(agentTickerBaseInterval)
	a.updateConnectionState(ConnectionStateChecking)

	assertSelectedPairValid := func() bool {
		if a.selectedPair.remote == nil || a.selectedPair.local == nil {
			return false
		} else if time.Since(a.selectedPair.remote.GetBase().LastSeen) > stunTimeout {
			a.selectedPair.remote = nil
			a.selectedPair.local = nil
			a.updateConnectionState(ConnectionStateDisconnected)
			return false
		}

		return true
	}

	for {
		select {
		case <-t.C:
			a.Lock()
			if assertSelectedPairValid() {
				a.Unlock()
				continue
			}

			for _, localCandidate := range a.LocalCandidates {
				for _, remoteCandidate := range a.remoteCandidates {
					a.pingCandidate(localCandidate, remoteCandidate)
				}
			}

			a.Unlock()
		case <-a.taskLoopChan:
			t.Stop()
			return
		}
	}
}

// AddRemoteCandidate adds a new remote candidate
func (a *Agent) AddRemoteCandidate(c Candidate) {
	a.Lock()
	defer a.Unlock()
	a.remoteCandidates = append(a.remoteCandidates, c)
}

// AddLocalCandidate adds a new local candidate
func (a *Agent) AddLocalCandidate(c Candidate) {
	a.Lock()
	defer a.Unlock()
	a.LocalCandidates = append(a.LocalCandidates, c)
}

// Close cleans up the Agent
func (a *Agent) Close() {
	if a.taskLoopChan != nil {
		close(a.taskLoopChan)
	}
}

func isCandidateMatch(c Candidate, testAddress string, testPort int) bool {
	if c.GetBase().Address == testAddress && c.GetBase().Port == testPort {
		return true
	}

	switch c := c.(type) {
	case *CandidateSrflx:
		if c.RemoteAddress == testAddress && c.RemotePort == testPort {
			return true
		}
	}

	return false
}

func getTransportAddrCandidate(candidates []Candidate, addr *stun.TransportAddr) Candidate {
	for _, c := range candidates {
		if isCandidateMatch(c, addr.IP.String(), addr.Port) {
			return c
		}
	}
	return nil
}

func getUDPAddrCandidate(candidates []Candidate, addr *net.UDPAddr) Candidate {
	for _, c := range candidates {
		if isCandidateMatch(c, addr.IP.String(), addr.Port) {
			return c
		}
	}
	return nil
}

func (a *Agent) sendBindingSuccess(m *stun.Message, local *stun.TransportAddr, remote *net.UDPAddr) {
	if out, err := stun.Build(stun.ClassSuccessResponse, stun.MethodBinding, m.TransactionID,
		&stun.XorMappedAddress{
			XorAddress: stun.XorAddress{
				IP:   remote.IP,
				Port: remote.Port,
			},
		},
		&stun.MessageIntegrity{
			Key: []byte(a.LocalPwd),
		},
		&stun.Fingerprint{},
	); err != nil {
		fmt.Printf("Failed to handle inbound ICE from: %s to: %s error: %s", local.String(), remote.String(), err.Error())
	} else {
		a.Send(out.Pack(), local, remote)
		// a.outboundCallback(out.Pack(), local, remote)
	}

}

func (a *Agent) handleInbound(buf []byte, local *stun.TransportAddr, remote *net.UDPAddr) {
	a.Lock()
	defer a.Unlock()

	localCandidate := getTransportAddrCandidate(a.LocalCandidates, local)
	if localCandidate == nil {
		// TODO debug
		// fmt.Printf("Could not find local candidate for %s:%d ", local.IP.String(), local.Port)
		return
	}

	remoteCandidate := getUDPAddrCandidate(a.remoteCandidates, remote)
	if remoteCandidate == nil {
		// TODO debug
		// fmt.Printf("Could not find remote candidate for %s:%d ", remote.IP.String(), remote.Port)
		return
	}
	remoteCandidate.GetBase().LastSeen = time.Now()

	m, err := stun.NewMessage(buf)
	if err != nil {
		fmt.Println(fmt.Sprintf("Failed to handle decode ICE from: %s to: %s error: %s", local.String(), remote.String(), err.Error()))
		return
	}

	if a.isControlling {
		a.handleInboundControlling(m, local, remote, localCandidate, remoteCandidate)
	} else {
		a.handleInboundControlled(m, local, remote, localCandidate, remoteCandidate)
	}

}

func (a *Agent) handleInboundControlled(m *stun.Message, local *stun.TransportAddr, remote *net.UDPAddr, localCandidate, remoteCandidate Candidate) {
	if _, isControlled := m.GetOneAttribute(stun.AttrIceControlled); isControlled && !a.isControlling {
		fmt.Println("inbound isControlled && a.isControlling == false")
		return
	}

	_, useCandidateFound := m.GetOneAttribute(stun.AttrUseCandidate)
	a.setValidPair(localCandidate, remoteCandidate, useCandidateFound)
	a.sendBindingSuccess(m, local, remote)
}

func (a *Agent) handleInboundControlling(m *stun.Message, local *stun.TransportAddr, remote *net.UDPAddr, localCandidate, remoteCandidate Candidate) {
	if _, isControlling := m.GetOneAttribute(stun.AttrIceControlling); isControlling && a.isControlling {
		fmt.Println("inbound isControlling && a.isControlling == true")
		return
	} else if _, useCandidate := m.GetOneAttribute(stun.AttrUseCandidate); useCandidate && a.isControlling {
		fmt.Println("useCandidate && a.isControlling == true")
		return
	}

	final := m.Class == stun.ClassSuccessResponse && m.Method == stun.MethodBinding
	a.setValidPair(localCandidate, remoteCandidate, final)

	if !final {
		a.sendBindingSuccess(m, local, remote)
	}
}

// SelectedPair gets the current selected pair's Addresses (or returns nil)
func (a *Agent) SelectedPair() (local *stun.TransportAddr, remote *net.UDPAddr) {
	a.RLock()
	defer a.RUnlock()

	if a.selectedPair.remote == nil || a.selectedPair.local == nil {
		for _, p := range a.validPairs {
			return p.GetAddrs()
		}
		return nil, nil
	}

	return a.selectedPair.GetAddrs()
}
