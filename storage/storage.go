package storage

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"path"
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

func ReadJson(r io.Reader) ([]byte, error) {

	s, err := ioutil.ReadAll(r)
	if err != nil {
		return s, err
	}

	lines := strings.Split(string(s), "\r\n")

	var b bytes.Buffer
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "//") {
			continue
		}
		if i < len(lines)-1 {
			nextLine := strings.TrimSpace(lines[i+1])
			if nextLine == "]" ||
				nextLine == "]," ||
				nextLine == "}" ||
				nextLine == "}," {
				if strings.HasSuffix(line, ",") {
					line = strings.TrimSuffix(line, ",")
				}
			}
		}
		b.WriteString(line)
	}

	return b.Bytes(), nil
}

func ReadJsonConfig(uri, filename string, config interface{}) error {
	store, err := OpenURI(uri)
	if err != nil {
		return err
	}

	fileext := path.Ext(filename)
	filename1 := strings.TrimSuffix(filename, fileext) + ".user" + fileext

	for i, name := range []string{filename, filename1} {
		object, err := store.GetObject(name, -1, -1)
		if err != nil {
			if i == 0 {
				return err
			} else {
				continue
			}
		}

		rc := object.Body()
		defer rc.Close()

		data, err := ReadJson(rc)
		if err != nil {
			return err
		}

		err = json.Unmarshal(data, config)
		if err != nil {
			return err
		}
	}

	return nil
}
