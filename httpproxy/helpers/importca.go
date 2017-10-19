//+build !windows

package helpers

import (
	"crypto/x509"
)

func ImportCAToSystemRoot(cert *x509.Certificate) error {
	return nil
}

func RemoveCAFromSystemRoot(name string) error {
	return nil
}
