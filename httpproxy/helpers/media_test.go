package helpers

import (
	"testing"
)

func TestIsBinary1(t *testing.T) {
	data := []byte("\xed\xbdyW\x1b")
	if !IsBinary(data) {
		t.Errorf("data=%s must be binary", data)
	}
}

func TestIsBinary2(t *testing.T) {
	data := []byte("hello world!")
	if IsBinary(data) {
		t.Errorf("data=%s must not be binary", data)
	}
}

func TestIsBinary3(t *testing.T) {
	data := []byte("\xef\xbb\xbfhello world!")
	if IsBinary(data) {
		t.Errorf("data=%s must not be binary", data)
	}
}
