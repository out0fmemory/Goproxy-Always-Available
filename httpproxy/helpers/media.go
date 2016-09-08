package helpers

import (
	"bytes"
)

func IsBinary(b []byte) bool {
	if len(b) > 64 {
		b = b[:64]
	}
	if bytes.HasPrefix(b, []byte{0xef, 0xbb, 0xbf}) {
		return false
	}
	for _, c := range b {
		if c > 0x7f {
			return true
		}
	}
	return false
}

func IsGzip(b []byte) bool {
	return bytes.HasPrefix(b, []byte{0x1f, 0x8b, 0x08, 0x00, 0x00})
}
