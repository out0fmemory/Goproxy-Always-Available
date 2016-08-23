package main

import (
	"flag"
	"io"
	"net"
	"net/http"
	"os"
	"sort"
	"strings"
)

func SetFlagsIfAbsent(m map[string]string) {
	seen := map[string]struct{}{}

	for i := 1; i < len(os.Args); i++ {
		for key := range m {
			if strings.HasPrefix(os.Args[i], "-"+key+"=") {
				seen[key] = struct{}{}
			}
		}
	}

	for key, value := range m {
		if _, ok := seen[key]; !ok {
			flag.Set(key, value)
		}
	}
}

type FlushWriter struct {
	w io.Writer
}

func (fw FlushWriter) Write(p []byte) (n int, err error) {
	n, err = fw.w.Write(p)
	if f, ok := fw.w.(http.Flusher); ok {
		f.Flush()
	}
	return
}

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
			return 10
		case strings.HasPrefix(s, "169.254."):
			return 20
		case strings.HasPrefix(s, "192.168."):
			return 30
		case strings.HasPrefix(s, "172."):
			return 40
		case strings.HasPrefix(s, "10."):
			return 50
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
