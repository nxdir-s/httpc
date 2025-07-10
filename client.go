package httpc

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"time"

	"github.com/throttled/throttled/v2"
	"go.opentelemetry.io/contrib/instrumentation/net/http/httptrace/otelhttptrace"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"golang.org/x/oauth2/clientcredentials"
)

const (
	DefaultTimeout   int = 10
	MaxRateLimitKeys int = 65536
	MaxIdleConns     int = 100
	MaxConnsPerHost  int = 100
)

type ErrStatusCode struct {
	msg *bytes.Buffer
}

func (e *ErrStatusCode) Error() string {
	return "recieved bad status code: " + e.msg.String()
}

type ErrInvalidResource struct {
	err error
}

func (e *ErrInvalidResource) Error() string {
	return "error parsing resource: " + e.err.Error()
}

type ErrDecode struct {
	err error
}

func (e *ErrDecode) Error() string {
	return "failed to decode response body: " + e.err.Error()
}

type ErrRequest struct {
	err error
}

func (e *ErrRequest) Error() string {
	return "error making HTTP request: " + e.err.Error()
}

type Config struct {
	TlsConfig    *tls.Config
	BaseUrl      string
	Timeout      int
	OTelEnabled  bool
	RetryEnabled bool
	RetryMax     int
}

type Client struct {
	Http        *http.Client
	Credentials *clientcredentials.Config
	BaseUrl     *url.URL
	RateLimiter *throttled.GCRARateLimiterCtx
	Headers     map[string]string
}

// NewClient creates a new Client
func NewClient(ctx context.Context, cfg *Config, opts ...ClientOption) (*Client, error) {
	baseUrl, err := url.ParseRequestURI(cfg.BaseUrl)
	if err != nil {
		return nil, err
	}

	timeout := DefaultTimeout * int(time.Second)
	if cfg.Timeout != 0 {
		timeout = cfg.Timeout
	}

	var httpTransport http.RoundTripper
	httpTransport, err = getRoundTripper(cfg, timeout)
	if err != nil {
		return nil, err
	}

	client := &Client{
		BaseUrl: baseUrl,
		Http: &http.Client{
			Timeout:   time.Duration(timeout),
			Transport: httpTransport,
		},
	}

	for _, opt := range opts {
		if err := opt(client); err != nil {
			return nil, err
		}
	}

	return client, nil
}

