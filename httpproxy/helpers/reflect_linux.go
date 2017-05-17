package helpers

import (
	"net"

	"golang.org/x/sys/unix"
)

func GetTCPInfo(c net.Conn) (*unix.TCPInfo, error) {
	fd, err := ReflectSysFDFromConn(c)
	if err != nil {
		return nil, err
	}

	return unix.GetsockoptTCPInfo(fd, unix.SOL_TCP, unix.TCP_INFO)
}
