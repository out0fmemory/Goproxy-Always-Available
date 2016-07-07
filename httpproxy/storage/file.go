package storage

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type fileStore struct {
	Dirname string
}

var _ Store = &fileStore{}

func (s *fileStore) Get(name string, start, end int64) (*http.Response, error) {
	if start > 0 || end > 0 {
		return nil, fmt.Errorf("%T.GetObject do not support start end parameters", s)
	}

	req, err := http.NewRequest(http.MethodGet, "/"+strings.TrimLeft(name, "/"), nil)

	filename := filepath.Join(s.Dirname, name)

	fi, err := os.Stat(filename)
	if err != nil {
		return nil, err
	}

	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	resp := &http.Response{
		StatusCode:    http.StatusOK,
		Header:        http.Header{},
		Request:       req,
		Close:         true,
		ContentLength: fi.Size(),
		Body:          ioutil.NopCloser(bytes.NewReader(data)),
	}

	resp.Header.Set("Last-Modified", fi.ModTime().Format(DateFormat))
	resp.Header.Set("Content-Type", mime.TypeByExtension(filepath.Ext(filename)))

	return resp, nil
}

func (s *fileStore) Head(name string) (*http.Response, error) {
	filename := filepath.Join(s.Dirname, name)

	req, _ := http.NewRequest(http.MethodHead, "/"+strings.TrimLeft(name, "/"), nil)

	fi, err := os.Stat(filename)
	if err != nil {
		return nil, err
	}

	resp := &http.Response{
		StatusCode:    http.StatusOK,
		Header:        http.Header{},
		Request:       req,
		Close:         true,
		ContentLength: fi.Size(),
	}

	resp.Header.Set("Last-Modified", fi.ModTime().Format(DateFormat))
	resp.Header.Set("Content-Type", mime.TypeByExtension(filepath.Ext(filename)))

	return resp, nil
}

func (s *fileStore) Put(name string, header http.Header, data io.ReadCloser) (*http.Response, error) {
	defer data.Close()

	filename := filepath.Join(s.Dirname, name)

	f, err := ioutil.TempFile(filepath.Dir(filename), filepath.Base(filename)+".")
	if err != nil {
		return nil, err
	}

	if _, err = io.Copy(f, data); err != nil {
		return nil, err
	}
	if err = f.Close(); err != nil {
		return nil, err
	}

	if err = os.Rename(f.Name(), filename); err != nil {
		return nil, err
	}

	resp := &http.Response{
		StatusCode:    http.StatusOK,
		Header:        http.Header{},
		ContentLength: 0,
	}

	return resp, nil
}

func (s *fileStore) Copy(dest string, src string) (*http.Response, error) {
	data, err := ioutil.ReadFile(filepath.Join(s.Dirname, src))
	if err != nil {
		return nil, err
	}

	err = ioutil.WriteFile(filepath.Join(s.Dirname, dest), data, 0644)
	if err != nil {
		return nil, err
	}

	resp := &http.Response{
		StatusCode:    http.StatusOK,
		Header:        http.Header{},
		ContentLength: 0,
	}

	return resp, nil
}

func (s *fileStore) Delete(object string) (*http.Response, error) {
	filename := filepath.Join(s.Dirname, object)

	if err := os.Remove(filename); err != nil {
		return nil, err
	}

	resp := &http.Response{
		StatusCode:    http.StatusOK,
		Header:        http.Header{},
		ContentLength: 0,
	}

	return resp, nil
}

func (s *fileStore) UnmarshallJson(name string, config interface{}) error {
	return readJsonConfig(s, name, config)
}
