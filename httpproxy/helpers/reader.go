package helpers

import (
	"io"
)

type multiReadCloser struct {
	readers     []io.Reader
	multiReader io.Reader
}

func NewMultiReadCloser(readers ...io.Reader) io.ReadCloser {
	return &multiReadCloser{
		readers:     readers,
		multiReader: io.MultiReader(readers...),
	}
}

func (r *multiReadCloser) Read(p []byte) (n int, err error) {
	return r.multiReader.Read(p)
}

func (r *multiReadCloser) Close() (err error) {
	for _, r := range r.readers {
		if c, ok := r.(io.Closer); ok {
			if e := c.Close(); e != nil {
				err = e
			}
		}
	}

	return err
}

type xorReadCloser struct {
	rc  io.ReadCloser
	key []byte
}

func NewXorReadCloser(rc io.ReadCloser, key []byte) io.ReadCloser {
	return &xorReadCloser{
		rc:  rc,
		key: key,
	}
}

func (x *xorReadCloser) Read(p []byte) (n int, err error) {
	n, err = x.rc.Read(p)
	c := x.key[0]
	for i := 0; i < n; i++ {
		p[i] ^= c
	}

	return n, err
}

func (x *xorReadCloser) Close() error {
	return x.rc.Close()
}
