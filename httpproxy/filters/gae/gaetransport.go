package gae

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/phuslu/glog"
	quic "github.com/phuslu/quic-go"
	"github.com/phuslu/quic-go/h2quic"

	"../../helpers"
)

type Transport struct {
	RoundTripper http.RoundTripper
	MultiDialer  *helpers.MultiDialer
	RetryTimes   int
}

type QuicBody struct {
	quic.Stream
	OnTimeoutError func()
}

func (b *QuicBody) Read(data []byte) (int, error) {
	n, err := b.Stream.Read(data)
	if err != nil && b.OnTimeoutError != nil {
		if te, ok := err.(interface {
			Timeout() bool
		}); ok && te.Timeout() {
			b.OnTimeoutError()
		}
	}
	return n, err
}

func (t *Transport) roundTripQuic(req *http.Request) (*http.Response, error) {
	t1 := t.RoundTripper.(*h2quic.RoundTripper)

	resp, err := t1.RoundTripOpt(req, h2quic.RoundTripOpt{OnlyCachedConn: true})

	var shouldRetry bool
	switch err {
	case nil:
		break
	case h2quic.ErrNoCachedConn:
		shouldRetry = true
	default:
		if te, ok := err.(interface {
			Timeout() bool
		}); ok && te.Timeout() {
			t1.Close()
			shouldRetry = true
		} else if strings.Contains(err.Error(), "PublicReset:") {
			shouldRetry = true
		}
	}

	if shouldRetry {
		resp, err = t1.RoundTripOpt(req, h2quic.RoundTripOpt{OnlyCachedConn: false})
	}

	if resp != nil && resp.Body != nil {
		if stream, ok := resp.Body.(quic.Stream); ok {
			resp.Body = &QuicBody{
				Stream:         stream,
				OnTimeoutError: func() { t1.Close() },
			}
		}
	}

	return resp, err
}

func (t *Transport) roundTripTLS(req *http.Request) (*http.Response, error) {
	resp, err := t.RoundTripper.RoundTrip(req)

	if ne, ok := err.(*net.OpError); ok && ne != nil {
		switch {
		case ne.Addr == nil:
			break
		case ne.Error() == "unexpected EOF":
			helpers.CloseConnections(t.RoundTripper)
		case ne.Timeout() || ne.Op == "read":
			ip, _, _ := net.SplitHostPort(ne.Addr.String())
			glog.Warningf("GAE %s RoundTrip %s error: %#v, close connection to it", ne.Net, ip, ne.Err)
			helpers.CloseConnectionByRemoteHost(t.RoundTripper, ip)
			if t.MultiDialer != nil {
				duration := 5 * time.Minute
				glog.Warningf("GAE: %s is timeout, add to blacklist for %v", ip, duration)
				t.MultiDialer.IPBlackList.Set(ip, struct{}{}, time.Now().Add(duration))
			}
		}
	}

	return resp, err
}

func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	var err error
	var resp *http.Response

	_, isQuic := t.RoundTripper.(*h2quic.RoundTripper)

	for i := 0; i < t.RetryTimes; i++ {
		if i > 0 {
			glog.Warningf("GAE %T.RoundTrip(retry=%d) for %#v", t.RoundTripper, i, req.URL.String())
		}

		if isQuic {
			resp, err = t.roundTripQuic(req)
		} else {
			resp, err = t.roundTripTLS(req)
		}

		if err == nil {
			break
		}
	}

	if resp != nil && resp.StatusCode >= http.StatusBadRequest {
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

	return resp, err
}

type GAETransport struct {
	Transport   *Transport
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

		resp, err := t.Transport.RoundTrip(req1)

		if err != nil {
			if i == retryTimes-1 {
				return nil, err
			} else {
				glog.Warningf("GAE: request \"%s\" error: %T(%v), retry...", req.URL.String(), err, err)
				if err.Error() == "unexpected EOF" {
					helpers.CloseConnections(t.Transport.RoundTripper)
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
							if !helpers.CloseConnectionByRemoteHost(t.Transport.RoundTripper, host) {
								glog.Warningf("GAE: CloseConnectionByRemoteAddr(%T, %#v) failed.", t.Transport.RoundTripper, addr)
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
