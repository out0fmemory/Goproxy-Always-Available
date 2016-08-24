package storage

import (
	"bytes"
	"io"
	"io/ioutil"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

type FileStore struct {
	Dirname string
}

var _ Store = &FileStore{}

func (s *FileStore) Get(name string) (*http.Response, error) {
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

func (s *FileStore) List(name string) ([]string, error) {
	fis, err := ioutil.ReadDir(filepath.Join(s.Dirname, name))
	if err != nil {
		return nil, err
	}

	names := make([]string, len(fis))
	for i, fi := range fis {
		names[i] = path.Join(name, fi.Name())
	}

	return names, nil
}

func (s *FileStore) Head(name string) (*http.Response, error) {
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

func (s *FileStore) Put(name string, header http.Header, data io.ReadCloser) (*http.Response, error) {
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

func (s *FileStore) Copy(dest string, src string) (*http.Response, error) {
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

func (s *FileStore) Delete(object string) (*http.Response, error) {
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

func (s *FileStore) UnmarshallJson(name string, config interface{}) error {
	return readJsonConfig(s, name, config)
}
