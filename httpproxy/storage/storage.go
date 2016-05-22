package storage

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	DateFormat = "Mon, 02 Jan 2006 15:04:05 GMT"
)

var (
	ErrNotImplemented error = errors.New("Not Implemented")
	ErrNotExists      error = errors.New("Not Exists")
	ErrNoExpiry       error = errors.New("No Expiry Field")
	ErrNoLastModified error = errors.New("No LastModified Field")
)

type Object interface {
	LastModified() (time.Time, error)
	ETag() string
	Expiry() (time.Time, error)
	ContentMD5() string
	ContentLength() int64
	ContentType() string
	ContentEncoding() string
	Body() io.ReadCloser
	Response() (*http.Response, error)
}

type Store interface {
	URL() string
	DateFormat() string
	GetObject(object string, start, end int64) (Object, error)
	PutObject(object string, header http.Header, data io.ReadCloser) error
	CopyObject(destObject string, srcObject string) error
	HeadObject(object string) (http.Header, error)
	DeleteObject(object string) error
}

func OpenURI(uri string) (Store, error) {
	parts := strings.SplitN(uri, "://", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("Invalid URI: %s", uri)
	}
	scheme := parts[0]
	dirname := parts[1]
	if dirname == "" {
		dirname = "."
	}
	return Open(scheme, dirname)
}

func expandPick(sourceString string) string {
	matches, err := filepath.Glob(sourceString)
	if err != nil || len(matches) == 0 {
		return sourceString
	}

	sort.Strings(matches)

	return matches[len(matches)-1]
}

func Open(driver, sourceString string) (Store, error) {
	switch driver {
	case "file":
		return NewFileStore(expandPick(sourceString))
	case "zip":
		return NewZipStore(expandPick(sourceString))
	default:
		return nil, fmt.Errorf("Invaild Storage dirver: %#v", driver)
	}
}

const (
	EnvConfigStoreURI = "CONFIG_STORE_URI"
	ConfigZip         = "config.zip"
)

// Lookup config uri by filename
func LookupConfigStoreURI(filterName string) string {
	if env := os.Getenv(EnvConfigStoreURI); env != "" {
		return env
	}

	if fi, err := os.Stat(ConfigZip); err == nil && !fi.IsDir() {
		return "zip://" + ConfigZip
	}

	for _, dirname := range []string{filepath.Dir(os.Args[0]), ".", "httpproxy", "httpproxy/filters/" + filterName} {
		filename := dirname + "/" + filterName + ".json"
		if _, err := os.Stat(filename); err == nil {
			return "file://" + dirname
		}
	}
	return "file://."
}
