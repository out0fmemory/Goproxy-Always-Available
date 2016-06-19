package storage

import (
	"archive/zip"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type zipObject struct {
	resp *http.Response
}

func (o *zipObject) convertHeaderTime(name string) (time.Time, error) {
	value := o.resp.Header.Get(name)
	if value == "" {
		return time.Time{}, ErrNoLastModified
	}
	return time.Parse(DateFormat, value)
}

func (o *zipObject) Expiry() (time.Time, error) {
	return time.Now().Add(1 * time.Hour), nil
}

func (o *zipObject) LastModified() (time.Time, error) {
	return o.convertHeaderTime("Last-Modified")
}

func (o *zipObject) ContentMD5() string {
	return o.resp.Header.Get("Content-MD5")
}

func (o *zipObject) ContentType() string {
	return o.resp.Header.Get("Content-Type")
}

func (o *zipObject) ContentEncoding() string {
	return o.resp.Header.Get("Content-Encoding")
}

func (o *zipObject) ContentLength() int64 {
	return o.resp.ContentLength
}

func (o *zipObject) ETag() string {
	return o.resp.Header.Get("Etag")
}

func (o *zipObject) Body() io.ReadCloser {
	return o.resp.Body
}

func (o *zipObject) Response() (*http.Response, error) {
	return o.resp, nil
}

type zipStore struct {
	Filename  string
	File      *os.File
	ZipReader *zip.Reader
	ZipFiles  map[string]*zip.File
}

func NewZipStore(filename string) (Store, error) {
	var err error
	var zs zipStore

	zs.Filename = filename
	zs.File, err = os.OpenFile(filename, os.O_RDWR, 0644)
	if err != nil {
		return nil, err
	}

	fi, err := zs.File.Stat()
	if err != nil {
		return nil, err
	}

	zs.ZipReader, err = zip.NewReader(zs.File, fi.Size())
	if err != nil {
		return nil, err
	}

	zs.ZipFiles = make(map[string]*zip.File)
	for _, zf := range zs.ZipReader.File {
		zs.ZipFiles[zf.Name] = zf
	}

	return &zs, nil
}

func (s *zipStore) URL() string {
	return fmt.Sprintf("zip://%s", path.Clean(s.Filename))
}

func (s *zipStore) DateFormat() string {
	return DateFormat
}

func (s *zipStore) GetObject(object string, start, end int64) (Object, error) {
	if start > 0 || end > 0 {
		return nil, fmt.Errorf("%T.GetObject do not support start end parameters", s)
	}

	zf, ok := s.ZipFiles[object]
	if !ok {
		return nil, ErrNotExists
	}

	req, err := http.NewRequest(http.MethodGet, "/"+strings.TrimLeft(object, "/"), nil)
	if err != nil {
		return nil, err
	}

	header := http.Header{}
	header.Set("Last-Modified", zf.ModTime().Format(DateFormat))
	header.Set("Content-Length", strconv.FormatUint(zf.UncompressedSize64, 10))

	contentType := mime.TypeByExtension(filepath.Ext(zf.Name))
	if contentType != "" {
		header.Set("Content-Type", contentType)
	}

	rc, err := zf.Open()
	if err != nil {
		return nil, err
	}

	resp := &http.Response{
		StatusCode:    http.StatusOK,
		Header:        header,
		Request:       req,
		Close:         true,
		ContentLength: int64(zf.UncompressedSize64),
		Body:          rc,
	}

	return &zipObject{resp}, nil
}

func (s *zipStore) HeadObject(object string) (http.Header, error) {

	zf, ok := s.ZipFiles[object]
	if !ok {
		return nil, ErrNotExists
	}

	var header http.Header
	header.Set("Last-Modified", zf.ModTime().Format(DateFormat))
	header.Set("Content-Length", strconv.FormatUint(zf.UncompressedSize64, 10))

	contentType := mime.TypeByExtension(filepath.Ext(zf.Name))
	if contentType != "" {
		header.Set("Content-Type", contentType)
	}

	return header, nil
}

func (s *zipStore) PutObject(object string, header http.Header, data io.ReadCloser) error {
	return ErrNotImplemented
}

func (s *zipStore) CopyObject(destObject string, srcObject string) error {
	return ErrNotImplemented
}

func (s *zipStore) DeleteObject(object string) error {
	return ErrNotImplemented
}
