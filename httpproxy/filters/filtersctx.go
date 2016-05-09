package filters

import (
	"context"
	"net"
	"net/http"
)

const (
	ContextKey = "context·"
)

type racer struct {
	ln  net.Listener
	rw  http.ResponseWriter
	rtf RoundTripFilter
	hj  bool
}

func NewContext(ctx context.Context, ln net.Listener, rw http.ResponseWriter) context.Context {
	return context.WithValue(ctx, ContextKey, &racer{ln, rw, nil, false})
}

func GetListener(ctx context.Context) net.Listener {
	return ctx.Value(ContextKey).(*racer).ln
}

func GetResponseWriter(ctx context.Context) http.ResponseWriter {
	return ctx.Value(ContextKey).(*racer).rw
}

func GetRoundTripFilter(ctx context.Context) RoundTripFilter {
	return ctx.Value(ContextKey).(*racer).rtf
}

func GetHijacked(ctx context.Context) bool {
	return ctx.Value(ContextKey).(*racer).hj
}

func SetRoundTripFilter(ctx context.Context, filter RoundTripFilter) {
	ctx.Value(ContextKey).(*racer).rtf = filter
}

func SetHijacked(ctx context.Context, hijacked bool) {
	ctx.Value(ContextKey).(*racer).hj = hijacked
}

func WithString(ctx context.Context, name, value string) context.Context {
	return context.WithValue(ctx, "string·"+name, value)
}

func String(ctx context.Context, name string) string {
	value := ctx.Value("string·" + name)
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
	return context.WithValue(ctx, "bool·"+name, value)
}

func Bool(ctx context.Context, name string) (bool, bool) {
	value := ctx.Value("bool·" + name)
	if value == nil {
		return false, false
	}

	v, ok := value.(bool)
	if !ok {
		return false, false
	}

	return v, true
}
