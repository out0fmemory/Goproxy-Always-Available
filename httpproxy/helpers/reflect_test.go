package helpers

import (
	"net/http"
	"testing"

	"github.com/phuslu/quic-go/h2quic"
)

func TestReflectRemoteAddrFromResponseHTTP(t *testing.T) {
	u := "http://www.google.cn"
	resp, err := http.Get(u)
	if err != nil {
		t.Errorf("http.Get(%#v) error: %v", u, err)
	}

	addr, err := ReflectRemoteAddrFromResponse(resp)
	if err != nil {
		t.Errorf("reflectRemoteAddrFromResponse(%T) error: %v", resp, err)
	}

	t.Logf("u=%#v, addr=%#v", u, addr)
}

func TestReflectRemoteAddrFromResponseHTTPS(t *testing.T) {
	u := "https://www.bing.com"
	req, _ := http.NewRequest(http.MethodGet, u, nil)
	tr := &http.Transport{
		DisableCompression: true,
	}
	resp, err := tr.RoundTrip(req)
	if err != nil {
		t.Errorf("http.Get(%#v) error: %v", u, err)
	}

	addr, err := ReflectRemoteAddrFromResponse(resp)
	if err != nil {
		t.Errorf("reflectRemoteAddrFromResponse(%T) error: %v", resp, err)
	}

	t.Logf("u=%#v, addr=%#v", u, addr)
}

func TestReflectRemoteAddrFromResponseQuic(t *testing.T) {
	u := "https://www.google.cn"
	req, _ := http.NewRequest(http.MethodGet, u, nil)
	resp, err := (&h2quic.RoundTripper{
		DisableCompression: true,
	}).RoundTrip(req)
	if err != nil {
		t.Errorf("http.Get(%#v) error: %v", u, err)
	}

	addr, err := ReflectRemoteAddrFromResponse(resp)
	if err != nil {
		t.Errorf("reflectRemoteAddrFromResponse(%T) error: %v", resp, err)
	}

	t.Logf("u=%#v, addr=%#v", u, addr)
}
