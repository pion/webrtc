package ice

import "net"

// State is an enum showing the state of a ICE Connection
type ConnectionState int

// List of supported States
const (
	// New ICE agent is gathering addresses
	New = iota + 1

	// Checking ICE agent has been given local and remote candidates, and is attempting to find a match
	Checking

	// Connected ICE agent has a pairing, but is still checking other pairs
	Connected

	// Completed ICE agent has finished
	Completed

	// Failed ICE agent never could sucessfully connect
	Failed

	// Failed ICE agent connected sucessfully, but has entered a failed state
	Disconnected

	// Closed ICE agent has finished and is no longer handling requests
	Closed
)

func (c ConnectionState) String() string {
	switch c {
	case New:
		return "New"
	case Checking:
		return "Checking"
	case Connected:
		return "Connected"
	case Completed:
		return "Completed"
	case Failed:
		return "Failed"
	case Disconnected:
		return "Disconnected"
	case Closed:
		return "Closed"
	default:
		return "Invalid"
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