// Get makes a GET request to the supplied endpoint and returns the response.
func (c *Client) Get(ctx context.Context, resource string, headers map[string]string) (*http.Response, error) {
	pathUrl, err := url.ParseRequestURI(resource)
	if err != nil {
		return nil, &ErrInvalidResource{err}
	}

	fullUrl := c.BaseUrl.ResolveReference(pathUrl)

	if c.RateLimiter != nil {
		for {
			limited, context, err := c.RateLimiter.RateLimitCtx(ctx, c.BaseUrl.String(), 1)
			if err != nil {
				return nil, err
			}

			if limited {
				time.Sleep(context.RetryAfter)
				continue
			}

			break
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullUrl.String(), nil)
	if err != nil {
		return nil, err
	}

	for key, val := range c.Headers {
		req.Header.Set(key, val)
	}

	for key, val := range headers {
		req.Header.Set(key, val)
	}

	resp, err := c.Http.Do(req)
	if err != nil {
		return nil, &ErrRequest{err}
	}

	if resp.StatusCode/10 != 20 {
		errBody := &bytes.Buffer{}

		resp.Write(errBody)
		resp.Body.Close()

		return nil, &ErrStatusCode{errBody}
	}

	return resp, nil
}

// Post makes a POST request to the supplied endpoint and returns the response. If a struct pointer is supplied, the response body will be decoded into it
func (c *Client) Post(ctx context.Context, resource string, body io.Reader, headers map[string]string, decoded interface{}) (*http.Response, error) {
	pathUrl, err := url.ParseRequestURI(resource)
	if err != nil {
		return nil, &ErrInvalidResource{err}
	}

	fullUrl := c.BaseUrl.ResolveReference(pathUrl)

	if c.RateLimiter != nil {
		for {
			limited, context, err := c.RateLimiter.RateLimitCtx(ctx, c.BaseUrl.String(), 1)
			if err != nil {
				return nil, err
			}

			if limited {
				time.Sleep(context.RetryAfter)
				continue
			}

			break
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fullUrl.String(), body)
	if err != nil {
		return nil, err
	}

	for key, val := range c.Headers {
		req.Header.Set(key, val)
	}

	for key, val := range headers {
		req.Header.Set(key, val)
	}

	resp, err := c.Http.Do(req)
	if err != nil {
		return nil, &ErrRequest{err}
	}

	if resp.StatusCode/10 != 20 {
		errBody := &bytes.Buffer{}

		resp.Write(errBody)
		resp.Body.Close()

		return nil, &ErrStatusCode{errBody}
	}

	if decoded != nil {
		if err := json.NewDecoder(resp.Body).Decode(decoded); err != nil {
			resp.Body.Close()
			return nil, &ErrDecode{err}
		}
	}

	return resp, nil
}

// Put makes a PUT request to the supplied endpoint and returns the response. If a struct pointer is supplied, the response body will be decoded into it
func (c *Client) Put(ctx context.Context, resource string, body io.Reader, headers map[string]string, decoded interface{}) (*http.Response, error) {
	pathUrl, err := url.ParseRequestURI(resource)
	if err != nil {
		return nil, &ErrInvalidResource{err}
	}

	fullUrl := c.BaseUrl.ResolveReference(pathUrl)

	if c.RateLimiter != nil {
		for {
			limited, context, err := c.RateLimiter.RateLimitCtx(ctx, c.BaseUrl.String(), 1)
			if err != nil {
				return nil, err
			}

			if limited {
				time.Sleep(context.RetryAfter)
				continue
			}

			break
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, fullUrl.String(), body)
	if err != nil {
		return nil, err
	}

	for key, val := range c.Headers {
		req.Header.Set(key, val)
	}

	for key, val := range headers {
		req.Header.Set(key, val)
	}

	resp, err := c.Http.Do(req)
	if err != nil {
		return nil, &ErrRequest{err}
	}

	if resp.StatusCode/10 != 20 {
		errBody := &bytes.Buffer{}

		resp.Write(errBody)
		resp.Body.Close()

		return nil, &ErrStatusCode{errBody}
	}

	if decoded != nil {
		if err := json.NewDecoder(resp.Body).Decode(decoded); err != nil {
			resp.Body.Close()
			return nil, &ErrDecode{err}
		}
	}

	return resp, nil
}

// Delete makes a DELETE request to the supplied endpoint and returns the response. If a struct pointer is supplied, the response body will be decoded into it
func (c *Client) Delete(ctx context.Context, resource string, body io.Reader, headers map[string]string, decoded interface{}) (*http.Response, error) {
	pathUrl, err := url.ParseRequestURI(resource)
	if err != nil {
		return nil, &ErrInvalidResource{err}
	}

	fullUrl := c.BaseUrl.ResolveReference(pathUrl)

	if c.RateLimiter != nil {
		for {
			limited, context, err := c.RateLimiter.RateLimitCtx(ctx, c.BaseUrl.String(), 1)
			if err != nil {
				return nil, err
			}

			if limited {
				time.Sleep(context.RetryAfter)
				continue
			}

			break
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, fullUrl.String(), body)
	if err != nil {
		return nil, err
	}

	for key, val := range c.Headers {
		req.Header.Set(key, val)
	}

	for key, val := range headers {
		req.Header.Set(key, val)
	}

	resp, err := c.Http.Do(req)
	if err != nil {
		return nil, &ErrRequest{err}
	}

	if resp.StatusCode/10 != 20 {
		errBody := &bytes.Buffer{}

		resp.Write(errBody)
		resp.Body.Close()

		return nil, &ErrStatusCode{errBody}
	}

	if decoded != nil {
		if err := json.NewDecoder(resp.Body).Decode(decoded); err != nil {
			resp.Body.Close()
			return nil, &ErrDecode{err}
		}
	}

	return resp, nil
}

// Patch makes a PATCH request to the supplied endpoint and returns the response. If a struct pointer is supplied, the response body will be decoded into it
func (c *Client) Patch(ctx context.Context, resource string, body io.Reader, headers map[string]string, decoded interface{}) (*http.Response, error) {
	pathUrl, err := url.ParseRequestURI(resource)
	if err != nil {
		return nil, &ErrInvalidResource{err}
	}

	fullUrl := c.BaseUrl.ResolveReference(pathUrl)

	if c.RateLimiter != nil {
		for {
			limited, context, err := c.RateLimiter.RateLimitCtx(ctx, c.BaseUrl.String(), 1)
			if err != nil {
				return nil, err
			}

			if limited {
				time.Sleep(context.RetryAfter)
				continue
			}

			break
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, fullUrl.String(), body)
	if err != nil {
		return nil, err
	}

	for key, val := range c.Headers {
		req.Header.Set(key, val)
	}

	for key, val := range headers {
		req.Header.Set(key, val)
	}

	resp, err := c.Http.Do(req)
	if err != nil {
		return nil, &ErrRequest{err}
	}
	defer resp.Body.Close()

	if resp.StatusCode/10 != 20 {
		errBody := &bytes.Buffer{}

		resp.Write(errBody)
		resp.Body.Close()

		return nil, &ErrStatusCode{errBody}
	}

	if decoded != nil {
		if err := json.NewDecoder(resp.Body).Decode(decoded); err != nil {
			resp.Body.Close()
			return nil, &ErrDecode{err}
		}
	}

	return resp, nil
}

// Stream makes a request to the supplied endpoint and pipes the response body to the returned io.Reader
func (c *Client) Stream(ctx context.Context, method string, resource string, body io.Reader, headers map[string]string) (io.Reader, error) {
	pathUrl, err := url.ParseRequestURI(resource)
	if err != nil {
		return nil, &ErrInvalidResource{err}
	}

	fullUrl := c.BaseUrl.ResolveReference(pathUrl)

	if c.RateLimiter != nil {
		for {
			limited, context, err := c.RateLimiter.RateLimitCtx(ctx, c.BaseUrl.String(), 1)
			if err != nil {
				return nil, err
			}

			if limited {
				time.Sleep(context.RetryAfter)
				continue
			}

			break
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, fullUrl.String(), body)
	if err != nil {
		return nil, err
	}

	for key, val := range c.Headers {
		req.Header.Set(key, val)
	}

	for key, val := range headers {
		req.Header.Set(key, val)
	}

	resp, err := c.Http.Do(req)
	if err != nil {
		return nil, &ErrRequest{err}
	}

	if resp.StatusCode/10 != 20 {
		errBody := &bytes.Buffer{}

		resp.Write(errBody)
		resp.Body.Close()

		return nil, &ErrStatusCode{errBody}
	}

	pr, pw := io.Pipe()

	go func() {
		defer resp.Body.Close()
		defer pw.Close()

		io.Copy(pw, resp.Body)
	}()

	return pr, nil
}

func getRoundTripper(cfg *Config, timeout int) (http.RoundTripper, error) {
	var transport http.RoundTripper
	var err error

	defaultTransport := &http.Transport{
		Dial: (&net.Dialer{
			Timeout: time.Duration(timeout),
		}).Dial,
		TLSClientConfig:     cfg.TlsConfig,
		MaxIdleConns:        MaxIdleConns,
		MaxConnsPerHost:     MaxConnsPerHost,
		MaxIdleConnsPerHost: MaxConnsPerHost,
		IdleConnTimeout:     time.Duration(timeout),
		TLSHandshakeTimeout: time.Duration(timeout),
	}

	transport = defaultTransport

	if cfg.RetryEnabled {
		transport, err = NewRetryTransport(defaultTransport, cfg.RetryMax)
		if err != nil {
			return nil, err
		}
	}

	if cfg.OTelEnabled {
		transport = otelhttp.NewTransport(
			transport,
			otelhttp.WithTracerProvider(otel.GetTracerProvider()),
			otelhttp.WithMeterProvider(otel.GetMeterProvider()),
			otelhttp.WithClientTrace(func(ctx context.Context) *httptrace.ClientTrace {
				return otelhttptrace.NewClientTrace(ctx,
					otelhttptrace.WithoutSubSpans(),
					otelhttptrace.WithTracerProvider(otel.GetTracerProvider()),
				)
			}),
		)
	}

	return transport, nil
}
