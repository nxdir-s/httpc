package httpc

import (
	"context"
	"net/http"
	"net/url"

	"github.com/throttled/throttled/v2"
	"github.com/throttled/throttled/v2/store/memstore"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

type ClientOption func(c *Client) error

// WithCustomClient replaces the default http client with the supplied one
func WithCustomClient(client *http.Client) ClientOption {
	return func(c *Client) error {
		c.Http = client
		return nil
	}
}

// WithDefaultHeaders adds default headers to the client
func WithDefaultHeaders(headers map[string]string) ClientOption {
	return func(c *Client) error {
		c.Headers = headers
		return nil
	}
}

// WithCredentials sets up oauth2 and replaces the default http client
func WithCredentials(ctx context.Context, clientId, key, baseUrl, resource string) ClientOption {
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
			ClientID:     clientId,
			ClientSecret: key,
			TokenURL:     authUrl.String(),
		}

		ctx = context.WithValue(ctx, oauth2.HTTPClient, c.Http)
		c.Http = credentials.Client(ctx)

		return nil
	}
}

// WithRateLimiter configures a rate limiter with the supplied limit (per minute)
func WithRateLimiter(rateLimit int) ClientOption {
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

		c.RateLimiter = rateLimiter

		return nil
	}
}
