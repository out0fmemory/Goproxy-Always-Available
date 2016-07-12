// +build !windows

package helpers

import (
	"net"
)

func LookupIP(host string) (ips []net.IP, err error) {
	return net.LookupIP(host)
}
