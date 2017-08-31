package gae

import (
	"bufio"
	"bytes"
	"compress/flate"
	"compress/gzip"
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

	"github.com/dsnet/compress/brotli"

	"../../helpers"
)

const (
	GAEScheme string = "https"
	GAEDomain string = ".appspot.com"
	GAEPath   string = "/_gh/"
)

type Servers struct {
	perferAppid atomic.Value
	muAppID     sync.RWMutex
	appids1     []string
	appids2     []string
	password    string
	sslVerify   bool
}

func NewServers(appids []string, password string, sslVerify bool) *Servers {
	server := &Servers{
		appids1:   appids,
		appids2:   []string{},
		password:  password,
		sslVerify: sslVerify,
	}
	server.perferAppid.Store(server.appids1[0])
	return server
}

func (s *Servers) ToggleBadAppID(appid string) {
	s.muAppID.Lock()
	defer s.muAppID.Unlock()
	appids := make([]string, 0)
	for _, id := range s.appids1 {
		if id != appid {
			appids = append(appids, id)
		}
	}
	s.appids1 = appids
	s.appids2 = append(s.appids2, appid)
	if len(s.appids1) == 0 {
		s.appids1, s.appids2 = s.appids2, s.appids1
		helpers.ShuffleStrings(s.appids1)
	}
	s.perferAppid.Store(s.appids1[0])
}

func (s *Servers) ToggleBadServer(fetchserver *url.URL) {
	s.ToggleBadAppID(strings.TrimSuffix(fetchserver.Host, GAEDomain))
}

func (s *Servers) EncodeRequest(req *http.Request, fetchserver *url.URL, deadline time.Duration, brotli bool) (*http.Request, error) {
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
		URL:    fetchserver,
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

	if resp.Header.Get("Content-Type") == "image/gif" {
		return s.DecodeResponse1(resp)
	} else {
		return s.DecodeResponse2(resp)
	}
}

func (s *Servers) DecodeResponse1(resp *http.Response) (resp1 *http.Response, err error) {
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

func (s *Servers) DecodeResponse2(resp *http.Response) (*http.Response, error) {
	var err error
	var r io.Reader

	switch resp.Header.Get("Content-Encoding") {
	case "br":
		r, err = brotli.NewReader(resp.Body, nil)
	case "gzip":
		r, err = gzip.NewReader(resp.Body)
	default:
		r = resp.Body
	}

	if err != nil {
		return nil, err
	}

	resp1, err := http.ReadResponse(bufio.NewReader(r), resp.Request)
	if err != nil {
		return nil, err
	}

	return resp1, nil
}

func (s *Servers) PickFetchServer(req *http.Request, base int) *url.URL {
	perfer := !helpers.IsStaticRequest(req)

	if base > 0 {
		perfer = false
	}

	if req.Method == http.MethodPost {
		perfer = true
	}

	if perfer {
		return toServer(s.perferAppid.Load().(string))
	} else {
		s.muAppID.RLock()
		defer s.muAppID.RUnlock()
		return toServer(s.appids1[rand.Intn(len(s.appids1))])
	}
}

func toServer(appid string) *url.URL {
	return &url.URL{
		Scheme: GAEScheme,
		Host:   appid + GAEDomain,
		Path:   GAEPath,
	}
}
