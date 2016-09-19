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
	Get(name string) (*http.Response, error)
	List(name string) ([]string, error)
	Put(name string, header http.Header, data io.ReadCloser) (*http.Response, error)
	Copy(dest string, src string) (*http.Response, error)
	Head(name string) (*http.Response, error)
	Delete(name string) (*http.Response, error)
	UnmarshallJson(name string, config interface{}) error
}

// Lookup config uri by filename
func LookupStoreByFilterName(name string) Store {
	var store Store
	for _, dirname := range []string{filepath.Dir(os.Args[0]), ".", "httpproxy", "httpproxy/filters/" + name} {
		filename := dirname + "/" + name + ".json"
		if _, err := os.Stat(filename); err == nil {
			store = &FileStore{dirname}
			break
		}
	}
	if store == nil {
		store = &FileStore{"."}
	}
	return store
}

func NotExist(store Store, name string) bool {
	resp, err := store.Head(name)
	return os.IsNotExist(err) || (resp != nil && resp.StatusCode == http.StatusNotFound)
}
