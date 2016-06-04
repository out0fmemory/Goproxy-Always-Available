package helpers

import (
	"crypto/x509"
	"syscall"
	"unsafe"
)

var (
	crypt32                              = syscall.NewLazyDLL("crypt32.dll")
	procCertAddEncodedCertificateToStore = crypt32.NewProc("CertAddEncodedCertificateToStore")
	procCertDeleteCertificateFromStore   = crypt32.NewProc("CertDeleteCertificateFromStore")
)

func ImportCAToSystemRoot(cert *x509.Certificate) error {
	data := cert.Raw

	store, err := syscall.CertOpenStore(10, 0, 0, 0x4000|0x20000|0x00000004, uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("root"))))
	if err != nil {
		return err
	}
	defer syscall.CertCloseStore(store, 0)

	_, _, err = procCertAddEncodedCertificateToStore.Call(uintptr(store), 1, uintptr(unsafe.Pointer(&data[0])), uintptr(uint(len(data))), 4, 0)
	if err.(syscall.Errno) != 0 {
		return err
	}

	return nil
}

func RemoveCAFromSystemRoot(name string) error {
	store, err := syscall.CertOpenStore(10, 0, 0, 0x4000|0x20000|0x00000004, uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("root"))))
	if err != nil {
		return nil
	}
	defer syscall.CertCloseStore(store, 0)

	certs := make([]*syscall.CertContext, 0)
	var cert *syscall.CertContext
	for {
		cert, err = syscall.CertEnumCertificatesInStore(store, cert)
		if err != nil {
			break
		}

		buf := (*[1 << 20]byte)(unsafe.Pointer(cert.EncodedCert))[:]
		buf2 := make([]byte, cert.Length)
		copy(buf2, buf)

		c, err := x509.ParseCertificate(buf2)
		if err != nil {
			return err
		}

		if c.Subject.CommonName == name ||
			(len(c.Subject.Names) > 0 && c.Subject.Names[0].Value == name) ||
			(len(c.Subject.Organization) > 0 && c.Subject.Organization[0] == name) {
			certs = append(certs, cert)
		}
	}

	for _, cert := range certs {
		_, _, err = procCertDeleteCertificateFromStore.Call(uintptr(unsafe.Pointer(cert)))
	}

	if se, ok := err.(syscall.Errno); ok && se != 0 {
		return err
	}

	return nil
}
