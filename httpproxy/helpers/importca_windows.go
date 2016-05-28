package helpers

import (
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"syscall"
	"unsafe"
)

var (
	crypt32                              = syscall.NewLazyDLL("crypt32.dll")
	procCertAddEncodedCertificateToStore = crypt32.NewProc("CertAddEncodedCertificateToStore")
)

func ImportCAToSystemRoot(name, filename string) error {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}

	block, _ := pem.Decode(data)
	if block.Type != "CERTIFICATE" {
		return fmt.Errorf("\"%s\" type is %v, not CERTIFICATE", filename, block.Type)
	}

	handle, err := syscall.CertOpenStore(10, 0, 0, 0x4000|0x20000|0x00000004, uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("root"))))
	if err != nil {
		return err
	}
	defer syscall.CertCloseStore(handle, 0)

	_, _, _ = procCertAddEncodedCertificateToStore.Call(uintptr(handle), 1, uintptr(unsafe.Pointer(&block.Bytes[0])), uintptr(uint(len(block.Bytes))), 4, 0)

	return err
}
