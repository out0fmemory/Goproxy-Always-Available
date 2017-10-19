package gae

import (
	"bytes"
	"context"
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
	Transport   *h2quic.RoundTripper
	MultiDialer *helpers.MultiDialer
}

func (b *QuicBody) OnError(err error) {
	var shouldClose bool

	if te, ok := err.(interface {
		Timeout() bool
	}); ok && te.Timeout() {
		shouldClose = true
	}

	if shouldClose {
		b.Transport.Close()
		ip, _, _ := net.SplitHostPort(b.RemoteAddr().String())
		duration := 5 * time.Minute
		glog.Warningf("GAE: QuicBody(%v) is timeout, add to blacklist for %v", ip, duration)
		b.MultiDialer.IPBlackList.Set(ip, struct{}{}, time.Now().Add(duration))
	}
}

func (t *Transport) roundTripQuic(req *http.Request) (*http.Response, error) {
	t1 := t.RoundTripper.(*h2quic.RoundTripper)

	if !strings.HasSuffix(req.Host, ".appspot.com") {
		req = req.WithContext(context.WithValue(req.Context(), "ResponseHeaderTimeout", 8*time.Second))
	}

	resp, err := t1.RoundTripOpt(req, h2quic.RoundTripOpt{OnlyCachedConn: true})

	if err != nil {
		if ne, ok := err.(*net.OpError); ok && ne != nil && ne.Addr != nil {
			ip, _, _ := net.SplitHostPort(ne.Addr.String())
			duration := 5 * time.Minute
			glog.Warningf("GAE: QuicBody(%v) is timeout, add to blacklist for %v", ip, duration)
			t.MultiDialer.IPBlackList.Set(ip, struct{}{}, time.Now().Add(duration))
			helpers.CloseConnectionByRemoteHost(t1, ip)
		} else {
			t1.Close()
		}
		resp, err = t1.RoundTripOpt(req, h2quic.RoundTripOpt{OnlyCachedConn: false})
	}

	if resp != nil && resp.Body != nil {
		if stream, ok := resp.Body.(quic.Stream); ok {
			resp.Body = &QuicBody{
				Stream:      stream,
				Transport:   t1,
				MultiDialer: t.MultiDialer,
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

	retry := t.RetryTimes
	if req.Method != http.MethodGet && req.Header.Get("Content-Length") != "" {
		retry = 1
	}

	for i := 0; i < retry; i++ {
		if isQuic {
			resp, err = t.roundTripQuic(req)
		} else {
			resp, err = t.roundTripTLS(req)
		}

		if err != nil {
			glog.Warningf("GAE %T.RoundTrip(%#v) error: %+v", t.RoundTripper, req.URL.String(), err)
			continue
		}

		if resp != nil && resp.StatusCode == http.StatusBadRequest {
			glog.Warningf("GAE %T.RoundTrip(%#v) get HTTP Error %d", t.RoundTripper, req.URL.String(), resp.StatusCode)
			continue
		}

		break
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
				case resp.StatusCode >= 301 && resp.Header.Get("Location") != "":
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
					helpers.CloseConnectionByRemoteHost(t.RoundTripper, ip)
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
	BrotliSites *helpers.HostMatcher
	Deadline    time.Duration
	RetryDelay  time.Duration
	RetryTimes  int
}

func (t *GAETransport) RoundTrip(req *http.Request) (*http.Response, error) {
	deadline := t.Deadline
	brotli := t.BrotliSites.Match(req.Host) && strings.Contains(req.Header.Get("Accept-Encoding"), "br")
	retryTimes := t.RetryTimes
	retryDelay := t.RetryDelay
	for i := 0; i < retryTimes; i++ {
		server := t.Servers.PickFetchServer(req, i)
		req1, err := t.Servers.EncodeRequest(req, server, deadline, brotli)
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
							helpers.CloseConnectionByRemoteHost(t.Transport.RoundTripper, ip)
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
