package server

import (
	"compress/gzip"
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

// responseWriter captures status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// LoggingMiddleware adds request logging
func LoggingMiddleware(log *logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			next.ServeHTTP(rw, r)

			// Log request details
			duration := time.Since(start).Seconds()
			log.LogRequest(r.Method, r.URL.Path, rw.statusCode, duration*1000) // Log in milliseconds
			
			// Record metrics
			metrics.RecordHTTPRequest(r.Method, r.URL.Path, rw.statusCode, duration)
		})
	}
}

// CORSMiddleware adds CORS headers based on allowed origins
func CORSMiddleware(allowedOrigins []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// Check if origin is allowed
			allowed := false
			for _, allowedOrigin := range allowedOrigins {
				if allowedOrigin == "*" || allowedOrigin == origin {
					allowed = true
					break
				}
			}

			if allowed && origin != "" {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			} else if len(allowedOrigins) > 0 && allowedOrigins[0] == "*" {
				w.Header().Set("Access-Control-Allow-Origin", "*")
			}

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

