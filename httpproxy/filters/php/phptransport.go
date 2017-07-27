package php

import (
	"fmt"
	"math/rand"
	"net/http"

	"../../helpers"
)

type Transport struct {
	http.RoundTripper
	Servers []Server
}

func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	i := 0

	if helpers.IsStaticRequest(req) {
		i = rand.Intn(len(t.Servers))
	}

	server := t.Servers[i]

	req1, err := server.encodeRequest(req)
	if err != nil {
		return nil, fmt.Errorf("PHP encodeRequest: %s", err.Error())
	}

	res, err := t.RoundTripper.RoundTrip(req1)
	if err != nil {
		return nil, err
	}

	resp, err := server.decodeResponse(res)
	return resp, err
}
