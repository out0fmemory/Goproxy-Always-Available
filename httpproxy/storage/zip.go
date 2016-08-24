package storage

import (
	"archive/zip"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type ZipStore struct {
	Filename string
	zfs      map[string]*zip.File
}

var _ Store = &ZipStore{}

func (s *ZipStore) init() error {
	if s.zfs != nil {
		return nil
	}

	var err error

	f, err := os.OpenFile(s.Filename, os.O_RDWR, 0644)
	if err != nil {
		return err
	}

	fi, err := f.Stat()
	if err != nil {
		return err
	}

	zr, err := zip.NewReader(f, fi.Size())
	if err != nil {
		return err
	}

	s.zfs = make(map[string]*zip.File)
	for _, zf := range zr.File {
		s.zfs[zf.Name] = zf
	}

	return nil
}

func (s *ZipStore) Get(name string) (*http.Response, error) {
	if err := s.init(); err != nil {
		return nil, err
	}

	zf, ok := s.zfs[name]
	if !ok {
		return nil, os.ErrNotExist
	}

	req, err := http.NewRequest(http.MethodGet, "/"+strings.TrimLeft(name, "/"), nil)
	if err != nil {
		return nil, err
	}

	rc, err := zf.Open()
	if err != nil {
		return nil, err
	}

	resp := &http.Response{
		StatusCode:    http.StatusOK,
		Header:        http.Header{},
		Request:       req,
		Close:         true,
		ContentLength: int64(zf.UncompressedSize64),
		Body:          rc,
	}

	resp.Header.Set("Last-Modified", zf.ModTime().Format(DateFormat))
	resp.Header.Set("Content-Length", strconv.FormatUint(zf.UncompressedSize64, 10))
	if ct := mime.TypeByExtension(filepath.Ext(zf.Name)); ct != "" {
		resp.Header.Set("Content-Type", ct)
	}

	return resp, nil
}

func (s *ZipStore) List(name string) ([]string, error) {
	if err := s.init(); err != nil {
		return nil, err
	}

	prefix := strings.TrimRight(name, "/") + "/"
	names := make([]string, 0)
	for key := range s.zfs {
		if strings.HasPrefix(key, prefix) {
			names = append(names, key)
		}
	}

	return names, nil
}

func (s *ZipStore) Head(name string) (*http.Response, error) {
	if err := s.init(); err != nil {
		return nil, err
	}

	zf, ok := s.zfs[name]
	if !ok {
		return nil, os.ErrNotExist
	}

	req, err := http.NewRequest(http.MethodGet, "/"+strings.TrimLeft(name, "/"), nil)
	if err != nil {
		return nil, err
	}

	resp := &http.Response{
		StatusCode:    http.StatusOK,
		Header:        http.Header{},
		Request:       req,
		Close:         true,
		ContentLength: int64(zf.UncompressedSize64),
	}

	resp.Header.Set("Last-Modified", zf.ModTime().Format(DateFormat))
	resp.Header.Set("Content-Length", strconv.FormatUint(zf.UncompressedSize64, 10))
	if ct := mime.TypeByExtension(filepath.Ext(zf.Name)); ct != "" {
		resp.Header.Set("Content-Type", ct)
	}

	return resp, nil
}

func (s *ZipStore) Put(name string, header http.Header, data io.ReadCloser) (*http.Response, error) {
	return nil, ErrNotImplemented
}

func (s *ZipStore) Copy(dest string, src string) (*http.Response, error) {
	return nil, ErrNotImplemented
}

func (s *ZipStore) Delete(name string) (*http.Response, error) {
	return nil, ErrNotImplemented
}

func (s *ZipStore) UnmarshallJson(name string, config interface{}) error {
	return readJsonConfig(s, name, config)
}
