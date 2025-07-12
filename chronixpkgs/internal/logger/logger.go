package logger

import (
	"context"
	"log/slog"
	"os"
)

// Logger is a wrapper around slog.Logger with convenience methods
type Logger struct {
	*slog.Logger
}

// LogLevel represents the logging level
type LogLevel string

const (
	LevelDebug LogLevel = "debug"
	LevelInfo  LogLevel = "info"
	LevelWarn  LogLevel = "warn"
	LevelError LogLevel = "error"
)

// Config holds logger configuration
type Config struct {
	Level  LogLevel
	Format string // "json" or "text"
}

// New creates a new logger with the given configuration
func New(cfg Config) *Logger {
	var level slog.Level
	switch cfg.Level {
	case LevelDebug:
		level = slog.LevelDebug
	case LevelWarn:
		level = slog.LevelWarn
	case LevelError:
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level: level,
		// Add source file information in debug mode
		AddSource: cfg.Level == LevelDebug,
	}

	var handler slog.Handler
	if cfg.Format == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	return &Logger{
		Logger: slog.New(handler),
	}
}

// Default creates a logger with default settings
func Default() *Logger {
	return New(Config{
		Level:  LevelInfo,
		Format: "text",
	})
}

// WithContext returns a logger with context values
func (l *Logger) WithContext(ctx context.Context) *Logger {
	// Extract common context values
	attrs := []slog.Attr{}
	
	// Add request ID if present
	if reqID := ctx.Value("request_id"); reqID != nil {
		attrs = append(attrs, slog.String("request_id", reqID.(string)))
	}
	
	// Add user if present
	if user := ctx.Value("user"); user != nil {
		attrs = append(attrs, slog.String("user", user.(string)))
	}
	
	if len(attrs) == 0 {
		return l
	}
	
	args := make([]any, 0, len(attrs)*2)
	for _, attr := range attrs {
		args = append(args, attr.Key, attr.Value)
	}
	return &Logger{
		Logger: l.With(args...),
	}
}

// WithError adds an error field to the logger
func (l *Logger) WithError(err error) *Logger {
	return &Logger{
		Logger: l.With(slog.String("error", err.Error())),
	}
}

// WithField adds a single field to the logger
func (l *Logger) WithField(key string, value any) *Logger {
	return &Logger{
		Logger: l.With(slog.Any(key, value)),
	}
}

// WithFields adds multiple fields to the logger
func (l *Logger) WithFields(fields map[string]any) *Logger {
	attrs := make([]any, 0, len(fields)*2)
	for k, v := range fields {
		attrs = append(attrs, k, v)
	}
	return &Logger{
		Logger: l.With(attrs...),
	}
}

// Common logging helpers for HTTP requests
func (l *Logger) LogRequest(method, path string, status int, duration float64) {
	l.Info("http request",
		slog.String("method", method),
		slog.String("path", path),
		slog.Int("status", status),
		slog.Float64("duration_ms", duration),
	)
}

// LogDatabaseQuery logs database queries
func (l *Logger) LogDatabaseQuery(query string, duration float64, err error) {
	if err != nil {
		l.Error("database query failed",
			slog.String("query", query),
			slog.Float64("duration_ms", duration),
			slog.String("error", err.Error()),
		)
	} else if l.Enabled(context.Background(), slog.LevelDebug) {
		l.Debug("database query",
			slog.String("query", query),
			slog.Float64("duration_ms", duration),
		)
	}
}

// LogEventProcessed logs event processing
func (l *Logger) LogEventProcessed(eventType, eventID string, success bool, duration float64) {
	if success {
		l.Info("event processed",
			slog.String("event_type", eventType),
			slog.String("event_id", eventID),
			slog.Float64("duration_ms", duration),
		)
	} else {
		l.Error("event processing failed",
			slog.String("event_type", eventType),
			slog.String("event_id", eventID),
			slog.Float64("duration_ms", duration),
		)
	}
}