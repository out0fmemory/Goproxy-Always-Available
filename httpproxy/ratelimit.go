package httpproxy

import (
	"io"

	"github.com/juju/ratelimit"
)

type rateLimitReader struct {
	rc  io.ReadCloser // underlying reader
	rlr io.Reader     // ratelimit.Reader
}

func NewRateLimitReader(rc io.ReadCloser, rate float64, capacity int64) io.ReadCloser {
	var rlr rateLimitReader

	rlr.rc = rc
	rlr.rlr = ratelimit.Reader(rc, ratelimit.NewBucketWithRate(rate, capacity))

	return &rlr
}

func (rlr *rateLimitReader) Read(p []byte) (n int, err error) {
	return rlr.rlr.Read(p)
}

func (rlr *rateLimitReader) Close() error {
	return rlr.rc.Close()
}
