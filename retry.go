package httpc

import (
	"bytes"
	"io"
	"math"
	"net/http"
	"time"
)

const (
	DefaultRetryLimit int = 3
)

type ErrRetryCopy struct {
	err error
}

func (e *ErrRetryCopy) Error() string {
	return "error copying request body for retry: " + e.err.Error()
}

type RetryTransport struct {
	transport http.RoundTripper
	retryMax  int
}

// NewRetryTransport wraps the supplied http transport with a retryable implementation
func NewRetryTransport(transport *http.Transport, limit int) *RetryTransport {
	var retryLimit int
	retryLimit = DefaultRetryLimit

	if limit != 0 {
		retryLimit = limit
	}

	return &RetryTransport{
		transport: transport,
		retryMax:  retryLimit,
	}
}

// RoundTrip implements the http.RoundTripper interface with retries
func (t *RetryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var bodyBytes []byte
	var err error

	if req.Body != nil {
		bodyBytes, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, &ErrRetryCopy{err}
		}

		req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	}

	resp, err := t.transport.RoundTrip(req)

	retries := 0
	for shouldRetry(resp, err) && retries < t.retryMax {
		time.Sleep(backoff(retries))

		// drain response body to reuse connection
		if resp.Body != nil {
			drainBody(resp.Body)
		}

		if req.Body != nil {
			req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		}

		resp, err = t.transport.RoundTrip(req)

		retries++
	}

	return resp, err
}

func drainBody(body io.ReadCloser) error {
	defer body.Close()

	if _, err := io.ReadAll(body); err != nil {
		return err
	}

	return nil
}

// shouldRetry checks for errors and non 2XX status codes to determine whether to retry
func shouldRetry(resp *http.Response, err error) bool {
	if err != nil {
		return true
	}

	if resp.StatusCode/10 != 20 {
		return true
	}

	return false
}

// backoff doubles the delay
func backoff(retries int) time.Duration {
	return time.Duration(math.Pow(2, float64(retries))) * time.Second
}
