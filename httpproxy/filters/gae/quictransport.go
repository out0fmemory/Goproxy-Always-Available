package gae

import (
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/phuslu/glog"
	"github.com/phuslu/quic-go/h2quic"

	"../../helpers"
)

type QuicTransport struct {
	RoundTripper *h2quic.QuicRoundTripper
	MultiDialer  *helpers.MultiDialer
	RetryDelay   time.Duration
	RetryTimes   int
}

func (t *QuicTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var err error
	var resp *http.Response
	for i := 0; i < t.RetryTimes; i++ {
		resp, err = t.RoundTripper.RoundTrip(req)

		if err == nil {
			return resp, nil
		}

		if i == t.RetryTimes-1 {
			break
		}

		ne, ok := err.(*net.OpError)
		if !ok {
			break
		}

		shouldClose := strings.HasPrefix(ne.Err.Error(), "NetworkIdleTimeout:")

		if true || shouldClose {
			// FIXME: fix InvalidStreamID bug, see https://github.com/lucas-clemente/quic-go/issues/691
			if ip, _, err := net.SplitHostPort(ne.Addr.String()); err == nil {
				glog.Warningf("GAE Quic RoundTrip %s error: %+v, close connection to it", ip, ne.Err)
				helpers.CloseConnectionByRemoteHost(t.RoundTripper, ip)
				if t.MultiDialer != nil {
					duration := 5 * time.Minute
					glog.Warningf("GAE: %s is timeout, add to blacklist for %v", ip, duration)
					t.MultiDialer.IPBlackList.Set(ip, struct{}{}, time.Now().Add(duration))
				}
			}
		}

		if t.RetryDelay > 0 {
			time.Sleep(t.RetryDelay)
		}
	}
	return resp, err
}
