package helpers

import (
	"crypto/x509"
	"encoding/pem"
	"io/ioutil"
	"os"
	"os/exec"
)

func ImportCAToSystemRoot(cert *x509.Certificate) error {
	tmpfile, err := ioutil.TempFile("", "goproxy-ca")
	if err != nil {
		return nil
	}
	defer os.Remove(tmpfile.Name())

	if err := pem.Encode(tmpfile, &pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw}); err != nil {
		return err
	}
	if err := tmpfile.Close(); err != nil {
		return err
	}

	cmd := exec.Command("security",
		"add-trusted-cert",
		"-d",
		"-r", "trustRoot",
		"-k", "/Library/Keychains/System.keychain",
		tmpfile.Name())
	if err = cmd.Run(); err != nil {
		return err
	}
	return nil
}

func RemoveCAFromSystemRoot(name string) error {
	return nil
}
