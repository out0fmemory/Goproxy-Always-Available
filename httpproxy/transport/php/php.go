package php

import (
	"fmt"
	"net/http"
)

type Transport struct {
	http.RoundTripper
	Server
}

func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	req1, err := t.Server.encodeRequest(req)
	if err != nil {
		return nil, fmt.Errorf("PHP encodeRequest: %s", err.Error())
	}

	res, err := t.RoundTripper.RoundTrip(req1)
	if err != nil {
		return nil, err
	}

	resp, err := t.Server.decodeResponse(res)
	return resp, err
}
