package observability

import (
	"fmt"
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// HTTPMiddleware returns OpenTelemetry HTTP middleware
func HTTPMiddleware(provider *Provider, operation string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		// Use otelhttp with custom options
		handler := otelhttp.NewHandler(next, operation,
			otelhttp.WithTracerProvider(provider.TracerProvider),
			otelhttp.WithMeterProvider(provider.MeterProvider),
			otelhttp.WithSpanNameFormatter(func(operation string, r *http.Request) string {
				return fmt.Sprintf("%s %s", r.Method, r.URL.Path)
			}),
			otelhttp.WithSpanOptions(
				trace.WithAttributes(
					attribute.String("service.name", provider.config.ServiceName),
					attribute.String("service.version", provider.config.ServiceVersion),
				),
			),
		)
		
		return handler
	}
}

// InstrumentHTTPClient instruments an HTTP client with OpenTelemetry
func InstrumentHTTPClient(client *http.Client, provider *Provider) *http.Client {
	client.Transport = otelhttp.NewTransport(
		client.Transport,
		otelhttp.WithTracerProvider(provider.TracerProvider),
		otelhttp.WithMeterProvider(provider.MeterProvider),
	)
	return client
}