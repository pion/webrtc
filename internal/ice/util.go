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
			switch addr := addr.(type) {
			case *net.IPNet:
				ip = addr.IP
			case *net.IPAddr:
				ip = addr.IP

			}

			if ip == nil || ip.IsLoopback() {
				continue
			}

			// The conditions of invalidation written below are defined in
			// https://tools.ietf.org/html/rfc8445#section-5.1.1.1
			if ipv4 := ip.To4(); ipv4 == nil {
				if !isSupportedIPv6(ip) {
					continue
				}
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

func isSupportedIPv6(ip net.IP) bool {
	if len(ip) != net.IPv6len ||
		!isZeros(ip[0:12]) || // !(IPv4-compatible IPv6)
		ip[0] == 0xfe && ip[1]&0xc0 == 0xc0 || // !(IPv6 site-local unicast)
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() {
		return false
	}
	return true
}

func isZeros(ip net.IP) bool {
	for i := 0; i < len(ip); i++ {
		if ip[i] != 0 {
			return false
		}
	}
	return true
}
