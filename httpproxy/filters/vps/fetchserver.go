package vps

import (
	"bufio"
	"net/http"
	"net/url"
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
	Password  string
	SSLVerify bool
}

func (f *FetchServer) encodeRequest(req *http.Request) (*http.Request, error) {
	req1 := &http.Request{
		Proto:      "HTTP/1.2",
		ProtoMajor: 2,
		ProtoMinor: 0,
		Method:     "POST",
		URL:        req.URL,
		Host:       req.Host,
		Header:     http.Header{},
	}

	for key, values := range req.Header {
		if b, ok := reqWriteExcludeHeader[key]; ok && b {
			continue
		}
		for _, value := range values {
			req1.Header.Add(key, value)
		}
	}

	return req1, nil
}

func (f *FetchServer) decodeResponse(resp *http.Response) (resp1 *http.Response, err error) {
	if resp.StatusCode != 200 {
		return resp, nil
	}

	resp, err = http.ReadResponse(bufio.NewReader(resp.Body), resp.Request)
	return resp, err
}
