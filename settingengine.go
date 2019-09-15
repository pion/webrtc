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
		ICELite         bool
		ICETrickle      bool
		ICENetworkTypes []NetworkType
		InterfaceFilter func(string) bool
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
