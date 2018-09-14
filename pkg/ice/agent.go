// Package ice implements the Interactive Connectivity Establishment (ICE)
// protocol defined in rfc5245.
package ice

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pions/pkg/stun"
	"github.com/pions/webrtc/internal/util"
	"github.com/pkg/errors"
)

// Unknown defines default public constant to use for "enum" like struct
// comparisons when no value was defined.
const Unknown = iota

// Agent represents the ICE agent
type Agent struct {
	sync.RWMutex

	tieBreaker      uint64
	connectionState ConnectionState
	gatheringState  GatheringState

	haveStarted   bool
	isControlling bool
	taskLoopStop  chan bool

	LocalUfrag      string
	LocalPwd        string
	LocalCandidates []Candidate

	remoteUfrag      string
	remotePwd        string
	remoteCandidates []Candidate

	selectedPair CandidatePair
	validPairs   []CandidatePair

	transports map[string]*transport

	OnConnectionStateChange func(ConnectionState)
	OnReceive               func(ReceiveEvent)
}

const (
	agentTickerBaseInterval = 3 * time.Second
	stunTimeout             = 10 * time.Second
)

// NewAgent creates a new Agent
func NewAgent(iceServers *[]*URL) (*Agent, error) {
	buf := make([]byte, 8)
	_, err := rand.Read(buf)
	if err != nil {
		return nil, ErrRandomNumber
	}

	agent := &Agent{
		tieBreaker:      binary.LittleEndian.Uint64(buf),
		gatheringState:  GatheringStateComplete, // TODO trickle-ice
		connectionState: ConnectionStateNew,

		taskLoopStop: make(chan bool),

		LocalUfrag: util.RandSeq(16),
		LocalPwd:   util.RandSeq(32),

		transports: make(map[string]*transport),
	}

	if err := agent.gatherHostCandidates(); err != nil {
		return nil, err
	}

	if err := agent.gatherSrvRflxCandidates(iceServers); err != nil {
		return nil, err
	}

	return agent, nil
}

// AddRemoteCandidate adds a new remote candidate
func (a *Agent) AddRemoteCandidate(c Candidate) {
	a.Lock()
	defer a.Unlock()
	a.remoteCandidates = append(a.remoteCandidates, c)
}

// SelectedPair gets the current selected pair's Addresses (or returns nil)
func (a *Agent) SelectedPair() (local *net.UDPAddr, remote *net.UDPAddr) {
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

func (a *Agent) Send(raw []byte, local, remote *net.UDPAddr) error {
	if err := a.transports[local.String()].send(raw, nil, remote); err != nil {
		return err
	}

	return nil
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

	go a.taskLoop()
	return nil
}

// Close cleans up the Agent
func (a *Agent) Close() {
	if a.taskLoopStop != nil {
		close(a.taskLoopStop)
	}

	for _, t := range a.transports {
		t.close()
	}
}

func (a *Agent) taskLoop() {
	// TODO this should be dynamic, and grow when the connection is stable
	ticker := time.NewTicker(agentTickerBaseInterval)
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
		case <-ticker.C:
			a.RLock()
			if assertSelectedPairValid() {
				a.RUnlock()
				continue
			}

			for _, localCandidate := range a.LocalCandidates {
				for _, remoteCandidate := range a.remoteCandidates {
					a.pingCandidate(localCandidate, remoteCandidate)
				}
			}

			a.RUnlock()
		case <-a.taskLoopStop:
			ticker.Stop()
			return
		}
	}
}

func (a *Agent) updateConnectionState(newState ConnectionState) {
	a.connectionState = newState
	// Call handler async since we may be holding the agent lock
	// and the handler may also require it
	if a.OnConnectionStateChange != nil {
		go a.OnConnectionStateChange(a.connectionState)
	}
}

