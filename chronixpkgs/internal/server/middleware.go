package server

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/zimbatm/nix-experiments/chronixpkgs/internal/logger"
	"github.com/zimbatm/nix-experiments/chronixpkgs/internal/metrics"
)

// gzipResponseWriter wraps http.ResponseWriter to provide gzip compression
type gzipResponseWriter struct {
	http.ResponseWriter
	writer *gzip.Writer
}

func (w *gzipResponseWriter) Write(b []byte) (int, error) {
	return w.writer.Write(b)
}

// Flush implements http.Flusher interface
func (w *gzipResponseWriter) Flush() {
	w.writer.Flush()
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// gzipPool reuses gzip writers for better performance
var gzipPool = sync.Pool{
	New: func() interface{} {
		w, _ := gzip.NewWriterLevel(nil, gzip.BestSpeed)
		return w
	},
}

// CompressionMiddleware adds gzip compression to responses
func CompressionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip compression for SSE
		if strings.Contains(r.URL.Path, "/stream") {
			next.ServeHTTP(w, r)
			return
		}

		// Check if client accepts gzip
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}

		// Get a gzip writer from the pool
		gz := gzipPool.Get().(*gzip.Writer)
		defer func() {
			// Ensure gz is reset even if there's a panic
			gz.Reset(nil)
			gzipPool.Put(gz)
		}()

		gz.Reset(w)
		defer gz.Close()

		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Del("Content-Length") // Content-Length is not valid with compression

		gzw := &gzipResponseWriter{ResponseWriter: w, writer: gz}
		next.ServeHTTP(gzw, r)
	})
}

// CacheMiddleware adds cache headers for static responses
func CacheMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Add cache headers for event queries
		if strings.Contains(r.URL.Path, "/events") && r.Method == "GET" {
			// Events are immutable, cache for 1 hour
			w.Header().Set("Cache-Control", "public, max-age=3600")
		}

		// Health check can be cached briefly
		if r.URL.Path == "/health" {
			w.Header().Set("Cache-Control", "public, max-age=10")
		}

		next.ServeHTTP(w, r)
	})
}

// responseRecorder wraps http.ResponseWriter to capture status code and implement common interfaces
type responseRecorder struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

func newResponseRecorder(w http.ResponseWriter) *responseRecorder {
	return &responseRecorder{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
		written:        false,
	}
}

func (rw *responseRecorder) WriteHeader(code int) {
	if !rw.written {
		rw.statusCode = code
		rw.ResponseWriter.WriteHeader(code)
		rw.written = true
	}
}

func (rw *responseRecorder) Write(b []byte) (int, error) {
	if !rw.written {
		rw.WriteHeader(http.StatusOK)
	}
	return rw.ResponseWriter.Write(b)
}

// Flush implements http.Flusher interface
func (rw *responseRecorder) Flush() {
	if flusher, ok := rw.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// Hijack implements http.Hijacker interface
func (rw *responseRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hijacker, ok := rw.ResponseWriter.(http.Hijacker); ok {
		return hijacker.Hijack()
	}
	return nil, nil, fmt.Errorf("response writer does not support hijacking")
}

// LoggingMiddleware adds request logging
func LoggingMiddleware(log *logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rw := newResponseRecorder(w)

			next.ServeHTTP(rw, r)

			// Log request details
			duration := time.Since(start).Seconds()
			log.LogRequest(r.Method, r.URL.Path, rw.statusCode, duration*1000) // Log in milliseconds

			// Record metrics
			metrics.RecordHTTPRequest(r.Method, r.URL.Path, rw.statusCode, duration)
		})
	}
}

// CORSMiddleware adds CORS headers - always allows all origins
func CORSMiddleware(allowedOrigins []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Always allow all origins
			w.Header().Set("Access-Control-Allow-Origin", "*")

			if r.Method == http.MethodOptions {
				// Handle preflight requests
				w.Header().Set("Access-Control-Allow-Methods", "GET, HEAD, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, Authorization")
				w.Header().Set("Access-Control-Max-Age", "86400") // 24 hours
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
