// +build !linux

package httpproxy

import (
	"fmt"
)

func noCloseOnExec(fd uintptr) error {
	return fmt.Errorf("Not Implemented")
}