func (a *Agent) pingCandidate(local, remote Candidate) {
	var msg *stun.Message
	var err error

	if a.isControlling {
		msg, err = stun.Build(
			stun.ClassRequest,
			stun.MethodBinding,
			stun.GenerateTransactionId(),
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
		msg, err = stun.Build(
			stun.ClassRequest,
			stun.MethodBinding,
			stun.GenerateTransactionId(),
			&stun.Username{Username: a.remoteUfrag + ":" + a.LocalUfrag},
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

	localAddr := net.UDPAddr{}
	localAddr.IP, localAddr.Zone = splitIPZone(local.GetBase().Address)
	localAddr.Port = local.GetBase().Port

	remoteAddr := net.UDPAddr{}
	remoteAddr.IP, remoteAddr.Zone = splitIPZone(remote.GetBase().Address)
	remoteAddr.Port = remote.GetBase().Port

	a.Send(msg.Pack(), &localAddr, &remoteAddr)
}

func splitIPZone(s string) (ip net.IP, zone string) {
	if i := strings.LastIndex(s, "%"); i > 0 {
		ip, zone = net.ParseIP(s[:i]), s[i+1:]
	} else {
		ip = net.ParseIP(s)
	}
	return
}

func isCandidateMatch(c Candidate, testAddr *net.UDPAddr) bool {
	host, _, _ := net.SplitHostPort(testAddr.String())
	if c.GetBase().Address == host && c.GetBase().Port == testAddr.Port {
		return true
	}

	switch c := c.(type) {
	case *CandidateSrflx:
		if c.RemoteAddress == host && c.RemotePort == testAddr.Port {
			return true
		}
	}

	return false
}

func getUDPAddrCandidate(candidates []Candidate, addr *net.UDPAddr) Candidate {
	for _, c := range candidates {
		if isCandidateMatch(c, addr) {
			return c
		}
	}
	return nil
}

func (a *Agent) sendBindingSuccess(m *stun.Message, local, remote *net.UDPAddr) {
	if out, err := stun.Build(
		stun.ClassSuccessResponse,
		stun.MethodBinding,
		m.TransactionID,
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
	}

}

func (a *Agent) handleInbound(buf []byte, local, remote *net.UDPAddr) {
	a.Lock()
	defer a.Unlock()

	localCandidate := getUDPAddrCandidate(a.LocalCandidates, local)
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

func (a *Agent) handleInboundControlling(m *stun.Message, local, remote *net.UDPAddr, localCandidate, remoteCandidate Candidate) {
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

func (a *Agent) handleInboundControlled(m *stun.Message, local, remote *net.UDPAddr, localCandidate, remoteCandidate Candidate) {
	if _, isControlled := m.GetOneAttribute(stun.AttrIceControlled); isControlled && !a.isControlling {
		fmt.Println("inbound isControlled && a.isControlling == false")
		return
	}

	a.setValidPair(localCandidate, remoteCandidate, false)
	a.sendBindingSuccess(m, local, remote)
}

func (a *Agent) setValidPair(local, remote Candidate, selected bool) {
	pair := newCandidatePair(local, remote)

	if selected {
		a.selectedPair = pair
		a.validPairs = nil
		// TODO: only set state to connected on selecting final pair?
		a.updateConnectionState(ConnectionStateConnected)
	} else {
		// keep track of pairs with succesfull bindings since any of them
		// can be used for communication until the final pair is selected:
		// https://tools.ietf.org/html/draft-ietf-ice-rfc5245bis-20#section-12
		a.validPairs = append(a.validPairs, pair)
	}
}

func (a *Agent) gatherHostCandidates() error {
	for _, ip := range getLocalInterfaces() {
		transport, err := newTransport(net.JoinHostPort(ip, "0"))
		if err != nil {
			return err
		}
		transport.onReceive = a.onReceiveHandler

		a.transports[transport.addr.String()] = transport
		a.addLocalCandidate(&CandidateHost{
			CandidateBase: CandidateBase{
				Protocol: ProtoTypeUDP,
				Address:  transport.host(),
				Port:     transport.port(),
			},
		})
	}
	return nil
}

// addLocalCandidate adds a new local candidate
func (a *Agent) addLocalCandidate(c Candidate) {
	a.Lock()
	defer a.Unlock()
	a.LocalCandidates = append(a.LocalCandidates, c)
}

func (a *Agent) onReceiveHandler(packet *packet) {
	if packet.buffer[0] < 2 {
		a.handleInbound(packet.buffer, packet.transport.addr, packet.addr)
	}

	if a.OnReceive != nil {
		go a.OnReceive(ReceiveEvent{
			Buffer: packet.buffer,
			Local:  packet.transport.addr.String(),
			Remote: packet.addr.String(),
		})
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
			switch addr := addr.(type) {
			case *net.IPNet:
				ip = addr.IP
			case *net.IPAddr:
				ip = addr.IP

			}

			if ip == nil || ip.IsLoopback() {
				continue
			}

			// The conditions of invalidation written below are defined in
			// https://tools.ietf.org/html/rfc8445#section-5.1.1.1
			if ip := ip.To4(); ip != nil {
				IPs = append(IPs, ip.String())
				continue
			}

			if len(ip) != net.IPv6len ||
				!isZeros(ip[0:12]) || // !(IPv4-compatible IPv6)
				ip[0] == 0xfe && ip[1]&0xc0 == 0xc0 || // !(IPv6 site-local unicast)
				ip.IsLinkLocalUnicast() ||
				ip.IsLinkLocalMulticast() {
				continue
			}

			IPs = append(IPs, ip.String())

		}
	}
	return IPs
}

func isZeros(p net.IP) bool {
	for i := 0; i < len(p); i++ {
		if p[i] != 0 {
			return false
		}
	}
	return true
}

func (a *Agent) gatherSrvRflxCandidates(iceServers *[]*URL) error {
	if iceServers == nil {
		return nil
	}

	for _, url := range *iceServers {
		err := a.addURL(url)
		if err != nil {
			return err
		}
	}
	return nil
}

// AddURL takes an ICE Url, allocates any state and adds the candidate
func (a *Agent) addURL(url *URL) error {
	switch url.Scheme {
	case SchemeTypeSTUN:
		candidate, err := a.getSrflxCandidate(url)
		if err != nil {
			return err
		}

		transport, err := newTransport(
			candidate.RemoteAddress + ":" + strconv.Itoa(candidate.RemotePort),
		)
		if err != nil {
			return err
		}

		host := candidate.GetBase().Address
		port := strconv.Itoa(candidate.GetBase().Port)
		a.transports[net.JoinHostPort(host, port)] = transport
		a.addLocalCandidate(candidate)
	default:
		return errors.Errorf("%s is not implemented", url.Scheme.String())
	}

	return nil
}

func (a *Agent) getSrflxCandidate(url *URL) (*CandidateSrflx, error) {
	// TODO Do we want the timeout to be configurable?
	proto := url.Proto.String()
	client, err := stun.NewClient(proto, fmt.Sprintf("%s:%d", url.Host, url.Port), time.Second*5)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to create STUN client")
	}
	localAddr, ok := client.LocalAddr().(*net.UDPAddr)
	if !ok {
		return nil, errors.Errorf("Failed to cast STUN client to UDPAddr")
	}

	resp, err := client.Request()
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to make STUN request")
	}

	if err = client.Close(); err != nil {
		return nil, errors.Wrapf(err, "Failed to close STUN client")
	}

	attr, ok := resp.GetOneAttribute(stun.AttrXORMappedAddress)
	if !ok {
		return nil, errors.Errorf("Got respond from STUN server that did not contain XORAddress")
	}

	var addr stun.XorAddress
	if err = addr.Unpack(resp, attr); err != nil {
		return nil, errors.Wrapf(err, "Failed to unpack STUN XorAddress response")
	}

	return &CandidateSrflx{
		CandidateBase: CandidateBase{
			Protocol: ProtoTypeUDP,
			Address:  addr.IP.String(),
			Port:     addr.Port,
		},
		RemoteAddress: localAddr.IP.String(),
		RemotePort:    localAddr.Port,
	}, nil
}
