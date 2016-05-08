package filters

import (
	"context"
	"net"
	"net/http"
)

const (
	ContextRoundTripFilterKey = "context·RoundTripFilter"
	contextListenerKey        = "context·Listener"
	contextResponseWriterKey  = "context·ResponseWriter"
	contextHijackedKey        = "context·Hijacked"
)

func NewContext(ctx context.Context, ln net.Listener, rw http.ResponseWriter) context.Context {
	ctx = context.WithValue(ctx, contextListenerKey, ln)
	ctx = context.WithValue(ctx, contextResponseWriterKey, rw)
	ctx = context.WithValue(ctx, contextHijackedKey, false)
	return ctx
}

func GetListener(ctx context.Context) net.Listener {
	return ctx.Value(contextListenerKey).(net.Listener)
}

func GetResponseWriter(ctx context.Context) http.ResponseWriter {
	return ctx.Value(contextResponseWriterKey).(http.ResponseWriter)
}

func GetRoundTripFilter(ctx context.Context) RoundTripFilter {
	return ctx.Value(ContextRoundTripFilterKey).(RoundTripFilter)
}

func PutHijacked(ctx context.Context, hijacked bool) context.Context {
	return context.WithValue(ctx, contextHijackedKey, hijacked)
}

func GetHijacked(ctx context.Context) bool {
	v := ctx.Value(contextHijackedKey)
	if v == nil {
		return false
	}

	v1, ok := v.(bool)
	if !ok {
		return false
	}

	return v1
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
