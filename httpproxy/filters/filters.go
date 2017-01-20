package filters

import (
	"context"
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

var (
	mu  = new(sync.Mutex)
	mm  = make(map[string]*sync.Mutex)
	fnm = make(map[string]func() (Filter, error))
	fm  = make(map[string]Filter)
)

func Register(name string, New func() (Filter, error)) {
	mu.Lock()
	defer mu.Unlock()
	if _, ok := fnm[name]; !ok {
		fnm[name] = New
		fm[name] = nil
		mm[name] = new(sync.Mutex)
	}
}

func GetFilter(name string) (Filter, error) {
	if f, ok := fm[name]; ok && f != nil {
		return f, nil
	}

	mu := mm[name]
	mu.Lock()
	defer mu.Unlock()

	if f, ok := fm[name]; ok && f != nil {
		return f, nil
	}

	f, err := fnm[name]()
	if err != nil {
		return nil, err
	}

	fm[name] = f

	return f, nil

}
