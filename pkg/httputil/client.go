package httputil

import (
	"net/http"
)

// NewClient creates a new HTTP client that respects proxy environment variables
// (HTTP_PROXY, HTTPS_PROXY, NO_PROXY).
func NewClient() *http.Client {
	return &http.Client{
		Transport: NewTransport(),
	}
}

// NewTransport creates a new HTTP transport that respects proxy environment variables.
// This is a convenience function that returns http.DefaultTransport, which already
// supports proxy configuration via environment variables.
func NewTransport() http.RoundTripper {
	return http.DefaultTransport
}
