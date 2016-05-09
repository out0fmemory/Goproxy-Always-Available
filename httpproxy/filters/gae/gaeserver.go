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
	"sync"
	"sync/atomic"
	"time"

	"../../helpers"
)

type Server struct {
	URL       *url.URL
	Password  string
	SSLVerify bool
	Deadline  time.Duration
}

func (f *Server) encodeRequest(req *http.Request) (*http.Request, error) {
	var err error
	var b bytes.Buffer

	w, err := flate.NewWriter(&b, flate.BestCompression)
	if err != nil {
		return nil, err
	}

	fmt.Fprintf(w, "%s %s HTTP/1.1\r\n", req.Method, req.URL.String())
	req.Header.WriteSubset(w, helpers.ReqWriteExcludeHeader)
	fmt.Fprintf(w, "X-Urlfetch-Password: %s\r\n", f.Password)
	if f.Deadline > 0 {
		fmt.Fprintf(w, "X-Urlfetch-Deadline: %d\r\n", f.Deadline/time.Second)
	}
	w.Close()

	b0 := make([]byte, 2)
	binary.BigEndian.PutUint16(b0, uint16(b.Len()))

	req1 := &http.Request{
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Method:     "POST",
		URL:        f.URL,
		Host:       f.URL.Host,
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

func (f *Server) decodeResponse(resp *http.Response) (resp1 *http.Response, err error) {
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

type Servers struct {
	servers   []Server
	roundOnce atomic.Value
}

func NewServers(servers []Server) *Servers {
	ss := &Servers{
		servers: servers,
	}
	ss.roundOnce.Store(new(sync.Once))
	return ss
}

func (ss *Servers) Len() int {
	return len(ss.servers)
}

func (ss *Servers) roundServers() {
	ss.roundOnce.Load().(*sync.Once).Do(func() {
		server := ss.servers[0]
		for i := 0; i < len(ss.servers)-1; i++ {
			ss.servers[i] = ss.servers[i+1]
		}
		ss.servers[len(ss.servers)-1] = server
		ss.roundOnce.Store(new(sync.Once))
	})
}

func (ss *Servers) pickServer(req *http.Request, i int) Server {
	n := 0

	if i > 0 && len(ss.servers) > 1 {
		n = 1 + rand.Intn(len(ss.servers)-1)
	} else {
		switch path.Ext(req.URL.Path) {
		case ".jpg", ".png", ".webp", ".bmp", ".gif", ".flv", ".mp4":
			n = rand.Intn(len(ss.servers))
		case "":
			name := path.Base(req.URL.Path)
			if strings.Contains(name, "play") ||
				strings.Contains(name, "video") {
				n = rand.Intn(len(ss.servers))
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
				n = rand.Intn(len(ss.servers))
			}
		}
	}

	return ss.servers[n]
}
