package ice

// GatherOptions provides options relating to the gathering of ICE candidates.
type GatherOptions struct {
	ICEServers      []Server
	ICEGatherPolicy TransportPolicy
}
