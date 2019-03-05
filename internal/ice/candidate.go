package ice

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/pions/stun"
)

const (
	receiveMTU             = 8192
	defaultLocalPreference = 65535

	// ComponentRTP indicates that the candidate is used for RTP
	ComponentRTP uint16 = 1
	// ComponentRTCP indicates that the candidate is used for RTCP
	ComponentRTCP
)

// Candidate represents an ICE candidate
type Candidate struct {
	NetworkType

	Type            CandidateType
	LocalPreference uint16
	Component       uint16
	IP              net.IP
	Port            int
	RelatedAddress  *CandidateRelatedAddress

	lock         sync.RWMutex
	lastSent     time.Time
	lastReceived time.Time

	agent    *Agent
	conn     net.PacketConn
	closeCh  chan struct{}
	closedCh chan struct{}
}

// NewCandidateHost creates a new host candidate
func NewCandidateHost(network string, ip net.IP, port int, component uint16) (*Candidate, error) {
	networkType, err := determineNetworkType(network, ip)
	if err != nil {
		return nil, err
	}

	return &Candidate{
		Type:            CandidateTypeHost,
		NetworkType:     networkType,
		IP:              ip,
		Port:            port,
		LocalPreference: defaultLocalPreference,
		Component:       component,
	}, nil
}

// NewCandidateServerReflexive creates a new server reflective candidate
func NewCandidateServerReflexive(network string, ip net.IP, port int, component uint16, relAddr string, relPort int) (*Candidate, error) {
	networkType, err := determineNetworkType(network, ip)
	if err != nil {
		return nil, err
	}
	return &Candidate{
		Type:            CandidateTypeServerReflexive,
		NetworkType:     networkType,
		IP:              ip,
		Port:            port,
		LocalPreference: defaultLocalPreference,
		Component:       component,
		RelatedAddress: &CandidateRelatedAddress{
			Address: relAddr,
			Port:    relPort,
		},
	}, nil
}

// NewCandidatePeerReflexive creates a new peer reflective candidate
func NewCandidatePeerReflexive(network string, ip net.IP, port int, component uint16, relAddr string, relPort int) (*Candidate, error) {
	networkType, err := determineNetworkType(network, ip)
	if err != nil {
		return nil, err
	}
	return &Candidate{
		Type:            CandidateTypePeerReflexive,
		NetworkType:     networkType,
		IP:              ip,
		Port:            port,
		LocalPreference: defaultLocalPreference,
		Component:       component,
		RelatedAddress: &CandidateRelatedAddress{
			Address: relAddr,
			Port:    relPort,
		},
	}, nil
}

// NewCandidateRelay creates a new relay candidate
func NewCandidateRelay(network string, ip net.IP, port int, component uint16, relAddr string, relPort int) (*Candidate, error) {
	networkType, err := determineNetworkType(network, ip)
	if err != nil {
		return nil, err
	}
	return &Candidate{
		Type:            CandidateTypeRelay,
		NetworkType:     networkType,
		IP:              ip,
		Port:            port,
		LocalPreference: defaultLocalPreference,
		Component:       component,
		RelatedAddress: &CandidateRelatedAddress{
			Address: relAddr,
			Port:    relPort,
		},
	}, nil
}

// start runs the candidate using the provided connection
func (c *Candidate) start(a *Agent, conn net.PacketConn) {
	c.agent = a
	c.conn = conn
	c.closeCh = make(chan struct{})
	c.closedCh = make(chan struct{})

	go c.recvLoop()
}

func (c *Candidate) recvLoop() {
	defer func() {
		close(c.closedCh)
	}()

	buffer := make([]byte, receiveMTU)
	for {
		n, srcAddr, err := c.conn.ReadFrom(buffer)
		if err != nil {
			return
		}

		if stun.IsSTUN(buffer[:n]) {
			m, err := stun.NewMessage(buffer[:n])
			if err != nil {
				iceLog.Warnf("Failed to handle decode ICE from %s to %s: %v", c.addr(), srcAddr, err)
				continue
			}
			err = c.agent.run(func(agent *Agent) {
				agent.handleInbound(m, c, srcAddr)
			})
			if err != nil {
				iceLog.Warnf("Failed to handle message: %v", err)
			}

			continue
		} else {
			err := c.agent.run(func(agent *Agent) {
				agent.noSTUNSeen(c, srcAddr)
			})
			if err != nil {
				iceLog.Warnf("Failed to handle message: %v", err)
			}
		}

		select {
		case bufin := <-c.agent.rcvCh:
			copy(bufin.buf, buffer[:n]) // TODO: avoid copy in common case?
			bufin.size <- n
		case <-c.closeCh:
			return
		}
	}
}

// close stops the recvLoop
func (c *Candidate) close() error {
	c.lock.Lock()
	defer c.lock.Unlock()
	if c.conn != nil {
		// Unblock recvLoop
		close(c.closeCh)
		// Close the conn
		err := c.conn.Close()
		if err != nil {
			return err
		}

		// Wait until the recvLoop is closed
		<-c.closedCh
	}

	return nil
}

func (c *Candidate) writeTo(raw []byte, dst *Candidate) (int, error) {
	n, err := c.conn.WriteTo(raw, dst.addr())
	if err != nil {
		return n, fmt.Errorf("failed to send packet: %v", err)
	}
	c.seen(true)
	return n, nil
}

// Priority computes the priority for this ICE Candidate
func (c *Candidate) Priority() uint16 {
	// The local preference MUST be an integer from 0 (lowest preference) to
	// 65535 (highest preference) inclusive.  When there is only a single IP
	// address, this value SHOULD be set to 65535.  If there are multiple
	// candidates for a particular component for a particular data stream
	// that have the same type, the local preference MUST be unique for each
	// one.
	return (2^24)*c.Type.Preference() +
		(2^8)*c.LocalPreference +
		(2^0)*(256-c.Component)
}

// Equal is used to compare two CandidateBases
func (c *Candidate) Equal(other *Candidate) bool {
	return c.NetworkType == other.NetworkType &&
		c.Type == other.Type &&
		c.IP.Equal(other.IP) &&
		c.Port == other.Port &&
		c.RelatedAddress.Equal(other.RelatedAddress)
}

// String makes the CandidateHost printable
func (c *Candidate) String() string {
	return fmt.Sprintf("%s %s:%d%s", c.Type, c.IP, c.Port, c.RelatedAddress)
}

// LastReceived returns a time.Time indicating the last time
// this candidate was received
func (c *Candidate) LastReceived() time.Time {
	c.lock.RLock()
	defer c.lock.RUnlock()
	return c.lastReceived
}

func (c *Candidate) setLastReceived(t time.Time) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.lastReceived = t
}

// LastSent returns a time.Time indicating the last time
// this candidate was sent
func (c *Candidate) LastSent() time.Time {
	c.lock.RLock()
	defer c.lock.RUnlock()
	return c.lastSent
}

func (c *Candidate) setLastSent(t time.Time) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.lastSent = t
}

func (c *Candidate) seen(outbound bool) {
	if outbound {
		c.setLastSent(time.Now())
	} else {
		c.setLastReceived(time.Now())
	}
}

func (c *Candidate) addr() net.Addr {
	return &net.UDPAddr{
		IP:   c.IP,
		Port: c.Port,
	}
}
