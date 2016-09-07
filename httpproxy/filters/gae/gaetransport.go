package gae

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"time"

	"github.com/phuslu/glog"

	"../../helpers"
)

type Transport struct {
	http.RoundTripper
	MultiDialer *helpers.MultiDialer
	Servers     *Servers
	Deadline    time.Duration
	RetryDelay  time.Duration
	RetryTimes  int
}

func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	deadline := t.Deadline
	for i := 0; i < t.RetryTimes; i++ {
		server := t.Servers.PickFetchServer(req, i)
		req1, err := t.Servers.EncodeRequest(req, server, deadline)
		if err != nil {
			return nil, fmt.Errorf("GAE EncodeRequest: %s", err.Error())
		}

		resp, err := t.RoundTripper.RoundTrip(req1)

		if err != nil {

			isTimeoutError := false
			if ne, ok := err.(interface {
				Timeout() bool
			}); ok && ne.Timeout() {
				isTimeoutError = true
			}
			if ne, ok := err.(*net.OpError); ok && ne.Op == "read" {
				isTimeoutError = true
			}

			if isTimeoutError {
				glog.Warningf("GAE: \"%s %s\" timeout: %v, helpers.CloseConnections(%T)", req.Method, req.URL.String(), err, t.RoundTripper)
				helpers.CloseConnections(t.RoundTripper)
			}

			if i == t.RetryTimes-1 {
				return nil, err
			} else {
				glog.Warningf("GAE: request \"%s\" error: %T(%v), retry...", req.URL.String(), err, err)
				continue
			}
		}

		if resp.StatusCode != http.StatusOK {
			if i == t.RetryTimes-1 {
				return resp, nil
			}

			switch resp.StatusCode {
			case http.StatusServiceUnavailable:
				glog.Warningf("GAE: %s over qouta, try switch to next appid...", server.Host)
				t.Servers.ToggleBadServer(server)
				time.Sleep(t.RetryDelay)
				continue
			case http.StatusFound,
				http.StatusBadGateway,
				http.StatusNotFound,
				http.StatusMethodNotAllowed:
				if t.MultiDialer != nil {
					if addr, err := helpers.ReflectRemoteAddrFromResponse(resp); err == nil {
						if ip, _, err := net.SplitHostPort(addr); err == nil {
							duration := 8 * time.Hour
							glog.Warningf("GAE: %s StatusCode is %d, not a gws/gvs ip, add to blacklist for %v", ip, resp.StatusCode, duration)
							t.MultiDialer.IPBlackList.Set(ip, struct{}{}, time.Now().Add(duration))
						}
						if !helpers.CloseConnectionByRemoteAddr(t.RoundTripper, addr) {
							glog.Warningf("GAE: CloseConnectionByRemoteAddr(%T, %#v) failed.", t.RoundTripper, addr)
						}
					}
				}
				continue
			default:
				return resp, nil
			}
		}

		resp1, err := t.Servers.DecodeResponse(resp)
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
				//FIXME: deadline += 10 * time.Second
				glog.Warningf("GAE: %s urlfetch %#v get DEADLINE_EXCEEDED, retry with deadline=%s...", req1.Host, req.URL.String(), deadline)
				continue
			case bytes.Contains(body, []byte("ver quota")):
				glog.Warningf("GAE: %s urlfetch %#v get over quota, retry...", req1.Host, req.URL.String())
				time.Sleep(t.RetryDelay)
				continue
			case bytes.Contains(body, []byte("urlfetch: CLOSED")):
				glog.Warningf("GAE: %s urlfetch %#v get urlfetch: CLOSED, retry...", req1.Host, req.URL.String())
				time.Sleep(t.RetryDelay)
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
