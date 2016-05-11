package gae

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"time"

	"../../dialer"
	"../../helpers"

	"github.com/phuslu/glog"
)

type Transport struct {
	http.RoundTripper
	MultiDialer *dialer.MultiDialer
	Servers     *Servers
	RetryDelay  time.Duration
	RetryTimes  int
}

func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	for i := 0; i < t.RetryTimes; i++ {
		server := t.Servers.pickServer(req, i)

		req1, err := server.encodeRequest(req)
		if err != nil {
			return nil, fmt.Errorf("GAE encodeRequest: %s", err.Error())
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
				if t1, ok := t.RoundTripper.(interface {
					CloseConnections()
				}); ok {
					glog.Warningf("GAE: request \"%s\" timeout: %v, %T.CloseConnections()", req.URL.String(), err, t1)
					t1.CloseConnections()
					// t.MultiDialer.ClearCache()
				} else if t1, ok := t.RoundTripper.(interface {
					CloseIdleConnections()
				}); ok {
					glog.Warningf("GAE: request \"%s\" timeout: %v, %T.CloseIdleConnections()", req.URL.String(), err, t1)
					t1.CloseIdleConnections()
					// t.MultiDialer.ClearCache()
				}
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
				if t.Servers.Len() == 1 {
					glog.Warningf("GAE: %s over qouta, please add more appids to gae.user.json", server.URL.Host)
					return resp, nil
				} else {
					glog.Warningf("GAE: %s over qouta, switch to next appid...", server.URL.Host)
					t.Servers.roundServers()
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
				glog.V(2).Infof("GAE: %s urlfetch %#v get DEADLINE_EXCEEDED, retry...", req1.URL.Host, req.URL.String())
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
