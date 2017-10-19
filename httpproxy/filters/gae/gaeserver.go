package gae

import (
	"bufio"
	"bytes"
	"compress/flate"
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/phuslu/glog"

	"../../helpers"
)

type Servers struct {
	curURL    atomic.Value
	muURL     sync.RWMutex
	urls1     []url.URL
	urls2     []url.URL
	password  string
	sslVerify bool
}

func NewServers(urls []url.URL, password string, sslVerify bool) *Servers {
	server := &Servers{
		urls1:     urls,
		urls2:     []url.URL{},
		password:  password,
		sslVerify: sslVerify,
	}
	server.curURL.Store(server.urls1[0])
	return server
}

func (s *Servers) ToggleBadServer(fetchserver url.URL) {
	s.muURL.Lock()
	defer s.muURL.Unlock()
	urls := []url.URL{}
	for _, u := range s.urls1 {
		if u.Host != fetchserver.Host {
			urls = append(urls, u)
		}
	}
	s.urls1 = urls
	s.urls2 = append(s.urls2, fetchserver)
	if len(s.urls1) == 0 {
		s.urls1, s.urls2 = s.urls2, s.urls1
		rand.Shuffle(len(s.urls1), func(i int, j int) {
			s.urls1[i], s.urls1[j] = s.urls1[j], s.urls1[i]
		})
	}
	s.curURL.Store(s.urls1[0])
}

func (s *Servers) EncodeRequest(req *http.Request, fetchserver url.URL, deadline time.Duration, brotli bool) (*http.Request, error) {
	var err error
	var b bytes.Buffer

	helpers.FixRequestURL(req)

	w, err := flate.NewWriter(&b, flate.BestCompression)
	if err != nil {
		return nil, err
	}

	options := ""
	if deadline > 0 {
		options = fmt.Sprintf("deadline=%d", deadline/time.Second)
	}
	if brotli {
		options += ",brotli"
	}
	if s.password != "" {
		options += ",password=" + s.password
	}
	if s.sslVerify {
		options += ",sslverify"
	}

	fmt.Fprintf(w, "%s %s HTTP/1.1\r\n", req.Method, req.URL.String())
	fmt.Fprintf(w, "X-Urlfetch-Options: %s\r\n", options)
	req.Header.WriteSubset(w, helpers.ReqWriteExcludeHeader)
	w.Close()

	b0 := make([]byte, 2)
	binary.BigEndian.PutUint16(b0, uint16(b.Len()))

	req1 := &http.Request{
		Method: http.MethodPost,
		URL:    &fetchserver,
		Host:   fetchserver.Host,
		Header: http.Header{
			"User-Agent": []string{""},
		},
	}

	if req.ContentLength > 0 {
		req1.ContentLength = int64(len(b0)+b.Len()) + req.ContentLength
		req1.Body = helpers.NewMultiReadCloser(bytes.NewReader(b0), &b, req.Body)
	} else {
		req1.ContentLength = int64(len(b0) + b.Len())
		req1.Body = helpers.NewMultiReadCloser(bytes.NewReader(b0), &b)
	}

	return req1, nil
}

func (s *Servers) DecodeResponse(resp *http.Response) (resp1 *http.Response, err error) {
	if resp.StatusCode != http.StatusOK {
		return resp, nil
	}

	var hdrLen uint16
	if err = binary.Read(resp.Body, binary.BigEndian, &hdrLen); err != nil {
		return
	}

	hdrBuf := make([]byte, hdrLen)
	if _, err = io.ReadFull(resp.Body, hdrBuf); err != nil {
		return
	}

	resp1, err = http.ReadResponse(bufio.NewReader(flate.NewReader(bytes.NewReader(hdrBuf))), resp.Request)
	if err != nil {
		return
	}

	const cookieKey string = "Set-Cookie"
	if cookies, ok := resp1.Header[cookieKey]; ok && len(cookies) == 1 {
		parts := strings.Split(cookies[0], ", ")

		parts1 := make([]string, 0)
		for i := 0; i < len(parts); i++ {
			c := parts[i]
			if i == 0 || strings.Contains(strings.Split(c, ";")[0], "=") {
				parts1 = append(parts1, c)
			} else {
				parts1[len(parts1)-1] = parts1[len(parts1)-1] + ", " + c
			}
		}

		if len(parts1) > 1 {
			glog.Warningf("FetchServer(%+v) is not a goproxy GAE server, please upgrade!", resp.Request.Host)
			resp1.Header.Del(cookieKey)
			for i := 0; i < len(parts1); i++ {
				resp1.Header.Add(cookieKey, parts1[i])
			}
		}
	}

	if resp1.StatusCode >= http.StatusBadRequest {
		switch {
		case resp.Body == nil:
			break
		case resp1.Body == nil:
			resp1.Body = resp.Body
		default:
			b, _ := ioutil.ReadAll(resp1.Body)
			if b != nil && len(b) > 0 {
				resp1.Body = helpers.NewMultiReadCloser(bytes.NewReader(b), resp.Body)
			} else {
				resp1.Body = resp.Body
			}
		}
	} else {
		resp1.Body = resp.Body
	}

	return
}

func (s *Servers) PickFetchServer(req *http.Request, base int) url.URL {
	perfer := !helpers.IsStaticRequest(req)

	if base > 0 {
		perfer = false
	}

	if req.Method == http.MethodPost {
		perfer = true
	}

	if perfer {
		return s.curURL.Load().(url.URL)
	} else {
		s.muURL.RLock()
		defer s.muURL.RUnlock()
		return s.urls1[rand.Intn(len(s.urls1))]
	}
}
