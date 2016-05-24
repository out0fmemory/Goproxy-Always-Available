package helpers

import (
	"math/rand"
)

func ShuffleStrings(slice []string) {
	for i := range slice {
		j := rand.Intn(i + 1)
		slice[i], slice[j] = slice[j], slice[i]
	}
}

func ShuffleInts(slice []int) {
	for i := range slice {
		j := rand.Intn(i + 1)
		slice[i], slice[j] = slice[j], slice[i]
	}
}

func ShuffleUints(slice []uint) {
	for i := range slice {
		j := rand.Intn(i + 1)
		slice[i], slice[j] = slice[j], slice[i]
	}
}

func ShuffleUint16s(slice []uint16) {
	for i := range slice {
		j := rand.Intn(i + 1)
		slice[i], slice[j] = slice[j], slice[i]
	}
}

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
