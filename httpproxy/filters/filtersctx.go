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

func SetRoundTripFilter(ctx context.Context, filter RoundTripFilter) context.Context {
	ctx.Value(ContextKey).(*racer).rtf = filter
	return ctx
}

func GetHijacked(ctx context.Context) bool {
	return ctx.Value(ContextKey).(*racer).hj
}

func SetHijacked(ctx context.Context, hijacked bool) context.Context {
	ctx.Value(ContextKey).(*racer).hj = hijacked
	return ctx
}

func PutString(ctx context.Context, name, value string) context.Context {
	return context.WithValue(ctx, "string·"+name, value)
}

func GetString(ctx context.Context, name string) string {
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

func PutBool(ctx context.Context, name string, value bool) context.Context {
	return context.WithValue(ctx, "bool·"+name, value)
}

func GetBool(ctx context.Context, name string) (bool, bool) {
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
