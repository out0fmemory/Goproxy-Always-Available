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
	"path"
	"strings"
	"time"

	"github.com/cloudflare/golibs/lrucache"
	"github.com/phuslu/glog"

	"../../helpers"
)

const (
	GAEScheme string = "https"
	GAEDomain string = ".appspot.com"
	GAEPath   string = "/_gh/"
)

type Servers struct {
	appids    []string
	badAppIDs lrucache.Cache
	password  string
	sslVerify bool
	deadline  time.Duration
}

func NewServers(appids []string, password string, sslVerify bool, deadline time.Duration) *Servers {
	return &Servers{
		appids:    appids,
		badAppIDs: lrucache.NewLRUCache(uint(len(appids))),
		password:  password,
		sslVerify: sslVerify,
		deadline:  deadline,
	}
}

func (s *Servers) Len() int {
	return len(s.appids)
}

func (s *Servers) ToggleBadAppID(appid string, duration time.Duration) {
	s.badAppIDs.Set(appid, struct{}{}, time.Now().Add(duration))
}

func (s *Servers) IsBadAppID(appid string) bool {
	_, ok := s.badAppIDs.Get(appid)
	return ok
}

func (s *Servers) ToggleBadServer(fetchserver *url.URL, duration time.Duration) {
	appid := strings.TrimSuffix(fetchserver.Host, GAEDomain)
	s.ToggleBadAppID(appid, duration)
}

func (s *Servers) EncodeRequest(req *http.Request, fetchserver *url.URL) (*http.Request, error) {
	var err error
	var b bytes.Buffer

	w, err := flate.NewWriter(&b, flate.BestCompression)
	if err != nil {
		return nil, err
	}

	fmt.Fprintf(w, "%s %s HTTP/1.1\r\n", req.Method, req.URL.String())
	req.Header.WriteSubset(w, helpers.ReqWriteExcludeHeader)
	fmt.Fprintf(w, "X-Urlfetch-Password: %s\r\n", s.password)
	if s.deadline > 0 {
		fmt.Fprintf(w, "X-Urlfetch-Deadline: %d\r\n", s.deadline/time.Second)
	}
	if s.sslVerify {
		fmt.Fprintf(w, "X-Urlfetch-SSLVerify: 1\r\n")
	}
	w.Close()

	b0 := make([]byte, 2)
	binary.BigEndian.PutUint16(b0, uint16(b.Len()))

	req1 := &http.Request{
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Method:     "POST",
		URL:        fetchserver,
		Host:       fetchserver.Host,
		Header:     http.Header{},
	}

	if req1.URL.Scheme == "https" {
		req1.Header.Set("User-Agent", "a")
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

func (s *Servers) PickFetchServer(req *http.Request, base int) *url.URL {
	randChoice := false

	switch {
	case len(s.appids) == 1:
		randChoice = false
	case base > 0:
		randChoice = true
	default:
		switch path.Ext(req.URL.Path) {
		case ".jpg", ".png", ".webp", ".bmp", ".gif", ".flv", ".mp4":
			randChoice = true
		case "":
			name := path.Base(req.URL.Path)
			if strings.Contains(name, "play") ||
				strings.Contains(name, "video") {
				randChoice = true
			}
		default:
			if req.Header.Get("Range") != "" ||
				strings.Contains(req.URL.Host, "img.") ||
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
				randChoice = true
			}
		}
	}

	var appid string
	switch {
	case s.badAppIDs.Len() == 0:
		if !randChoice {
			appid = s.appids[0]
		} else {
			appid = s.appids[rand.Intn(len(s.appids))]
		}
	case s.badAppIDs.Len() == len(s.appids):
		glog.Errorf("GAE: all appids=%v over quota", s.appids)
		appid = s.appids[0]
	case !randChoice:
		for _, id := range s.appids {
			if !s.IsBadAppID(id) {
				appid = id
				break
			}
		}
	default:
		oks := len(s.appids) - s.badAppIDs.Len()
		count := oks
		for _, id := range s.appids {
			if s.IsBadAppID(id) {
				continue
			}
			count -= 1 + rand.Intn(oks)
			if count <= 0 {
				appid = id
				break
			}
		}
	}

	return &url.URL{
		Scheme: GAEScheme,
		Host:   appid + GAEDomain,
		Path:   GAEPath,
	}
}
