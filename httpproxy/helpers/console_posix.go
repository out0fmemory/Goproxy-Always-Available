// +build linux darwin freebsd

package helpers

import (
	"os"
)

func SetConsoleTitle(name string) {
	os.Stdout.WriteString("\x1b]2;" + name + "\x07")
}
