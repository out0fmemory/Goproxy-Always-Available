package gae

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/phuslu/glog"
	"github.com/phuslu/quic-go/h2quic"

	"../../helpers"
)

type onErrorBody struct {
	io.ReadCloser
	OnError func(error)
}

func (b *onErrorBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	if b.OnError != nil && err != nil {
		b.OnError(err)
	}
	return n, err
}

type Transport struct {
	RoundTripper http.RoundTripper
	MultiDialer  *helpers.MultiDialer
	RetryTimes   int
}

func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	var err error
	var resp *http.Response
	for i := 0; i < t.RetryTimes; i++ {
		if i > 0 {
			glog.Warningf("GAE %T.RoundTrip(retry=%d) for %#v", t, i, req.URL.String())
		}

		resp, err = t.RoundTripper.RoundTrip(req)

		if i == t.RetryTimes-1 {
			break
		}

		if ne, ok := err.(*net.OpError); ok && ne.Addr != nil {
			if ip, _, err := net.SplitHostPort(ne.Addr.String()); err == nil {
				if ne.Timeout() || ne.Op == "read" {
					glog.Warningf("GAE %s RoundTrip %s error: %#v, close connection to it", ne.Net, ip, ne.Err)
					switch ne.Net {
					case "udp":
						helpers.CloseConnections(t.RoundTripper)
					default:
						helpers.CloseConnectionByRemoteHost(t.RoundTripper, ip)
					}
					if t.MultiDialer != nil {
						duration := 5 * time.Minute
						glog.Warningf("GAE: %s is timeout, add to blacklist for %v", ip, duration)
						t.MultiDialer.IPBlackList.Set(ip, struct{}{}, time.Now().Add(duration))
					}
				}
			}
			if err.Error() == "unexpected EOF" {
				helpers.CloseConnections(t.RoundTripper)
				break
			}
		}

		if resp != nil && err == nil {
			if resp.StatusCode >= http.StatusBadRequest {
				body, err := ioutil.ReadAll(resp.Body)
				if err != nil {
					resp.Body.Close()
					return nil, err
				}

				if addr, err := helpers.ReflectRemoteAddrFromResponse(resp); err == nil {
					if ip, _, err := net.SplitHostPort(addr); err == nil {
						var duration time.Duration
						switch {
						case resp.StatusCode == http.StatusBadGateway && bytes.Contains(body, []byte("Please try again in 30 seconds.")):
							duration = 1 * time.Hour
						case resp.StatusCode >= 301 && strings.Contains(resp.Header.Get("Location"), "hangouts.google.com"):
							duration = 2 * time.Hour
						case resp.StatusCode == http.StatusNotFound && bytes.Contains(body, []byte("<ins>That’s all we know.</ins>")):
							server := resp.Header.Get("Server")
							if server != "gws" && !strings.HasPrefix(server, "gvs") {
								if t.MultiDialer.TLSConnDuration.Len() > 10 {
									duration = 5 * time.Minute
								}
							}
						}

						if duration > 0 && t.MultiDialer != nil {
							glog.Warningf("GAE: %s StatusCode is %d, not a gws/gvs ip, add to blacklist for %v", ip, resp.StatusCode, duration)
							t.MultiDialer.IPBlackList.Set(ip, struct{}{}, time.Now().Add(duration))

							if !helpers.CloseConnectionByRemoteHost(t.RoundTripper, ip) {
								glog.Warningf("GAE: CloseConnectionByRemoteHost(%T, %#v) failed.", t.RoundTripper, ip)
							}
						}
					}
				}

				resp.Body.Close()
				resp.Body = ioutil.NopCloser(bytes.NewReader(body))
			}
			break
		}
	}

	if resp != nil && resp.Body != nil {
		if _, ok := t.RoundTripper.(*h2quic.RoundTripper); ok {
			resp.Body = &onErrorBody{
				ReadCloser: resp.Body,
				OnError: func(err error) {
					if ne, ok := err.(*net.OpError); ok && ne.Op == "read" {
						glog.Warningf("GAE %s resp.Body OnError: %+v, close all connection to it", ne.Net, ne.Err)
						helpers.CloseConnections(t.RoundTripper)
					}
				},
			}
		}
	}

	return resp, err
}

type GAETransport struct {
	http.RoundTripper
	MultiDialer *helpers.MultiDialer
	Servers     *Servers
	Deadline    time.Duration
	RetryDelay  time.Duration
	RetryTimes  int
}

func (t *GAETransport) RoundTrip(req *http.Request) (*http.Response, error) {
	deadline := t.Deadline
	retryTimes := t.RetryTimes
	retryDelay := t.RetryDelay
	for i := 0; i < retryTimes; i++ {
		server := t.Servers.PickFetchServer(req, i)
		req1, err := t.Servers.EncodeRequest(req, server, deadline)
		if err != nil {
			return nil, fmt.Errorf("GAE EncodeRequest: %s", err.Error())
		}

		resp, err := t.RoundTripper.RoundTrip(req1)

		if err != nil {
			if i == retryTimes-1 {
				return nil, err
			} else {
				glog.Warningf("GAE: request \"%s\" error: %T(%v), retry...", req.URL.String(), err, err)
				if err.Error() == "unexpected EOF" {
					helpers.CloseConnections(t.RoundTripper)
					return nil, err
				}
				continue
			}
		}

		if resp.StatusCode != http.StatusOK {
			if i == retryTimes-1 {
				return resp, nil
			}

			switch resp.StatusCode {
			case http.StatusServiceUnavailable:
				glog.Warningf("GAE: %s over qouta, try switch to next appid...", server.Host)
				t.Servers.ToggleBadServer(server)
				time.Sleep(retryDelay)
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
						if host, _, err := net.SplitHostPort(addr); err == nil {
							if !helpers.CloseConnectionByRemoteHost(t.RoundTripper, host) {
								glog.Warningf("GAE: CloseConnectionByRemoteAddr(%T, %#v) failed.", t.RoundTripper, addr)
							}
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
		if i == retryTimes-1 {
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
				time.Sleep(deadline)
				continue
			case bytes.Contains(body, []byte("ver quota")):
				glog.Warningf("GAE: %s urlfetch %#v get over quota, retry...", req1.Host, req.URL.String())
				t.Servers.ToggleBadServer(server)
				time.Sleep(retryDelay)
				continue
			case bytes.Contains(body, []byte("urlfetch: CLOSED")):
				glog.Warningf("GAE: %s urlfetch %#v get urlfetch: CLOSED, retry...", req1.Host, req.URL.String())
				time.Sleep(retryDelay)
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
