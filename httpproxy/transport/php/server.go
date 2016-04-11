package php

import (
	"bufio"
	"bytes"
	"compress/flate"
	"encoding/binary"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"../../../httpproxy"
	"../../transport"
)

type Server struct {
	URL       *url.URL
	Password  string
	SSLVerify bool
}

func (s *Server) encodeRequest(req *http.Request) (*http.Request, error) {
	var err error
	var b bytes.Buffer

	w, err := flate.NewWriter(&b, flate.BestCompression)
	if err != nil {
		return nil, err
	}

	fmt.Fprintf(w, "%s %s HTTP/1.1\r\n", req.Method, req.URL.String())
	req.Header.WriteSubset(w, transport.ReqWriteExcludeHeader)
	fmt.Fprintf(w, "X-Urlfetch-Password: %s\r\n", s.Password)
	if s.URL.Scheme == "https" {
		io.WriteString(w, "X-Urlfetch-Https: 1\r\n")
	}
	if s.SSLVerify {
		io.WriteString(w, "X-Urlfetch-SSLVerify: 1\r\n")
	}
	io.WriteString(w, "\r\n")
	if err != nil {
		return nil, err
	}
	w.Close()

	b0 := make([]byte, 2)
	binary.BigEndian.PutUint16(b0, uint16(b.Len()))

	req1 := &http.Request{
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Method:     "POST",
		URL:        s.URL,
		Host:       s.URL.Host,
		Header:     http.Header{},
	}

	if req1.URL.Scheme == "https" {
		req1.Header.Set("User-Agent", "a")
	}

	if s.URL.Scheme == "http" {
		for _, key := range []string{"User-Agent", "Accept", "Accept-Encoding", "Accept-Language"} {
			if value := req.Header.Get(key); value != "" {
				req1.Header.Set(key, value)
			}
		}
	}

	if req.ContentLength > 0 {
		req1.ContentLength = int64(len(b0)+b.Len()) + req.ContentLength
		req1.Body = httpproxy.NewMultiReadCloser(bytes.NewReader(b0), &b, req.Body)
	} else {
		req1.ContentLength = int64(len(b0) + b.Len())
		req1.Body = httpproxy.NewMultiReadCloser(bytes.NewReader(b0), &b)
	}

	return req1, nil
}

func (s *Server) decodeResponse(resp *http.Response) (resp1 *http.Response, err error) {
	if resp.StatusCode != 200 {
		return resp, nil
	}

	if s.Password != "" && resp.Header.Get("Content-Type") == "image/gif" && resp.Body != nil {
		resp.Body = transport.NewXorReadCloser(resp.Body, []byte(s.Password))
	}

	resp, err = http.ReadResponse(bufio.NewReader(resp.Body), resp.Request)
	return resp, err
}
