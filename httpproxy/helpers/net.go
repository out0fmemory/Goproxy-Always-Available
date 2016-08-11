package helpers

import (
	"net"
	"sort"
	"strings"
)

type localips []net.IP

func (r localips) Len() int      { return len(r) }
func (r localips) Swap(i, j int) { r[i], r[j] = r[j], r[i] }
func (r localips) Less(i, j int) bool {
	weight := func(ip net.IP) int {
		s := ip.String()
		switch {
		case ip.To4() == nil:
			return 0
		case strings.HasPrefix(s, "127."):
			return 1
		case strings.HasPrefix(s, "169.254."):
			return 2
		case strings.HasPrefix(s, "192.168.1."):
			return 3
		case strings.HasPrefix(s, "172."):
			return 4
		case strings.HasPrefix(s, "10."):
			return 5
		default:
			return 100
		}
	}
	return weight(r[i]) < weight(r[j])
}

func LocalInterfaceIPs() ([]net.IP, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil, err
	}

	ips := make([]net.IP, 0)
	for _, addr := range addrs {
		addr1 := addr.String()
		switch addr.Network() {
		case "ip+net":
			addr1 = strings.Split(addr1, "/")[0]
		}
		if ip := net.ParseIP(addr1); ip != nil {
			ips = append(ips, ip)
		}
	}

	sort.Sort(localips(ips))

	return ips, nil
}
