package gae

import (
	"fmt"
	"math/rand"
	"net/http"
	"path"
	"strings"
	"sync"

	"github.com/golang/glog"
)

type Transport struct {
	http.Transport
	servers   []Server
	muServers sync.Mutex
}

func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
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

	server := t.servers[i]

	req1, err := server.encodeRequest(req)
	if err != nil {
		return nil, fmt.Errorf("GAE encodeRequest: %s", err.Error())
	}

	resp, err := t.Transport.RoundTrip(req1)
	if err != nil || resp == nil {
		glog.Errorf("%s \"GAE %s %s %s\" %#v %v", req.RemoteAddr, req.Method, req.URL.String(), req.Proto, resp, err)
		return resp, err
	} else {
		glog.Infof("%s \"GAE %s %s %s\" %d %s", req.RemoteAddr, req.Method, req.URL.String(), req.Proto, resp.StatusCode, resp.Header.Get("Content-Length"))
	}

	switch resp.StatusCode {
	case 503:
		if len(t.servers) == 1 {
			break
		}
		glog.Warningf("%s over qouta, switch to next appid.", server.URL.String())
		t.muServers.Lock()
		if server == t.servers[0] {
			for i := 0; i < len(t.servers)-1; i++ {
				t.servers[i] = t.servers[i+1]
			}
			t.servers[len(t.servers)-1] = server
		}
		t.muServers.Unlock()
		resp := &http.Response{
			Status:     "302 Moved Temporarily",
			StatusCode: 302,
			Proto:      "HTTP/1.1",
			ProtoMajor: 1,
			ProtoMinor: 1,
			Header: http.Header{
				"Location": []string{req.URL.String()},
			},
			Request:       req,
			Close:         false,
			ContentLength: 0,
		}
		return resp, nil
	default:
		break
	}

	resp1, err := server.decodeResponse(resp)
	if resp1 != nil {
		resp1.Request = req
	}

	return resp1, err
}
