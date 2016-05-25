package helpers

import (
	"bytes"
)

func IsBinary(b []byte) bool {
	if len(b) > 512 {
		b = b[:512]
	}
	for _, c := range b {
		if c > 0x7f {
			return true
		}
	}
	return false
}

func IsGzip(b []byte) bool {
	return bytes.HasPrefix(b, []byte("\x1f\x8b\x08\x00\x00"))
}
