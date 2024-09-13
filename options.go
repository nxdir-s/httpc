package httpc

import "net/http"

type ClientOption func(c *Client)

// WithCustomClient replaces the default http client with the supplied one
func WithCustomClient(client *http.Client) ClientOption {
	return func(c *Client) {
		c.Http = client
	}
}

// WithDefaultHeaders adds default headers to the client
func WithDefaultHeaders(headers map[string]string) ClientOption {
	return func(c *Client) {
		c.Headers = headers
	}
}
