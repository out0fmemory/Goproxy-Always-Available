// +build windows

package console

import (
	"syscall"
	"unsafe"
)

var (
	kernel32         = syscall.NewLazyDLL("kernel32.dll")
	SetConsoleTitleW = kernel32.NewProc("SetConsoleTitleW")
)

func SetWindowTitle(name string) {
	SetConsoleTitleW.Call(uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(name))))
}
