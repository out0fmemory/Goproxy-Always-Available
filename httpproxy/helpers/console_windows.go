// +build windows

package helpers

import (
	"syscall"
	"unsafe"
)

var (
	kernel32          = syscall.NewLazyDLL("kernel32.dll")
	pSetConsoleTitleW = kernel32.NewProc("SetConsoleTitleW")
)

func SetConsoleTitle(name string) {
	pSetConsoleTitleW.Call(uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(name))))
}
