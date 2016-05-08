package filters

import (
	"context"
	"net"
	"net/http"
)

func PutListener(ctx context.Context, ln net.Listener) context.Context {
	return context.WithValue(ctx, "context·Listener", ln)
}

func GetListener(ctx context.Context) net.Listener {
	return ctx.Value("context·Listener").(net.Listener)
}

func PutResponseWriter(ctx context.Context, rw http.ResponseWriter) context.Context {
	return context.WithValue(ctx, "context·ResponseWriter", rw)
}

func GetResponseWriter(ctx context.Context) http.ResponseWriter {
	return ctx.Value("context·ResponseWriter").(http.ResponseWriter)
}

func PutRoundTripFilter(ctx context.Context, filter RoundTripFilter) context.Context {
	return context.WithValue(ctx, "context·RoundTripFilter", filter)
}

func GetRoundTripFilter(ctx context.Context) RoundTripFilter {
	return ctx.Value("context·RoundTripFilter").(RoundTripFilter)
}

func PutHijacked(ctx context.Context, hijacked bool) context.Context {
	return context.WithValue(ctx, "context·Hijacked", hijacked)
}

func GetHijacked(ctx context.Context) bool {
	v := ctx.Value("context·Hijacked")
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
