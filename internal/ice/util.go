package ice

import (
	"net"
	"sync/atomic"
)

func localInterfaces() (ips []net.IP) {
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
			ips = append(ips, ip)
		}
	}
	return ips
}

type atomicError struct{ v atomic.Value }

func (a *atomicError) Store(err error) {
	a.v.Store(struct{ error }{err})
}
func (a *atomicError) Load() error {
	err, _ := a.v.Load().(struct{ error })
	return err.error
}
