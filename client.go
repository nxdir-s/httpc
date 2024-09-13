package httpc

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/throttled/throttled/v2"
	"github.com/throttled/throttled/v2/store/memstore"
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

type Config struct {
	TlsConfig      *tls.Config
	Timeout        int
	BaseUrl        string
	OTelEnabled    bool
	RetryEnabled   bool
	RetryMax       int
	RateLimit      int
	DefaultHeaders map[string]string
}

type Client struct {
	Http        *http.Client
	Credentials *clientcredentials.Config
	BaseUrl     *url.URL
	RateLimiter *throttled.GCRARateLimiterCtx
	Headers     map[string]string
}

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

	if cfg.RateLimit != 0 {
		store, err := memstore.NewCtx(MaxRateLimitKeys)
		if err != nil {
			return nil, err
		}

		quota := throttled.RateQuota{
			MaxRate: throttled.PerMin(cfg.RateLimit),
		}

		rateLimiter, err := throttled.NewGCRARateLimiterCtx(store, quota)
		if err != nil {
			return nil, err
		}

		client.RateLimiter = rateLimiter
	}

	for _, applyOption := range opts {
		applyOption(client)
	}

	return client, nil
}

func (c *Client) Get(ctx context.Context, resource string, headers map[string]string, decoded interface{}) (*http.Response, error) {
	pathUrl, err := url.ParseRequestURI(resource)
	if err != nil {
		return nil, InvalidResource{err}
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
		return nil, RequestError{err}
	}

	defer resp.Body.Close()

	if resp.StatusCode/10 != 20 {
		errBody := &bytes.Buffer{}
		resp.Write(errBody)

		return nil, BadStatusCode{errBody.String()}
	}

	if decoded != nil {
		err = json.NewDecoder(resp.Body).Decode(decoded)
		if err != nil {
			return nil, DecodeError{err}
		}
	}

	return resp, nil
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
		)
	}

	return transport, nil
}
