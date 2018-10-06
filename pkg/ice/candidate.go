package ice

import (
	"fmt"
	"math/rand"
	"net"
	"time"

	"golang.org/x/net/ipv4"
)

// Preference enums when generate Priority
const (
	HostCandidatePreference  uint16 = 126
	SrflxCandidatePreference uint16 = 100
)

// Candidate represents an ICE candidate
type Candidate interface {
	GetBase() *CandidateBase
	String() string
}

// CandidateBase represents an ICE candidate, a base with enough attributes
// for host candidates, see CandidateSrflx and CandidateRelay for more
type CandidateBase struct {
	Protocol     ProtoType
	Address      string
	Port         int
	LastSent     time.Time
	LastReceived time.Time
	Conn         *ipv4.PacketConn // TODO: make private
}

func (c *CandidateBase) addr() net.Addr {
	return &net.UDPAddr{
		IP:   net.ParseIP(c.Address),
		Port: c.Port,
	}
}

func (c *CandidateBase) sendTo(raw []byte, dst *CandidateBase) error {
	if _, err := c.Conn.WriteTo(raw, nil, dst.addr()); err != nil {
		return fmt.Errorf("failed to send packet: %v", err)
	}
	c.seen(true)
	return nil
}

func (c *CandidateBase) seen(outbound bool) {
	if outbound {
		c.LastSent = time.Now()
	} else {
		c.LastReceived = time.Now()
	}
}

// Priority computes the priority for this ICE Candidate
func (c *CandidateBase) Priority(typePreference uint16, component uint16) uint16 {
	localPreference := uint16(rand.New(rand.NewSource(time.Now().UnixNano())).Uint32() / 2)
	return (2^24)*typePreference +
		(2^8)*localPreference +
		(2^0)*(256-component)
}

// CandidateHost is a Candidate of typ Host
type CandidateHost struct {
	CandidateBase
}

// GetBase returns the CandidateBase, attributes shared between all Candidates
func (c *CandidateHost) GetBase() *CandidateBase {
	return &c.CandidateBase
}

// Address for CandidateHost
func (c *CandidateHost) Address() string {
	return c.CandidateBase.Address
}

// Port for CandidateHost
func (c *CandidateHost) Port() int {
	return c.CandidateBase.Port
}

// String makes the CandidateHost printable
func (c *CandidateHost) String() string {
	return fmt.Sprintf("%s:%d", c.CandidateBase.Address, c.CandidateBase.Port)
}

// CandidateSrflx is a Candidate of typ Server-Reflexive
type CandidateSrflx struct {
	CandidateBase
	RemoteAddress string
	RemotePort    int
}

// GetBase returns the CandidateBase, attributes shared between all Candidates
func (c *CandidateSrflx) GetBase() *CandidateBase {
	return &c.CandidateBase
}

// String makes the CandidateSrflx printable
func (c *CandidateSrflx) String() string {
	return fmt.Sprintf("%s:%d", c.RemoteAddress, c.RemotePort)
}
