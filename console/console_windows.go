// +build windows

package console

import (
	"syscall"
	"unsafe"
)

var (
	kernel32          = syscall.NewLazyDLL("kernel32.dll")
	pSetConsoleTitleW = kernel32.NewProc("SetConsoleTitleW")
)

func SetWindowTitle(name string) {
	pSetConsoleTitleW.Call(uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(name))))
}
