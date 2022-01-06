package distro

import (
	"fmt"
	"net/http"
)

type transport struct {
	token               string
	underlyingTransport http.RoundTripper
}

func (t *transport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Add("Authorization", fmt.Sprintf("token %s", t.token))
	return t.underlyingTransport.RoundTrip(req)
}
