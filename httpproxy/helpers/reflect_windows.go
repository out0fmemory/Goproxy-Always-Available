package helpers

import (
	"net"
	"reflect"
	"syscall"
)

func ReflectSysFDFromConn(c net.Conn) (syscall.Handle, error) {
	v := reflect.ValueOf(c)
	netfd := v.Elem().FieldByName("conn").FieldByName("fd").Elem()
	// fd = fe.FieldByName("sysfd").Pointer()
	fd := syscall.Handle(netfd.FieldByName("pfd").FieldByName("Sysfd").Pointer())
	return fd, nil
}
