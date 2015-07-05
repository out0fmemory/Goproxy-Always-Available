package filters

import (
	"fmt"
	"net/http"
	"os"
)

const (
	PackageName       = "httpproxy/filters"
	EnvConfigStoreURI = "CONFIG_STORE_URI"
	ConfigZip         = "config.zip"
)

type Filter interface {
	FilterName() string
}

type RequestFilter interface {
	FilterName() string
	Request(*Context, *http.Request) (*Context, *http.Request, error)
}

type RoundTripFilter interface {
	FilterName() string
	RoundTrip(*Context, *http.Request) (*Context, *http.Response, error)
}

type ResponseFilter interface {
	FilterName() string
	Response(*Context, *http.Response) (*Context, *http.Response, error)
}

type RegisteredFilter struct {
	New func() (Filter, error)
}

var (
	filters map[string]*RegisteredFilter
)

func init() {
	filters = make(map[string]*RegisteredFilter)
}

// Register a Filter
func Register(name string, registeredFilter *RegisteredFilter) error {
	if _, exists := filters[name]; exists {
		return fmt.Errorf("Name already registered %s", name)
	}

	filters[name] = registeredFilter
	return nil
}

// Lookup config uri by filename
func LookupConfigStoreURI(filterName string) string {
	if env := os.Getenv(EnvConfigStoreURI); env != "" {
		return env
	}

	if fi, err := os.Stat(ConfigZip); err == nil && !fi.IsDir() {
		return "zip://" + ConfigZip
	}

	for _, dirname := range []string{".", "../" + PackageName + "/" + filterName, "../" + filterName} {
		if _, err := os.Stat(dirname + "/" + filterName + ".json"); err == nil {
			return "file://" + dirname
		}
	}
	return "file://."
}

// NewFilter creates a new Filter of type "name"
func NewFilter(name string) (Filter, error) {
	filter, exists := filters[name]
	if !exists {
		return nil, fmt.Errorf("filters: Unknown filter %q", name)
	}
	return filter.New()
}
