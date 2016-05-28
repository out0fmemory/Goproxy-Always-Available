package helpers

import (
	"crypto/x509"
	"syscall"
	"unsafe"
)

var (
	crypt32                              = syscall.NewLazyDLL("crypt32.dll")
	procCertAddEncodedCertificateToStore = crypt32.NewProc("CertAddEncodedCertificateToStore")
)

func ImportCAToSystemRoot(cert *x509.Certificate) error {
	data := cert.Raw

	handle, err := syscall.CertOpenStore(10, 0, 0, 0x4000|0x20000|0x00000004, uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("root"))))
	if err != nil {
		return err
	}
	defer syscall.CertCloseStore(handle, 0)

	_, _, err = procCertAddEncodedCertificateToStore.Call(uintptr(handle), 1, uintptr(unsafe.Pointer(&data[0])), uintptr(uint(len(data))), 4, 0)
	if err.(syscall.Errno) != 0 {
		return err
	}

	return nil
}
