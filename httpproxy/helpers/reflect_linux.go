package helpers

import (
	"net"
	"reflect"

	"golang.org/x/sys/unix"
)

func ReflectSysFDFromConn(c net.Conn) (int, error) {
	v := reflect.ValueOf(c)
	netfd := v.Elem().FieldByName("conn").FieldByName("fd").Elem()
	// fd = int(fe.FieldByName("sysfd").Int())
	fd := int(netfd.FieldByName("pfd").FieldByName("Sysfd").Int())
	return fd, nil
}

func GetTCPInfo(c net.Conn) (*unix.TCPInfo, error) {
	fd, err := ReflectSysFDFromConn(c)
	if err != nil {
		return nil, err
	}

	return unix.GetsockoptTCPInfo(fd, unix.SOL_TCP, unix.TCP_INFO)
}
