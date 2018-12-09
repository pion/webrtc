package ice

import (
	"fmt"
	"math/rand"
	"net"
	"sync"
	"time"
)

// Preference enums when generate Priority
const (
	HostCandidatePreference  uint16 = 126
	SrflxCandidatePreference uint16 = 100
)

const receiveMTU = 8192

// Candidate represents an ICE candidate
type Candidate interface {
	GetBase() *CandidateBase
	String() string
	Equal(Candidate) bool
}

// CandidateBase represents an ICE candidate, a base with enough attributes
// for host candidates, see CandidateSrflx and CandidateRelay for more
type CandidateBase struct {
	lock sync.RWMutex
	NetworkType
	IP           net.IP
	Port         int
	lastSent     time.Time
	lastReceived time.Time
	conn         net.PacketConn
}

func (c *CandidateBase) addr() net.Addr {
	return &net.UDPAddr{
		IP:   c.IP,
		Port: c.Port,
	}
}

// LastSent returns a time.Time indicating the last time
// this candidate was sent
func (c *CandidateBase) LastSent() time.Time {
	c.lock.RLock()
	defer c.lock.RUnlock()
	return c.lastSent
}

func (c *CandidateBase) setLastSent(t time.Time) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.lastSent = t
}

// LastReceived returns a time.Time indicating the last time
// this candidate was received
func (c *CandidateBase) LastReceived() time.Time {
	c.lock.RLock()
	defer c.lock.RUnlock()
	return c.lastReceived
}

func (c *CandidateBase) setLastReceived(t time.Time) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.lastReceived = t
}

func (c *CandidateBase) writeTo(raw []byte, dst *CandidateBase) (int, error) {
	n, err := c.conn.WriteTo(raw, dst.addr())
	if err != nil {
		return n, fmt.Errorf("failed to send packet: %v", err)
	}
	c.seen(true)
	return n, nil
}

func (c *CandidateBase) seen(outbound bool) {
	if outbound {
		c.setLastSent(time.Now())
	} else {
		c.setLastReceived(time.Now())
	}
}

// Priority computes the priority for this ICE Candidate
func (c *CandidateBase) Priority(typePreference uint16, component uint16) uint16 {
	localPreference := uint16(rand.New(rand.NewSource(time.Now().UnixNano())).Uint32() / 2)
	return (2^24)*typePreference +
		(2^8)*localPreference +
		(2^0)*(256-component)
}

// Equal is used to compare two CandidateBases
func (c *CandidateBase) Equal(other *CandidateBase) bool {
	return c.NetworkType == other.NetworkType &&
		c.IP.Equal(other.IP) &&
		c.Port == other.Port
}

// CandidateHost is a Candidate of typ Host
type CandidateHost struct {
	CandidateBase
}

// GetBase returns the CandidateBase, attributes shared between all Candidates
func (c *CandidateHost) GetBase() *CandidateBase {
	return &c.CandidateBase
}

// Port for CandidateHost
func (c *CandidateHost) Port() int {
	return c.CandidateBase.Port
}

// String makes the CandidateHost printable
func (c *CandidateHost) String() string {
	return fmt.Sprintf("%s:%d", c.CandidateBase.IP, c.CandidateBase.Port)
}

// Equal is used to compare two Candidates
func (c *CandidateHost) Equal(other Candidate) bool {
	switch other.(type) {
	case *CandidateHost:
		return c.GetBase().Equal(other.GetBase())
	default:
		return false
	}
}

// CandidateSrflx is a Candidate of typ Server-Reflexive
type CandidateSrflx struct {
	CandidateBase
	RelatedAddress string
	RelatedPort    int
}

// GetBase returns the CandidateBase, attributes shared between all Candidates
func (c *CandidateSrflx) GetBase() *CandidateBase {
	return &c.CandidateBase
}

// String makes the CandidateSrflx printable
func (c *CandidateSrflx) String() string {
	return fmt.Sprintf("%s:%d", c.RelatedAddress, c.RelatedPort)
}

// Equal is used to compare two Candidates
func (c *CandidateSrflx) Equal(other Candidate) bool {
	switch v := other.(type) {
	case *CandidateSrflx:
		return c.GetBase().Equal(v.GetBase()) &&
			c.RelatedAddress == v.RelatedAddress &&
			c.RelatedPort == v.RelatedPort
	default:
		return false
	}
}
