package storage

import (
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

const (
	DateFormat = "Mon, 02 Jan 2006 15:04:05 GMT"
)

var (
	ErrNotImplemented error = errors.New("Not Implemented")
)

type Store interface {
	Get(name string, start, end int64) (*http.Response, error)
	Put(name string, header http.Header, data io.ReadCloser) (*http.Response, error)
	Copy(dest string, src string) (*http.Response, error)
	Head(name string) (*http.Response, error)
	Delete(name string) (*http.Response, error)
	UnmarshallJson(name string, config interface{}) error
}

// Lookup config uri by filename
func LookupStoreByConfig(name string) Store {
	var store Store
	for _, dirname := range []string{filepath.Dir(os.Args[0]), ".", "httpproxy", "httpproxy/filters/" + name} {
		filename := dirname + "/" + name + ".json"
		if _, err := os.Stat(filename); err == nil {
			store = &fileStore{dirname}
			break
		}
	}
	if store == nil {
		store = &fileStore{"."}
	}
	return store
}
