package filters

import (
	"context"
	"net"
	"net/http"
)

const (
	contextKey int = 0x3f71df90 // fmt.Sprintf("%x", md5.Sum([]byte("phuslu")))[:8]
)

type racer struct {
	h   http.Handler
	ln  net.Listener
	rw  http.ResponseWriter
	rtf RoundTripFilter
	b   string
}

func NewContext(ctx context.Context, h http.Handler, ln net.Listener, rw http.ResponseWriter, brand string) context.Context {
	return context.WithValue(ctx, contextKey, &racer{h, ln, rw, nil, brand})
}

func GetHandler(ctx context.Context) http.Handler {
	return ctx.Value(contextKey).(*racer).h
}

func GetListener(ctx context.Context) net.Listener {
	return ctx.Value(contextKey).(*racer).ln
}

func GetResponseWriter(ctx context.Context) http.ResponseWriter {
	return ctx.Value(contextKey).(*racer).rw
}

func GetRoundTripFilter(ctx context.Context) RoundTripFilter {
	return ctx.Value(contextKey).(*racer).rtf
}

func GetBranding(ctx context.Context) string {
	return ctx.Value(contextKey).(*racer).b
}

func SetRoundTripFilter(ctx context.Context, filter RoundTripFilter) {
	ctx.Value(contextKey).(*racer).rtf = filter
}

func WithString(ctx context.Context, name, value string) context.Context {
	return context.WithValue(ctx, name, value)
}

func String(ctx context.Context, name string) string {
	value := ctx.Value(name)
	if value == nil {
		return ""
	}

	s, ok := value.(string)
	if !ok {
		return ""
	}

	return s
}

func WithBool(ctx context.Context, name string, value bool) context.Context {
	return context.WithValue(ctx, name, value)
}

func Bool(ctx context.Context, name string) (bool, bool) {
	value := ctx.Value(name)
	if value == nil {
		return false, false
	}

	v, ok := value.(bool)
	if !ok {
		return false, false
	}

	return v, true
}
