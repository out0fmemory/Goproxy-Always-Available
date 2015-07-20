package storage

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	defaultFilePerm = 0644
)

type fileObject struct {
	resp *http.Response
}

func (o *fileObject) convertHeaderTime(name string) (time.Time, error) {
	value := o.resp.Header.Get(name)
	if value == "" {
		return time.Time{}, ErrNoLastModified
	}
	return time.Parse(DateFormat, value)
}

func (o *fileObject) Expires() (time.Time, error) {
	return time.Now().Add(1 * time.Hour), nil
}

func (o *fileObject) LastModified() (time.Time, error) {
	return o.convertHeaderTime("Last-Modified")
}

func (o *fileObject) ContentMD5() string {
	return o.resp.Header.Get("Content-MD5")
}

func (o *fileObject) ContentType() string {
	return o.resp.Header.Get("Content-Type")
}

func (o *fileObject) ContentEncoding() string {
	return o.resp.Header.Get("Content-Encoding")
}

func (o *fileObject) ContentLength() int64 {
	return o.resp.ContentLength
}

func (o *fileObject) ETag() string {
	return o.resp.Header.Get("Etag")
}

func (o *fileObject) Body() io.ReadCloser {
	return o.resp.Body
}

func (o *fileObject) Response() (*http.Response, error) {
	return o.resp, nil
}

type fileStore struct {
	Dirname string
}

func NewFileStore(dirname string) (Store, error) {
	return &fileStore{
		Dirname: dirname,
	}, nil
}

func (s *fileStore) URL() string {
	return fmt.Sprintf("file://%s", path.Clean(s.Dirname))
}

func (s *fileStore) DateFormat() string {
	return DateFormat
}

func (s *fileStore) GetObject(object string, start, end int64) (Object, error) {
	if start > 0 || end > 0 {
		return nil, fmt.Errorf("%T.GetObject do not support start end parameters", s)
	}

	req, err := http.NewRequest("GET", "/"+strings.TrimLeft(object, "/"), nil)

	filename := filepath.Join(s.Dirname, object)

	fi, err := os.Stat(filename)
	if err != nil {
		return nil, err
	}

	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	header := http.Header{}
	header.Set("Last-Modified", fi.ModTime().Format(DateFormat))
	header.Set("Content-Length", strconv.FormatInt(fi.Size(), 10))

	contentType := mime.TypeByExtension(filepath.Ext(filename))
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}
	header.Set("Content-Type", contentType)

	resp := &http.Response{
		Status:        "200 OK",
		StatusCode:    200,
		Proto:         "HTTP/1.0",
		ProtoMajor:    1,
		ProtoMinor:    0,
		Header:        header,
		Request:       req,
		Close:         true,
		ContentLength: fi.Size(),
		Body:          ioutil.NopCloser(bytes.NewReader(data)),
	}

	return &fileObject{resp}, nil
}

func (s *fileStore) HeadObject(object string) (http.Header, error) {
	filename := filepath.Join(s.Dirname, object)

	fi, err := os.Stat(filename)
	if err != nil {
		return nil, err
	}

	header := http.Header{}
	header.Set("Last-Modified", fi.ModTime().Format(DateFormat))
	header.Set("Content-Length", strconv.FormatInt(fi.Size(), 10))
	header.Set("Content-Type", mime.TypeByExtension(filepath.Ext(filename)))

	return header, nil
}

func (s *fileStore) PutObject(object string, header http.Header, data io.ReadCloser) error {
	defer data.Close()

	filename := filepath.Join(s.Dirname, object)
	b, err := ioutil.ReadAll(data)
	if err != nil {
		return err
	}

	if err = ioutil.WriteFile(filename, b, defaultFilePerm); err != nil {
		return err
	}

	return nil
}

func (s *fileStore) CopyObject(destObject string, srcObject string) error {
	data, err := ioutil.ReadFile(filepath.Join(s.Dirname, srcObject))
	if err != nil {
		return err
	}

	return ioutil.WriteFile(filepath.Join(s.Dirname, destObject), data, defaultFilePerm)
}

func (s *fileStore) DeleteObject(object string) error {
	filename := filepath.Join(s.Dirname, object)

	if err := os.Remove(filename); err != nil {
		return err
	}

	return nil
}
