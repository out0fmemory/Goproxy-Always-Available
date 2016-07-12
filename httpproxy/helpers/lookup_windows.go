// +build windows

package helpers

import (
	"net"
	"os"
	"syscall"
	"unsafe"
)

func lookup(name string, family int32) ([]net.IPAddr, error) {
	hints := syscall.AddrinfoW{
		Family:   family,
		Socktype: syscall.SOCK_STREAM,
		Protocol: syscall.IPPROTO_IP,
	}
	var result *syscall.AddrinfoW
	e := syscall.GetAddrInfoW(syscall.StringToUTF16Ptr(name), nil, &hints, &result)
	if e != nil {
		return nil, &net.DNSError{Err: os.NewSyscallError("getaddrinfow", e).Error(), Name: name}
	}
	defer syscall.FreeAddrInfoW(result)
	addrs := make([]net.IPAddr, 0, 5)
	for ; result != nil; result = result.Next {
		addr := unsafe.Pointer(result.Addr)
		switch result.Family {
		case syscall.AF_INET:
			a := (*syscall.RawSockaddrInet4)(addr).Addr
			addrs = append(addrs, net.IPAddr{IP: net.IPv4(a[0], a[1], a[2], a[3])})
		case syscall.AF_INET6:
			a := (*syscall.RawSockaddrInet6)(addr).Addr
			// FIXME: expose zoneToString ?
			// zone := zoneToString(int((*syscall.RawSockaddrInet6)(addr).Scope_id))
			zone := ""
			addrs = append(addrs, net.IPAddr{IP: net.IP{a[0], a[1], a[2], a[3], a[4], a[5], a[6], a[7], a[8], a[9], a[10], a[11], a[12], a[13], a[14], a[15]}, Zone: zone})
		default:
			return nil, &net.DNSError{Err: syscall.EWINDOWS.Error(), Name: name}
		}
	}
	return addrs, nil
}

func LookupIP(host string) ([]net.IP, error) {
	addrs1 := make([]net.IP, 0, 8)

	var addrs []net.IPAddr
	var err error

	for _, family := range []int{syscall.AF_INET, syscall.AF_INET6} {
		if addrs, err = lookup(host, int32(family)); err == nil {
			for _, addr := range addrs {
				addrs1 = append(addrs1, addr.IP)
			}
		}
	}

	if len(addrs1) > 0 {
		err = nil
	}

	return addrs1, err
}
