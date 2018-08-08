package ice

import "net"

// ConnectionState is an enum showing the state of a ICE Connection
type ConnectionState int

// List of supported States
const (
	// ConnectionStateNew indicates that ICE agent is gathering addresses and that
	// any of the ICE transports are in the "new" state and none of them are in
	// the "checking", "disconnected" or "failed" state, or all ICE transports
	// are in the "closed" state, or there are no transports.
	ConnectionStateNew = iota + 1

	// ConnectionStateChecking indicates that ICE agent has been given local and
	// remote candidates, and is attempting to find a match. Also
	// ConnectionStateChecking indicates that ant of the ICE transports are
	// in the "checking" state and none of them are in the "disconnected" or
	// "failed" state.
	ConnectionStateChecking

	// ConnectionStateConnected indicates that ICE agent has a pairing, but is
	// still checking other pairs. Also ConnectionStateConnected indicates that
	// all ICE transports are in the "connected", "completed" or "closed"
	// state and at least one of them is in the "connected" state.
	ConnectionStateConnected

	// ConnectionStateCompleted indicates that ICE agent has finished and that
	// all ICE transports are in the "completed" or "closed" state and at least
	// one of them is in the "completed" state.
	ConnectionStateCompleted

	// ConnectionStateDisconnected indicates that ICE agent connected
	// successfully, but has entered a failed state and that any of the ICE
	// transports are in the "disconnected" state and none of them are in the
	// "failed" state.
	ConnectionStateDisconnected

	// ConnectionStateFailed indicates that ICE agent never could successfully
	// connect and that any of the ICE transports are in the "failed" state.
	ConnectionStateFailed

	// ConnectionStateClosed indicates that ICE agent has finished and is no
	// longer handling requests and that The RTCPeerConnection struct's IsClosed
	// member variable is true.
	ConnectionStateClosed
)

func (c ConnectionState) String() string {
	switch c {
	case ConnectionStateNew:
		return "New"
	case ConnectionStateChecking:
		return "Checking"
	case ConnectionStateConnected:
		return "Connected"
	case ConnectionStateCompleted:
		return "Completed"
	case ConnectionStateDisconnected:
		return "Disconnected"
	case ConnectionStateFailed:
		return "Failed"
	case ConnectionStateClosed:
		return "Closed"
	default:
		return "Invalid"
	}
}

// GatheringState describes the state of the candidate gathering process
type GatheringState int

const (
	// GatheringStateNew indicates candidate gatering is not yet started and
	// that any of the ICE transports are in the "new" gathering state and
	// none of the transports are in the "gathering" state, or there are no
	// transports.
	GatheringStateNew GatheringState = iota + 1

	// GatheringStateGathering indicates candidate gatering is ongoing and that
	// any of the ICE transports are in the "gathering" state.
	GatheringStateGathering

	// GatheringStateComplete indicates candidate gatering has been completed
	// and that at least one ICE transport exists, and all ICE transports are
	// in the "completed" gathering state.
	GatheringStateComplete
)

func (t GatheringState) String() string {
	switch t {
	case GatheringStateNew:
		return "new"
	case GatheringStateGathering:
		return "gathering"
	case GatheringStateComplete:
		return "complete"
	default:
		return "Unknown"
	}
}

// HostInterfaces generates a slice of all the IPs associated with interfaces
func HostInterfaces() (ips []string) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ips
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
			return ips
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
			ips = append(ips, ip.String())
		}
	}
	return ips
}
