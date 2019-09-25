// +build !js

package webrtc

import (
	"time"

	"github.com/pion/ice"
	"github.com/pion/logging"
)

// SettingEngine allows influencing behavior in ways that are not
// supported by the WebRTC API. This allows us to support additional
// use-cases without deviating from the WebRTC API elsewhere.
type SettingEngine struct {
	ephemeralUDP struct {
		PortMin uint16
		PortMax uint16
	}
	detach struct {
		DataChannels bool
	}
	timeout struct {
		ICEConnection                *time.Duration
		ICEKeepalive                 *time.Duration
		ICECandidateSelectionTimeout *time.Duration
		ICEHostAcceptanceMinWait     *time.Duration
		ICESrflxAcceptanceMinWait    *time.Duration
		ICEPrflxAcceptanceMinWait    *time.Duration
		ICERelayAcceptanceMinWait    *time.Duration
	}
	candidates struct {
		ICELite            bool
		ICETrickle         bool
		ICENetworkTypes    []NetworkType
		InterfaceFilter    func(string) bool
		NAT1To1IPs         []string
		NAT1To1IPCandidate string
	}
	LoggerFactory logging.LoggerFactory
}

// DetachDataChannels enables detaching data channels. When enabled
// data channels have to be detached in the OnOpen callback using the
// DataChannel.Detach method.
func (e *SettingEngine) DetachDataChannels() {
	e.detach.DataChannels = true
}

// SetConnectionTimeout sets the amount of silence needed on a given candidate pair
// before the ICE agent considers the pair timed out.
func (e *SettingEngine) SetConnectionTimeout(connectionTimeout, keepAlive time.Duration) {
	e.timeout.ICEConnection = &connectionTimeout
	e.timeout.ICEKeepalive = &keepAlive
}

// SetCandidateSelectionTimeout sets the max ICECandidateSelectionTimeout
func (e *SettingEngine) SetCandidateSelectionTimeout(t time.Duration) {
	e.timeout.ICECandidateSelectionTimeout = &t
}

// SetHostAcceptanceMinWait sets the ICEHostAcceptanceMinWait
func (e *SettingEngine) SetHostAcceptanceMinWait(t time.Duration) {
	e.timeout.ICEHostAcceptanceMinWait = &t
}

// SetSrflxAcceptanceMinWait sets the ICESrflxAcceptanceMinWait
func (e *SettingEngine) SetSrflxAcceptanceMinWait(t time.Duration) {
	e.timeout.ICESrflxAcceptanceMinWait = &t
}

// SetPrflxAcceptanceMinWait sets the ICEPrflxAcceptanceMinWait
func (e *SettingEngine) SetPrflxAcceptanceMinWait(t time.Duration) {
	e.timeout.ICEPrflxAcceptanceMinWait = &t
}

// SetRelayAcceptanceMinWait sets the ICERelayAcceptanceMinWait
func (e *SettingEngine) SetRelayAcceptanceMinWait(t time.Duration) {
	e.timeout.ICERelayAcceptanceMinWait = &t
}

// SetEphemeralUDPPortRange limits the pool of ephemeral ports that
// ICE UDP connections can allocate from. This affects both host candidates,
// and the local address of server reflexive candidates.
func (e *SettingEngine) SetEphemeralUDPPortRange(portMin, portMax uint16) error {
	if portMax < portMin {
		return ice.ErrPort
	}

	e.ephemeralUDP.PortMin = portMin
	e.ephemeralUDP.PortMax = portMax
	return nil
}

// SetLite configures whether or not the ice agent should be a lite agent
func (e *SettingEngine) SetLite(lite bool) {
	e.candidates.ICELite = lite
}

// SetTrickle configures whether or not the ice agent should gather candidates
// via the trickle method or synchronously.
func (e *SettingEngine) SetTrickle(trickle bool) {
	e.candidates.ICETrickle = trickle
}

// SetNetworkTypes configures what types of candidate networks are supported
// during local and server reflexive gathering.
func (e *SettingEngine) SetNetworkTypes(candidateTypes []NetworkType) {
	e.candidates.ICENetworkTypes = candidateTypes
}

// SetInterfaceFilter sets the filtering functions when gathering ICE candidates
// This can be used to exclude certain network interfaces from ICE. Which may be
// useful if you know a certain interface will never succeed, or if you wish to reduce
// the amount of information you wish to expose to the remote peer
func (e *SettingEngine) SetInterfaceFilter(filter func(string) bool) {
	e.candidates.InterfaceFilter = filter
}

// SetNAT1To1IPs has a list of external IP addresses of 1:1 (D)NAT.
// This is useful when you are host a server using Pion on an AWS EC2 instance
// which has a private address, behind a 1:1 DNAT with a public IP (e.g.
// Elastic IP). In this case, you can give the public IP address so that
// Pion will use the public IP address in its candidate instead of the private IP
// address.
func (e *SettingEngine) SetNAT1To1IPs(ips []string) {
	e.candidates.NAT1To1IPs = ips
}

// SetNAT1To1IPCandidate is used along with SetNAT1To1IPs, to tell Pion which
// type of candidate should use the given public IP address.
// Two types of candidates are supported:
// - "host": The public IP address will be used for the host candidate in the SDP.
// - "srflx": A server reflexive candidate with the given public IP address will be added
// to the SDP. If you choose "host", then the private IP address won't be advertised with
// the peer. Also, this option cannot be used along with mDNS.
// If you choose "srflx", it simply adds a server reflexive candidate with the public IP.
// The host candidate is still available along with mDNS capabilities unaffected.
// Please note that you cannot give STUN server URL at the same time. It will result in
// an error otherwise.
func (e *SettingEngine) SetNAT1To1IPCandidate(candidate string) {
	e.candidates.NAT1To1IPCandidate = candidate
}
