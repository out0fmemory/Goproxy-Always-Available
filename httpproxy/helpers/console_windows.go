// +build windows

package helpers

import (
	"os"
	"syscall"
	"unsafe"
)

var (
	kernel32                    = syscall.NewLazyDLL("kernel32.dll")
	procSetConsoleTitleW        = kernel32.NewProc("SetConsoleTitleW")
	procSetConsoleTextAttribute = kernel32.NewProc("SetConsoleTextAttribute")
	hStderr                     = os.Stderr.Fd()
)

func SetConsoleTitle(name string) {
	procSetConsoleTitleW.Call(uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(name))))
}

func setConsoleTextAttribute(attr uint) error {
	_, _, err := procSetConsoleTextAttribute.Call(hStderr, uintptr(attr))
	return err
}

func SetConsoleTextColorRed() error {
	return setConsoleTextAttribute(0x04)
}

func SetConsoleTextColorYellow() error {
	return setConsoleTextAttribute(0x06)
}

func SetConsoleTextColorGreen() error {
	return setConsoleTextAttribute(0x02)
}

func SetConsoleTextColorReset() error {
	return setConsoleTextAttribute(0x07)
}
