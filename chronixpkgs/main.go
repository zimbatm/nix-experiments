package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/zimbatm/nix-experiments/chronixpkgs/internal/fetcher"
	"github.com/zimbatm/nix-experiments/chronixpkgs/internal/logger"
	"github.com/zimbatm/nix-experiments/chronixpkgs/internal/observability"
	"github.com/zimbatm/nix-experiments/chronixpkgs/internal/server"
	"github.com/zimbatm/nix-experiments/chronixpkgs/internal/storage"
	"github.com/zimbatm/nix-experiments/chronixpkgs/internal/utils"
	"github.com/zimbatm/nix-experiments/chronixpkgs/internal/webhook"
)

func run() error {
	// Essential flags
	var (
		githubToken     = flag.String("github-token", "", "GitHub API token (or use GITHUB_TOKEN env var)")
		webhookSecret   = flag.String("webhook-secret", "", "GitHub webhook secret (or use WEBHOOK_SECRET env var)")
		repo            = flag.String("repo", "NixOS/nixpkgs", "GitHub repository to monitor (owner/repo)")
		
		// Server options
		listen          = flag.String("listen", ":8080", "HTTP server listen address")
		readTimeout     = flag.Duration("read-timeout", 30*time.Second, "HTTP server read timeout")
		writeTimeout    = flag.Duration("write-timeout", 30*time.Second, "HTTP server write timeout")
		shutdownTimeout = flag.Duration("shutdown-timeout", 10*time.Second, "Server graceful shutdown timeout")
		
		// Storage options
		dataDir         = flag.String("data-dir", "./data", "Data directory for storage")
		retentionDays   = flag.Int("retention-days", 90, "Days to retain events")
		vacuumInterval  = flag.Duration("vacuum-interval", 24*time.Hour, "Interval for vacuum operations")
		
		// API options
		defaultLimit    = flag.Int("default-limit", 100, "Default event query limit")
		maxLimit        = flag.Int("max-limit", 1000, "Maximum event query limit")
		sseInterval     = flag.Duration("sse-interval", 10*time.Second, "SSE polling interval")
		
		// Optional features
		pollInterval    = flag.Duration("poll-interval", 5*time.Minute, "Polling interval for GitHub events")
		enableCORS      = flag.Bool("enable-cors", false, "Enable CORS")
		corsOrigins     = flag.String("cors-origins", "*", "Comma-separated list of allowed CORS origins")
		enableRateLimit = flag.Bool("enable-rate-limit", false, "Enable rate limiting")
		rateLimit       = flag.Int("rate-limit", 60, "Requests per minute per IP")
		rateLimitBurst  = flag.Int("rate-limit-burst", 10, "Rate limit burst size")
		
		// Logging options
		logLevel        = flag.String("log-level", "info", "Log level (debug, info, warn, error)")
		logFormat       = flag.String("log-format", "text", "Log format (text or json)")
		
		// Observability options
		enableOtel      = flag.Bool("enable-otel", false, "Enable OpenTelemetry")
		otelEndpoint    = flag.String("otel-endpoint", "localhost:4317", "OpenTelemetry OTLP gRPC endpoint")
		metricsPort     = flag.Int("metrics-port", 9090, "Prometheus metrics port")
	)
	
	flag.Parse()
	
	// Initialize logger
	log := logger.New(logger.Config{
		Level:  logger.LogLevel(*logLevel),
		Format: *logFormat,
	})
	
	// Initialize observability
	var obs *observability.Provider
	if *enableOtel {
		var err error
		obs, err = observability.Setup(context.Background(), observability.Config{
			ServiceName:    "chronixpkgs",
			ServiceVersion: "1.0.0",
			Environment:    "production",
			OTLPEndpoint:   *otelEndpoint,
			EnableTracing:  true,
			EnableMetrics:  true,
			PrometheusPort: *metricsPort,
		})
		if err != nil {
			log.WithError(err).Error("Failed to setup observability")
		} else {
			defer obs.Shutdown(context.Background())
			log.Info("OpenTelemetry initialized", "endpoint", *otelEndpoint)
		}
	}
	
	// Get token from env if not provided via flag
	if *githubToken == "" {
		*githubToken = os.Getenv("GITHUB_TOKEN")
	}
	if *githubToken == "" {
		return fmt.Errorf("GitHub token is required (use -github-token or GITHUB_TOKEN env var)")
	}
	
	// Get webhook secret from env if not provided via flag
	if *webhookSecret == "" {
		*webhookSecret = os.Getenv("WEBHOOK_SECRET")
	}
	if *webhookSecret == "" {
		return fmt.Errorf("webhook secret is required (use -webhook-secret or WEBHOOK_SECRET env var)")
	}
	
	// Normalize and validate repository format
	normalizedRepo := utils.NormalizeRepository(*repo)
	if err := utils.ValidateRepository(normalizedRepo); err != nil {
		return fmt.Errorf("invalid repository format: %w", err)
	}

	// Initialize storage
	store, err := storage.NewSQLiteStore(*dataDir)
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}
	defer store.Close()

	// Initialize fetcher
	eventFetcher, err := fetcher.NewGitHubFetcher(*githubToken, store)
	if err != nil {
		return fmt.Errorf("failed to initialize fetcher: %w", err)
	}
	eventFetcher.SetRepository(normalizedRepo)
	eventFetcher.SetPollInterval(*pollInterval)

	// Initialize webhook handler
	webhookHandler := webhook.NewHandler(*webhookSecret, eventFetcher)

	// Initialize HTTP server
	srv := server.NewServer(store, webhookHandler)
	srv.SetMonitoredRepo(normalizedRepo)
	srv.SetAPILimits(*defaultLimit, *maxLimit)
	srv.SetSSEInterval(*sseInterval)
	srv.SetLogger(log)
	if obs != nil {
		srv.SetObservability(obs)
	}
	
	// Configure CORS if enabled
	if *enableCORS {
		origins := strings.Split(*corsOrigins, ",")
		srv.EnableCORS(origins)
	}
	
	// Configure rate limiting if enabled
	if *enableRateLimit {
		srv.EnableRateLimit(*rateLimit, *rateLimitBurst)
	}

	// Start background tasks
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start polling for each enabled repository
	eventFetcher.StartPolling(ctx)

	// Start storage maintenance tasks
	if *vacuumInterval > 0 {
		go runVacuumTask(ctx, store, *retentionDays, *vacuumInterval, log)
	}
	
	// Start metrics server if enabled
	if obs != nil && obs.PrometheusRegistry() != nil {
		metricsAddr := fmt.Sprintf(":%d", *metricsPort)
		go func() {
			log.Info("Starting metrics server", "addr", metricsAddr)
			metricsMux := http.NewServeMux()
			metricsMux.Handle("/metrics", promhttp.HandlerFor(
				obs.PrometheusRegistry(),
				promhttp.HandlerOpts{
					EnableOpenMetrics: true,
				},
			))
			if err := http.ListenAndServe(metricsAddr, metricsMux); err != nil {
				log.WithError(err).Error("Metrics server failed")
			}
		}()
	}

	// Start HTTP server
	httpServer := &http.Server{
		Addr:         *listen,
		Handler:      srv,
		ReadTimeout:  *readTimeout,
		WriteTimeout: *writeTimeout,
	}

	// Channel to capture server errors
	serverErr := make(chan error, 1)
	
	go func() {
		log.Info("Starting server",
			"listen", *listen,
			"repository", normalizedRepo,
			"poll_interval", *pollInterval,
		)

		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- fmt.Errorf("failed to start server: %w", err)
		}
	}()

	// Wait for interrupt signal or server error
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	
	select {
	case <-sigCh:
		log.Info("Received shutdown signal")
	case err := <-serverErr:
		return err
	}

	log.Info("Shutting down...")

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), *shutdownTimeout)
	defer shutdownCancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.WithError(err).Error("Failed to shutdown server gracefully")
	}
	
	// Stop the server (cleanup rate limiter)
	srv.Stop()
	
	return nil
}

func runVacuumTask(ctx context.Context, store storage.Store, retentionDays int, interval time.Duration, log *logger.Logger) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			log.Info("Running storage vacuum", "retention_days", retentionDays)

			// Archive partitions older than retention period
			if maintenanceStore, ok := store.(storage.MaintenanceStore); ok {
				if err := maintenanceStore.ArchiveOldPartitions(ctx, retentionDays); err != nil {
					log.WithError(err).Error("Failed to archive old partitions")
				}

				// Run SQLite optimization
				if err := maintenanceStore.Optimize(ctx); err != nil {
					log.WithError(err).Error("Failed to optimize database")
				}
			}
		}
	}
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
