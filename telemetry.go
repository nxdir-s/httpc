package httpc

import (
	"context"
	"net/http"
	"net/http/httptrace"

	"go.opentelemetry.io/contrib/instrumentation/net/http/httptrace/otelhttptrace"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
)

// NewOTelTransport wraps the supplied round tripper with otel instrumentation
func NewOTelTransport(transport http.RoundTripper) http.RoundTripper {
	return otelhttp.NewTransport(
		transport,
		otelhttp.WithTracerProvider(otel.GetTracerProvider()),
		otelhttp.WithMeterProvider(otel.GetMeterProvider()),
		otelhttp.WithClientTrace(
			func(ctx context.Context) *httptrace.ClientTrace {
				return otelhttptrace.NewClientTrace(ctx,
					otelhttptrace.WithoutSubSpans(),
					otelhttptrace.WithTracerProvider(otel.GetTracerProvider()),
				)
			},
		),
	)
}
