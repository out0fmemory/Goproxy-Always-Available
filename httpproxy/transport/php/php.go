package fetchserver

import (
	"bufio"
	"bytes"
	"compress/flate"
	"encoding/binary"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/golang/glog"

	"../../../httpproxy"
	"../../transport"
)

type PHPServer struct {
	URL       *url.URL
	Password  string
	SSLVerify bool
}

type phpTranport struct {
	servers   []PHPServer
	roundTrip func(*http.Request) (*http.Response, error)
}

func (s *PHPServer) encodeRequest(req *http.Request) (*http.Request, error) {
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

func (s *PHPServer) decodeResponse(resp *http.Response) (resp1 *http.Response, err error) {
	if resp.StatusCode != 200 {
		return resp, nil
	}

	if s.Password != "" && resp.Header.Get("Content-Type") == "image/gif" && resp.Body != nil {
		resp.Body = transport.NewXorReadCloser(resp.Body, []byte(s.Password))
	}

	resp, err = http.ReadResponse(bufio.NewReader(resp.Body), resp.Request)
	return resp, err
}

func (t *phpTranport) RoundTrip(req *http.Request) (*http.Response, error) {
	i := 0
	switch path.Ext(req.URL.Path) {
	case ".jpg", ".png", ".webp", ".bmp", ".gif", ".flv", ".mp4":
		i = rand.Intn(len(t.servers))
	case "":
		name := path.Base(req.URL.Path)
		if strings.Contains(name, "play") ||
			strings.Contains(name, "video") {
			i = rand.Intn(len(t.servers))
		}
	default:
		if strings.Contains(req.URL.Host, "img.") ||
			strings.Contains(req.URL.Host, "cache.") ||
			strings.Contains(req.URL.Host, "video.") ||
			strings.Contains(req.URL.Host, "static.") ||
			strings.HasPrefix(req.URL.Host, "img") ||
			strings.HasPrefix(req.URL.Path, "/static") ||
			strings.HasPrefix(req.URL.Path, "/asset") ||
			strings.Contains(req.URL.Path, "min.js") ||
			strings.Contains(req.URL.Path, "static") ||
			strings.Contains(req.URL.Path, "asset") ||
			strings.Contains(req.URL.Path, "/cache/") {
			i = rand.Intn(len(t.servers))
		}
	}

	s := t.servers[i]

	req1, err := s.encodeRequest(req)
	if err != nil {
		return nil, fmt.Errorf("PHP encodeRequest: %s", err.Error())
	}

	res, err := t.roundTrip(req1)
	if err != nil {
		return nil, err
	} else {
		glog.Infof("%s \"PHP %s %s %s\" %d %s", req.RemoteAddr, req.Method, req.URL.String(), req.Proto, res.StatusCode, res.Header.Get("Content-Length"))
	}
	resp, err := s.decodeResponse(res)
	return resp, err
}

func ConfigureTransport(t *http.Transport, servers []PHPServer) {
	// t1 := &phpTranport{
	// 	servers:   servers,
	// 	roundTrip: t.RoundTrip,
	// }

	// t.RoundTrip = t1.RoundTrip
}
