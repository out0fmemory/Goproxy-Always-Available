package vps

import (
	"encoding/base64"
	"net/http"
	"net/url"

	"github.com/golang/glog"
	"github.com/phuslu/http2"
)

var (
	reqWriteExcludeHeader = map[string]bool{
		"Vary":                true,
		"Via":                 true,
		"X-Forwarded-For":     true,
		"Proxy-Authorization": true,
		"Proxy-Connection":    true,
		"Upgrade":             true,
		"X-Chrome-Variations": true,
		"Connection":          true,
		"Cache-Control":       true,
	}
)

type FetchServer struct {
	URL       *url.URL
	Username  string
	Password  string
	SSLVerify bool
	Transport *http2.Transport
}

func (f *FetchServer) RoundTrip(req *http.Request) (resp *http.Response, err error) {
	var req1 *http.Request
	var resp1 *http.Response

	req1, err = f.encodeRequest(req)
	if err != nil {
		return nil, err
	}

	resp1, err = f.Transport.RoundTrip(req1)
	if err != nil {
		return nil, err
	}

	resp, err = f.decodeResponse(resp1)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (f *FetchServer) encodeRequest(req *http.Request) (*http.Request, error) {
	for key, shouldDelete := range reqWriteExcludeHeader {
		if shouldDelete && req.Header.Get(key) != "" {
			req.Header.Del(key)
		}
	}

	req.Header.Set("Proxy-Authorization", base64.StdEncoding.EncodeToString([]byte(f.Username+":"+f.Password)))

	return req, nil
}

func (f *FetchServer) decodeResponse(resp *http.Response) (resp1 *http.Response, err error) {
	return resp, nil
}
