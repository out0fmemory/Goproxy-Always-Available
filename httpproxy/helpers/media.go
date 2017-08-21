package helpers

func IsBinary(b []byte) bool {
	if len(b) > 3 && b[0] == 0xef && b[1] == 0xbb && b[2] == 0xbf {
		// utf-8 text
		return false
	}
	for i, c := range b {
		if c > 0x7f {
			return true
		}
		if c == '\n' && i > 4 {
			break
		}
		if i > 32 {
			break
		}
	}
	return false
}

func IsGzip(b []byte) bool {
	// return *(*uint32)(unsafe.Pointer(&b[0])) == 0x00088b1f
	return len(b) > 4 &&
		b[0] == 0x1f &&
		b[1] == 0x8b &&
		b[2] == 0x08 &&
		b[3] == 0x00
}
