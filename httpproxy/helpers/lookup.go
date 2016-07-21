// +build !windows

package helpers

import (
	"io/ioutil"
	"net"
	"regexp"
)

var (
	nsRegex = regexp.MustCompile(`(?m)^nameserver\s+([0-9a-fA-F\.:]+)`)
)

func LookupIP(host string) (ips []net.IP, err error) {
	return net.LookupIP(host)
}

func GetLocalNameServers() ([]string, error) {
	b, err := ioutil.ReadFile("/etc/resolv.conf")
	if err != nil {
		return nil, err
	}
	nameservers := make([]string, 0, 4)
	for _, m := range nsRegex.FindAllStringSubmatch(string(b), -1) {
		nameservers = append(nameservers, m[1])
	}
	return nameservers, nil
}
