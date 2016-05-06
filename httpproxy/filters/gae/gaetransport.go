package gae

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"path"
	"strings"
	"sync"
	"time"

	"../../dialer"
	"../../helpers"

	"github.com/phuslu/glog"
)

type Transport struct {
	http.RoundTripper
	MultiDialer *dialer.MultiDialer
	Servers     []Server
	muServers   sync.Mutex
	RetryDelay  time.Duration
	RetryTimes  int
}

func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	for i := 0; i < t.RetryTimes; i++ {
		server := t.pickServer(req, i)
		server.Deadline = time.Duration(i+1) * DefaultGAEDeadline

		req1, err := server.encodeRequest(req)
		if err != nil {
			return nil, fmt.Errorf("GAE encodeRequest: %s", err.Error())
		}

		resp, err := t.RoundTripper.RoundTrip(req1)

		if err != nil {

			var isTimeoutError bool
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				isTimeoutError = true
			} else if ne, ok := err.(*net.OpError); ok && ne.Op == "read" {
				isTimeoutError = true
			} else {
				isTimeoutError = false
			}

			if isTimeoutError {
				if t1, ok := t.RoundTripper.(interface {
					CloseIdleConnections()
				}); ok {
					glog.Warningf("GAE: request \"%s\" timeout: %v, %T.CloseIdleConnections()", req.URL.String(), err, t1)
					go func() {
						defer func() { recover() }()
						t1.CloseIdleConnections()
					}()
				}
			}

			if i == t.RetryTimes-1 {
				return nil, err
			} else {
				glog.Warningf("GAE: request \"%s\" error: %T(%v), continue...", req.URL.String(), err, err)
				continue
			}
		}

		if resp.StatusCode != http.StatusOK {
			if i == t.RetryTimes-1 {
				return resp, nil
			}

			switch resp.StatusCode {
			case http.StatusServiceUnavailable:
				if len(t.Servers) == 1 {
					glog.Warningf("GAE: %s over qouta, please add more appids to gae.user.json", server.URL.Host)
				} else {
					glog.Warningf("GAE: %s over qouta, switch to next appid...", server.URL.Host)
					t.roundServers()
				}
				time.Sleep(t.RetryDelay)
				continue
			case http.StatusBadGateway, http.StatusNotFound:
				if t.MultiDialer != nil {
					if addr, err := helpers.ReflectRemoteAddrFromResponse(resp); err == nil {
						if ip, _, err := net.SplitHostPort(addr); err == nil {
							glog.Warningf("GAE: %s StatusCode is %d, does not looks like a gws/gvs ip, add to blacklist for 2 hours", ip, resp.StatusCode)
							t.MultiDialer.IPBlackList.Set(ip, struct{}{}, time.Now().Add(2*time.Hour))
						}
					}
				}
				continue
			default:
				return resp, nil
			}
		}

		resp1, err := server.decodeResponse(resp)
		if err != nil {
			return nil, err
		}
		if resp1 != nil {
			resp1.Request = req
		}
		if i == t.RetryTimes-1 {
			return resp, err
		}

		switch resp1.StatusCode {
		case http.StatusBadGateway:
			body, err := ioutil.ReadAll(resp1.Body)
			if err != nil {
				resp1.Body.Close()
				return nil, err
			}
			resp1.Body.Close()
			switch {
			case bytes.Contains(body, []byte("DEADLINE_EXCEEDED")):
				glog.Warningf("GAE: %s urlfetch %#v get DEADLINE_EXCEEDED, continue...", req1.URL.Host, req.URL.String())
				continue
			default:
				resp1.Body = ioutil.NopCloser(bytes.NewReader(body))
				return resp1, nil
			}
		default:
			return resp1, nil
		}
	}

	return nil, fmt.Errorf("GAE: cannot reach here with %#v", req)
}

func (t *Transport) roundServers() {
	server := t.Servers[0]
	t.muServers.Lock()
	if server == t.Servers[0] {
		for i := 0; i < len(t.Servers)-1; i++ {
			t.Servers[i] = t.Servers[i+1]
		}
		t.Servers[len(t.Servers)-1] = server
	}
	t.muServers.Unlock()
}

func (t *Transport) pickServer(req *http.Request, i int) Server {
	n := 0

	if i > 0 && len(t.Servers) > 1 {
		n = 1 + rand.Intn(len(t.Servers)-1)
	} else {
		switch path.Ext(req.URL.Path) {
		case ".jpg", ".png", ".webp", ".bmp", ".gif", ".flv", ".mp4":
			n = rand.Intn(len(t.Servers))
		case "":
			name := path.Base(req.URL.Path)
			if strings.Contains(name, "play") ||
				strings.Contains(name, "video") {
				n = rand.Intn(len(t.Servers))
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
				n = rand.Intn(len(t.Servers))
			}
		}
	}

	return t.Servers[n]
}
