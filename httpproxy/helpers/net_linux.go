package helpers

import (
	"net"

	"golang.org/x/sys/unix"
)

type TCPInfo struct {
	unix.TCPInfo
}

func GetTCPInfo(c net.Conn) (*TCPInfo, error) {
	cc, err := c.(*net.TCPConn).SyscallConn()
	if err != nil {
		return nil, err
	}

	var ti *unix.TCPInfo

	fn := func(s uintptr) {
		ti, err = unix.GetsockoptTCPInfo(int(s), unix.SOL_TCP, unix.TCP_INFO)
	}

	if err := cc.Control(fn); err != nil {
		return nil, err
	}

	if err != nil {
		return nil, err
	}

	return &TCPInfo{*ti}, nil
}
