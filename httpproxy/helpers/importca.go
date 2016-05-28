//+build !windows,!darwin

package helpers

import (
	"crypto/x509"
)

func ImportCAToSystemRoot(cert *x509.Certificate) error {
	return nil
}
