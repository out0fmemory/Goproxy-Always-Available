package helpers

import (
	"net/http"
	"testing"
)

func TestReflectRemoteAddrFromResponseHTTP(t *testing.T) {
	u := "http://www.google.cn"
	resp, err := http.Get(u)
	if err != nil {
		t.Errorf("http.Get(%#v) error: %v", u, err)
	}

	addr, err := reflectRemoteAddrFromResponse(resp)
	if err != nil {
		t.Errorf("reflectRemoteAddrFromResponse(%T) error: %v", resp, err)
	}

	t.Logf("u=%#v, addr=%#v", u, addr)
}

func TestReflectRemoteAddrFromResponseHTTPS(t *testing.T) {
	u := "https://www.bing.com"
	resp, err := http.Get(u)
	if err != nil {
		t.Errorf("http.Get(%#v) error: %v", u, err)
	}

	addr, err := reflectRemoteAddrFromResponse(resp)
	if err != nil {
		t.Errorf("reflectRemoteAddrFromResponse(%T) error: %v", resp, err)
	}

	t.Logf("u=%#v, addr=%#v", u, addr)
}
