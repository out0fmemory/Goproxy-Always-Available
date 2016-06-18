// +build linux darwin freebsd

package helpers

import (
	"os"
)

var (
	posixConsoleTextColorRed    []byte = []byte("\033[31m")
	posixConsoleTextColorYellow []byte = []byte("\033[33m")
	posixConsoleTextColorGreen  []byte = []byte("\033[32m")
	posixConsoleTextColorReset  []byte = []byte("\033[0m")
)

func SetConsoleTitle(name string) {
	os.Stdout.WriteString("\x1b]2;" + name + "\x07")
}

func SetConsoleTextColorRed() error {
	_, err := os.Stderr.Write(posixConsoleTextColorRed)
	return err
}

func SetConsoleTextColorYellow() error {
	_, err := os.Stderr.Write(posixConsoleTextColorYellow)
	return err
}

func SetConsoleTextColorGreen() error {
	_, err := os.Stderr.Write(posixConsoleTextColorGreen)
	return err
}

func SetConsoleTextColorReset() error {
	_, err := os.Stderr.Write(posixConsoleTextColorReset)
	return err
}
