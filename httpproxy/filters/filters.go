package filters

import (
	"context"
	"fmt"
	"net/http"
	"sync"
)

var (
	DummyRequest  *http.Request  = &http.Request{}
	DummyResponse *http.Response = &http.Response{}
)

type Filter interface {
	FilterName() string
}

type RequestFilter interface {
	Filter
	Request(context.Context, *http.Request) (context.Context, *http.Request, error)
}

type RoundTripFilter interface {
	Filter
	RoundTrip(context.Context, *http.Request) (context.Context, *http.Response, error)
}

type ResponseFilter interface {
	Filter
	Response(context.Context, *http.Response) (context.Context, *http.Response, error)
}

type RegisteredFilter struct {
	New func() (Filter, error)
}

var (
	registeredFilters map[string]*RegisteredFilter
	newedFilters      map[string]Filter
	muFilters         map[string]*sync.Mutex
)

func init() {
	registeredFilters = make(map[string]*RegisteredFilter)
	newedFilters = make(map[string]Filter)
	muFilters = make(map[string]*sync.Mutex)
}

// Register a Filter
func Register(name string, registeredFilter *RegisteredFilter) error {
	if _, exists := registeredFilters[name]; exists {
		return fmt.Errorf("Name already registered %s", name)
	}

	registeredFilters[name] = registeredFilter
	muFilters[name] = new(sync.Mutex)
	return nil
}

// GetFilter try get a existing Filter of type "name", otherwise create new one
func GetFilter(name string) (Filter, error) {
	muFilters[name].Lock()
	defer muFilters[name].Unlock()

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
