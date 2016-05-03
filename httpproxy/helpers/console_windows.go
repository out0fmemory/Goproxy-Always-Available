// +build windows

package helpers

import (
	"syscall"
	"unsafe"
)

var (
	kernel32             = syscall.NewLazyDLL("kernel32.dll")
	procSetConsoleTitleW = kernel32.NewProc("SetConsoleTitleW")
)

func SetConsoleTitle(name string) {
	procSetConsoleTitleW.Call(uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(name))))
}
