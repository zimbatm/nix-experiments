package observability

import (
	"context"
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	promexporter "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Config holds observability configuration
type Config struct {
	ServiceName     string
	ServiceVersion  string
	Environment     string
	OTLPEndpoint    string // OTLP gRPC endpoint for traces
	EnableTracing   bool
	EnableMetrics   bool
	PrometheusPort  int
}

// Provider holds the observability providers
type Provider struct {
	TracerProvider  trace.TracerProvider
	MeterProvider   metric.MeterProvider
	Tracer          trace.Tracer
	Meter           metric.Meter
	config          Config
	promRegistry    *prometheus.Registry
}

// Setup initializes OpenTelemetry providers
func Setup(ctx context.Context, cfg Config) (*Provider, error) {
	// Create resource
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.ServiceVersion),
			attribute.String("environment", cfg.Environment),
		),
		resource.WithHost(),
		resource.WithProcess(),
		resource.WithOS(),
		resource.WithContainer(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	p := &Provider{config: cfg}

	// Setup tracing
	if cfg.EnableTracing && cfg.OTLPEndpoint != "" {
		tp, err := setupTracing(ctx, res, cfg.OTLPEndpoint)
		if err != nil {
			return nil, fmt.Errorf("failed to setup tracing: %w", err)
		}
		p.TracerProvider = tp
		p.Tracer = tp.Tracer(cfg.ServiceName)
		otel.SetTracerProvider(tp)
	} else {
		// Use noop provider
		p.TracerProvider = otel.GetTracerProvider()
		p.Tracer = p.TracerProvider.Tracer(cfg.ServiceName)
	}

	// Setup metrics
	if cfg.EnableMetrics {
		mp, reg, err := setupMetrics(res)
		if err != nil {
			return nil, fmt.Errorf("failed to setup metrics: %w", err)
		}
		p.MeterProvider = mp
		p.Meter = mp.Meter(cfg.ServiceName)
		p.promRegistry = reg
		otel.SetMeterProvider(mp)
	} else {
		// Use noop provider
		p.MeterProvider = otel.GetMeterProvider()
		p.Meter = p.MeterProvider.Meter(cfg.ServiceName)
	}

	// Set global propagator
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return p, nil
}

// setupTracing configures the OpenTelemetry trace provider
func setupTracing(ctx context.Context, res *resource.Resource, endpoint string) (*sdktrace.TracerProvider, error) {
	// Create OTLP trace exporter
	conn, err := grpc.DialContext(ctx, endpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC connection: %w", err)
	}

	exporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithGRPCConn(conn))
	if err != nil {
		return nil, fmt.Errorf("failed to create trace exporter: %w", err)
	}

	// Create trace provider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)

	return tp, nil
}

// setupMetrics configures the OpenTelemetry metrics provider
func setupMetrics(res *resource.Resource) (*metric.MeterProvider, *prometheus.Registry, error) {
	// Create Prometheus registry
	reg := prometheus.NewRegistry()

	// Create Prometheus exporter
	exporter, err := promexporter.New(
		promexporter.WithRegisterer(reg),
		promexporter.WithoutTargetInfo(),
		promexporter.WithoutScopeInfo(),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create prometheus exporter: %w", err)
	}

	// Create meter provider
	mp := metric.NewMeterProvider(
		metric.WithResource(res),
		metric.WithReader(exporter),
	)

	return mp, reg, nil
}

// Shutdown gracefully shuts down the providers
func (p *Provider) Shutdown(ctx context.Context) error {
	// Shutdown tracer provider
	if tp, ok := p.TracerProvider.(*sdktrace.TracerProvider); ok && tp != nil {
		if err := tp.Shutdown(ctx); err != nil {
			return fmt.Errorf("failed to shutdown tracer provider: %w", err)
		}
	}

	// Shutdown meter provider
	if mp, ok := p.MeterProvider.(*metric.MeterProvider); ok && mp != nil {
		if err := mp.Shutdown(ctx); err != nil {
			return fmt.Errorf("failed to shutdown meter provider: %w", err)
		}
	}

	return nil
}

// PrometheusRegistry returns the Prometheus registry for metric serving
func (p *Provider) PrometheusRegistry() *prometheus.Registry {
	return p.promRegistry
}

// StartSpan starts a new span with common attributes
func (p *Provider) StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return p.Tracer.Start(ctx, name, opts...)
}

// RecordDuration records a duration metric
func (p *Provider) RecordDuration(ctx context.Context, name string, duration time.Duration, attrs ...attribute.KeyValue) {
	if histogram, err := p.Meter.Float64Histogram(name); err == nil {
		histogram.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))
	}
}

// IncrementCounter increments a counter metric
func (p *Provider) IncrementCounter(ctx context.Context, name string, attrs ...attribute.KeyValue) {
	if counter, err := p.Meter.Int64Counter(name); err == nil {
		counter.Add(ctx, 1, metric.WithAttributes(attrs...))
	}
}

// SetGauge sets a gauge value
func (p *Provider) SetGauge(ctx context.Context, name string, value float64, attrs ...attribute.KeyValue) {
	if gauge, err := p.Meter.Float64Gauge(name); err == nil {
		gauge.Record(ctx, value, metric.WithAttributes(attrs...))
	}
}