package httpc

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"os"
	"time"

	"github.com/throttled/throttled/v2"
	"github.com/throttled/throttled/v2/store/memstore"
	"go.opentelemetry.io/contrib/instrumentation/net/http/httptrace/otelhttptrace"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

type ErrStatusCode struct {
	code int
	msg  *bytes.Buffer
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

type ErrNewRequest struct {
	err error
}

func (e *ErrNewRequest) Error() string {
	return "error creating request: " + e.err.Error()
}

type ErrRequest struct {
	err error
}

func (e *ErrRequest) Error() string {
	return "error making HTTP request: " + e.err.Error()
}

type ErrRateLimit struct {
	err error
}

func (e *ErrRateLimit) Error() string {
	return "error checking rate limit: " + e.err.Error()
}

type ErrCopy struct {
	err error
}

func (e *ErrCopy) Error() string {
	return "error copying request body: " + e.err.Error()
}

const (
	_ int64 = 1 << (10 * iota)
	Kib
	Mib
	Gib
)

const (
	DefaultTimeout       int   = 10
	DefaultReadByteLimit int64 = 15 * Mib

	MaxRateLimitKeys int = 65536
	MaxIdleConns     int = 100
	MaxConnsPerHost  int = 100
)

type ClientOpt func(c *Client) error

// WithCustomClient replaces the default http client with the supplied one
func WithCustomClient(client *http.Client) ClientOpt {
	return func(c *Client) error {
		c.http = client
		return nil
	}
}

// WithDefaultHeaders adds default headers to the client
func WithDefaultHeaders(headers map[string]string) ClientOpt {
	return func(c *Client) error {
		c.headers = headers
		return nil
	}
}

// WithCredentials sets up oauth2 and replaces the default http client
func WithCredentials(ctx context.Context, id string, secret string, baseUrl string, resource string, scopes ...string) ClientOpt {
	return func(c *Client) error {
		baseUrl, err := url.ParseRequestURI(baseUrl)
		if err != nil {
			return err
		}

		authResource, err := url.ParseRequestURI(resource)
		if err != nil {
			return err
		}

		authUrl := baseUrl.ResolveReference(authResource)

		credentials := &clientcredentials.Config{
			ClientID:     id,
			ClientSecret: secret,
			TokenURL:     authUrl.String(),
		}

		credentials.Scopes = append(credentials.Scopes, scopes...)

		ctx = context.WithValue(ctx, oauth2.HTTPClient, c.http)
		c.http = credentials.Client(ctx)

		return nil
	}
}

// WithRateLimiter configures a rate limiter with the supplied limit (per minute)
func WithRateLimiter(rateLimit int) ClientOpt {
	return func(c *Client) error {
		store, err := memstore.NewCtx(MaxRateLimitKeys)
		if err != nil {
			return err
		}

		quota := throttled.RateQuota{
			MaxRate: throttled.PerMin(rateLimit),
		}

		rateLimiter, err := throttled.NewGCRARateLimiterCtx(store, quota)
		if err != nil {
			return err
		}

		c.rateLimiter = rateLimiter

		return nil
	}
}

type Config struct {
	TlsConfig     *tls.Config
	BaseUrl       string
	Timeout       int
	OTelEnabled   bool
	RetryEnabled  bool
	RetryLimit    int
	ReadByteLimit int64
}

type Client struct {
	http        *http.Client
	credentials *clientcredentials.Config
	baseUrl     *url.URL
	rateLimiter *throttled.GCRARateLimiterCtx
	headers     map[string]string
	limit       int64
}

// NewClient creates a new Client
func NewClient(ctx context.Context, cfg *Config, opts ...ClientOpt) (*Client, error) {
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
		baseUrl: baseUrl,
		http: &http.Client{
			Timeout:   time.Duration(timeout),
			Transport: httpTransport,
		},
		limit: cfg.ReadByteLimit,
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

	fullUrl := c.baseUrl.ResolveReference(pathUrl)

	if c.rateLimiter != nil {
		for {
			limited, context, err := c.rateLimiter.RateLimitCtx(ctx, c.baseUrl.String(), 1)
			if err != nil {
				return nil, &ErrRateLimit{err}
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
		return nil, &ErrNewRequest{err}
	}

	for key, val := range c.headers {
		req.Header.Set(key, val)
	}

	for key, val := range headers {
		req.Header.Set(key, val)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, &ErrRequest{err}
	}

	if resp.StatusCode/10 != 20 {
		defer resp.Body.Close()
		errBody := &bytes.Buffer{}

		var limit int64
		limit = int64(DefaultReadByteLimit)

		if c.limit != 0 {
			limit = c.limit
		}

		if _, err := io.Copy(errBody, io.LimitReader(resp.Body, limit)); err != nil {
			return nil, &ErrCopy{err}
		}

		return nil, &ErrStatusCode{resp.StatusCode, errBody}
	}

	return resp, nil
}

// Post makes a POST request to the supplied endpoint and returns the response. If a struct pointer is supplied, the response body will be decoded into it
func (c *Client) Post(ctx context.Context, resource string, body io.Reader, headers map[string]string, decoded interface{}) (*http.Response, error) {
	pathUrl, err := url.ParseRequestURI(resource)
	if err != nil {
		return nil, &ErrInvalidResource{err}
	}

	fullUrl := c.baseUrl.ResolveReference(pathUrl)

	if c.rateLimiter != nil {
		for {
			limited, context, err := c.rateLimiter.RateLimitCtx(ctx, c.baseUrl.String(), 1)
			if err != nil {
				return nil, &ErrRateLimit{err}
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
		return nil, &ErrNewRequest{err}
	}

	for key, val := range c.headers {
		req.Header.Set(key, val)
	}

	for key, val := range headers {
		req.Header.Set(key, val)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, &ErrRequest{err}
	}

	if resp.StatusCode/10 != 20 {
		defer resp.Body.Close()
		errBody := &bytes.Buffer{}

		var limit int64
		limit = int64(DefaultReadByteLimit)

		if c.limit != 0 {
			limit = c.limit
		}

		if _, err := io.Copy(errBody, io.LimitReader(resp.Body, limit)); err != nil {
			return nil, &ErrCopy{err}
		}

		return nil, &ErrStatusCode{resp.StatusCode, errBody}
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

	fullUrl := c.baseUrl.ResolveReference(pathUrl)

	if c.rateLimiter != nil {
		for {
			limited, context, err := c.rateLimiter.RateLimitCtx(ctx, c.baseUrl.String(), 1)
			if err != nil {
				return nil, &ErrRateLimit{err}
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
		return nil, &ErrNewRequest{err}
	}

	for key, val := range c.headers {
		req.Header.Set(key, val)
	}

	for key, val := range headers {
		req.Header.Set(key, val)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, &ErrRequest{err}
	}

	if resp.StatusCode/10 != 20 {
		defer resp.Body.Close()
		errBody := &bytes.Buffer{}

		var limit int64
		limit = int64(DefaultReadByteLimit)

		if c.limit != 0 {
			limit = c.limit
		}

		if _, err := io.Copy(errBody, io.LimitReader(resp.Body, limit)); err != nil {
			return nil, &ErrCopy{err}
		}

		return nil, &ErrStatusCode{resp.StatusCode, errBody}
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

	fullUrl := c.baseUrl.ResolveReference(pathUrl)

	if c.rateLimiter != nil {
		for {
			limited, context, err := c.rateLimiter.RateLimitCtx(ctx, c.baseUrl.String(), 1)
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

	for key, val := range c.headers {
		req.Header.Set(key, val)
	}

	for key, val := range headers {
		req.Header.Set(key, val)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, &ErrRequest{err}
	}

	if resp.StatusCode/10 != 20 {
		defer resp.Body.Close()
		errBody := &bytes.Buffer{}

		var limit int64
		limit = int64(DefaultReadByteLimit)

		if c.limit != 0 {
			limit = c.limit
		}

		if _, err := io.Copy(errBody, io.LimitReader(resp.Body, limit)); err != nil {
			return nil, &ErrCopy{err}
		}

		return nil, &ErrStatusCode{resp.StatusCode, errBody}
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

	fullUrl := c.baseUrl.ResolveReference(pathUrl)

	if c.rateLimiter != nil {
		for {
			limited, context, err := c.rateLimiter.RateLimitCtx(ctx, c.baseUrl.String(), 1)
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

	for key, val := range c.headers {
		req.Header.Set(key, val)
	}

	for key, val := range headers {
		req.Header.Set(key, val)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, &ErrRequest{err}
	}

	if resp.StatusCode/10 != 20 {
		defer resp.Body.Close()
		errBody := &bytes.Buffer{}

		var limit int64
		limit = int64(DefaultReadByteLimit)

		if c.limit != 0 {
			limit = c.limit
		}

		if _, err := io.Copy(errBody, io.LimitReader(resp.Body, limit)); err != nil {
			return nil, &ErrCopy{err}
		}

		return nil, &ErrStatusCode{resp.StatusCode, errBody}
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

	fullUrl := c.baseUrl.ResolveReference(pathUrl)

	if c.rateLimiter != nil {
		for {
			limited, context, err := c.rateLimiter.RateLimitCtx(ctx, c.baseUrl.String(), 1)
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

	for key, val := range c.headers {
		req.Header.Set(key, val)
	}

	for key, val := range headers {
		req.Header.Set(key, val)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, &ErrRequest{err}
	}

	if resp.StatusCode/10 != 20 {
		defer resp.Body.Close()
		errBody := &bytes.Buffer{}

		var limit int64
		limit = int64(DefaultReadByteLimit)

		if c.limit != 0 {
			limit = c.limit
		}

		if _, err := io.Copy(errBody, io.LimitReader(resp.Body, limit)); err != nil {
			return nil, &ErrCopy{err}
		}

		return nil, &ErrStatusCode{resp.StatusCode, errBody}
	}

	pr, pw := io.Pipe()

	go func() {
		defer resp.Body.Close()
		defer pw.Close()

		if _, err := io.Copy(pw, resp.Body); err != nil {
			fmt.Fprintf(os.Stdout, "failed to copy response body: %s\n", err.Error())
		}
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
		if transport, err = NewRetryTransport(defaultTransport, cfg.RetryLimit); err != nil {
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
