package filters

import (
	"context"
	"fmt"
	"net/http"
)

type Filter interface {
	FilterName() string
}

type RequestFilter interface {
	FilterName() string
	Request(context.Context, *http.Request) (context.Context, *http.Request, error)
}

type RoundTripFilter interface {
	FilterName() string
	RoundTrip(context.Context, *http.Request) (context.Context, *http.Response, error)
}

type ResponseFilter interface {
	FilterName() string
	Response(context.Context, *http.Response) (context.Context, *http.Response, error)
}

type RegisteredFilter struct {
	New func() (Filter, error)
}

var (
	registeredFilters map[string]*RegisteredFilter
	newedFilters      map[string]Filter
)

func init() {
	registeredFilters = make(map[string]*RegisteredFilter)
	newedFilters = make(map[string]Filter)
}

// Register a Filter
func Register(name string, registeredFilter *RegisteredFilter) error {
	if _, exists := registeredFilters[name]; exists {
		return fmt.Errorf("Name already registered %s", name)
	}

	registeredFilters[name] = registeredFilter
	return nil
}

// GetFilter try get a existing Filter of type "name", otherwise create new one
func GetFilter(name string) (Filter, error) {
	if f, ok := newedFilters[name]; ok {
		return f, nil
	}

	filterNew, ok := registeredFilters[name]
	if !ok {
		return nil, fmt.Errorf("registeredFilters: Unknown filter %q", name)
	}

	filter, err := filterNew.New()
	if err != nil {
		return nil, err
	}

	newedFilters[name] = filter

	return filter, nil
}
