package ice

import (
	"fmt"
	"math/rand"
	"net"
	"sync"
	"time"

	"github.com/pions/pkg/stun"
)

const receiveMTU = 8192

// Candidate represents an ICE candidate
type Candidate struct {
	NetworkType

	Type           CandidateType
	IP             net.IP
	Port           int
	RelatedAddress *CandidateRelatedAddress

	lock         sync.RWMutex
	lastSent     time.Time
	lastReceived time.Time

	agent    *Agent
	conn     net.PacketConn
	closeCh  chan struct{}
	closedCh chan struct{}
}

// NewCandidateHost creates a new host candidate
func NewCandidateHost(network string, ip net.IP, port int) (*Candidate, error) {
	networkType, err := determineNetworkType(network, ip)
	if err != nil {
		return nil, err
	}

	return &Candidate{
		Type:        CandidateTypeHost,
		NetworkType: networkType,
		IP:          ip,
		Port:        port,
	}, nil
}

// NewCandidateServerReflexive creates a new server reflective candidate
func NewCandidateServerReflexive(network string, ip net.IP, port int, relAddr string, relPort int) (*Candidate, error) {
	networkType, err := determineNetworkType(network, ip)
	if err != nil {
		return nil, err
	}
	return &Candidate{
		Type:        CandidateTypeServerReflexive,
		NetworkType: networkType,
		IP:          ip,
		Port:        port,
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
				fmt.Println(fmt.Sprintf("Failed to handle decode ICE from %s to %s: %v", c.addr(), srcAddr, err))
				continue
			}
			err = c.agent.run(func(agent *Agent) {
				agent.handleInbound(m, c, srcAddr)
			})
			if err != nil {
				fmt.Println(fmt.Sprintf("Failed to handle message: %v", err))
			}

			continue
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
func (c *Candidate) Priority(typePreference uint16, component uint16) uint16 {
	localPreference := uint16(rand.New(rand.NewSource(time.Now().UnixNano())).Uint32() / 2)
	return (2^24)*typePreference +
		(2^8)*localPreference +
		(2^0)*(256-component)
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
