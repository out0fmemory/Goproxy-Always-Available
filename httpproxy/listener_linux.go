// +build linux

package httpproxy

import (
	"fmt"
	"syscall"
)

func fcntl(fd int, cmd int, arg int) (val int, errno int) {
	r0, _, e1 := syscall.Syscall(syscall.SYS_FCNTL, uintptr(fd), uintptr(cmd), uintptr(arg))
	val = int(r0)
	errno = int(e1)
	return
}

func noCloseOnExec(fd uintptr) error {
	flag, errno := fcntl(int(fd), syscall.F_GETFD, 0)
	if errno != 0 {
		return fmt.Errorf("fcntl(%s, F_GETFD, 0) errno: %#v", fd, errno)
	}

	_, errno = fcntl(int(fd), syscall.F_SETFD, flag & ^syscall.FD_CLOEXEC)
	if errno != 0 {
		return fmt.Errorf("fcntl(%s, F_SETFD, 0) errno: %#v", fd, errno)
	}

	return nil
}
