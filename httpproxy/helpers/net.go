package helpers

import (
	"net"
	"strings"
)

func LocalIPv4s() ([]net.IP, error) {
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
		if ip := net.ParseIP(addr1); ip != nil && ip.To4() != nil {
			s := ip.String()
			if s == "::1" || strings.HasPrefix(s, "127.") || strings.HasPrefix(s, "169.254.") {
				continue
			}
			ips = append(ips, ip)
		}
	}

	return ips, nil
}

func LocalPerferIPv4() (net.IP, error) {
	addr, err := net.ResolveUDPAddr("udp4", "8.8.8.8:53")
	if err != nil {
		return nil, err
	}

	conn, err := net.DialUDP("udp4", nil, addr)
	if err != nil {
		return nil, err
	}

	return conn.LocalAddr().(*net.UDPAddr).IP, nil
}
