package httpproxy

import (
	"io"
)

type multiReadCloser struct {
	readers     []io.Reader
	multiReader io.Reader
}

func NewMultiReadCloser(readers ...io.Reader) io.ReadCloser {
	r := new(multiReadCloser)
	r.readers = readers
	r.multiReader = io.MultiReader(readers...)
	return r
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
