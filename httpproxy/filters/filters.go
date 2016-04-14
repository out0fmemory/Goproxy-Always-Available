package filters

import (
	"fmt"
	"net/http"
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
	registeredFilters map[string]*RegisteredFilter
	filters           map[string]Filter
)

func init() {
	registeredFilters = make(map[string]*RegisteredFilter)
}

// Register a Filter
func Register(name string, registeredFilter *RegisteredFilter) error {
	if _, exists := registeredFilters[name]; exists {
		return fmt.Errorf("Name already registered %s", name)
	}

	registeredFilters[name] = registeredFilter
	return nil
}

// NewFilter creates a new Filter of type "name"
func NewFilter(name string) (Filter, error) {
	filter, exists := registeredFilters[name]
	if !exists {
		return nil, fmt.Errorf("registeredFilters: Unknown filter %q", name)
	}
	return filter.New()
}

// GetFilter try get a existing Filter of type "name", otherwise create new one
func GetFilter(name string) (Filter, error) {
	filter, exists := filters[name]
	if exists {
		return filter, nil
	}
	return NewFilter(name)
}
