package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/graphql-go/handler"
	"github.com/zimbatm/nix-experiments/chronixpkgs/internal/graphql"
	"github.com/zimbatm/nix-experiments/chronixpkgs/internal/logger"
	"github.com/zimbatm/nix-experiments/chronixpkgs/internal/observability"
	"github.com/zimbatm/nix-experiments/chronixpkgs/internal/storage"
	"github.com/zimbatm/nix-experiments/chronixpkgs/internal/webhook"
)

type Server struct {
	store          storage.Store
	webhookHandler *webhook.Handler
	mux            *http.ServeMux
	handler        http.Handler
	monitoredRepo  string
	corsEnabled    bool
	corsOrigins    []string
	rateLimiter    *RateLimiter
	defaultLimit   int
	maxLimit       int
	sseInterval    time.Duration
	logger         *logger.Logger
	observability  *observability.Provider
}

func NewServer(store storage.Store, webhookHandler *webhook.Handler) *Server {
	s := &Server{
		store:          store,
		webhookHandler: webhookHandler,
		mux:            http.NewServeMux(),
		monitoredRepo:  "", // must be set via SetMonitoredRepo
		defaultLimit:   100,
		maxLimit:       1000,
		sseInterval:    10 * time.Second,
		logger:         logger.Default(),
	}

	// Register routes
	s.mux.Handle("/webhook/github", webhookHandler)
	s.mux.HandleFunc("/events", s.handleEvents)
	s.mux.HandleFunc("/events/stream", s.handleEventStream)
	s.mux.HandleFunc("/event-types", s.handleEventTypes)
	s.mux.HandleFunc("/health", s.handleHealth)

	// Add GraphQL endpoint
	resolver := graphql.NewResolver(store, s.monitoredRepo)
	schema, err := resolver.BuildSchema()
	if err != nil {
		// Log error but don't fail server startup
		s.logger.WithError(err).Error("Failed to build GraphQL schema")
	} else {
		graphqlHandler := handler.New(&handler.Config{
			Schema:   &schema,
			Pretty:   true,
			GraphiQL: true, // Enable GraphiQL interface at /graphql
		})
		s.mux.Handle("/graphql", graphqlHandler)
	}

	// Build default handler
	s.buildHandler()
	
	return s
}

func (s *Server) EnableCORS(origins []string) {
	s.corsEnabled = true
	s.corsOrigins = origins
	s.buildHandler()
}

func (s *Server) EnableRateLimit(requestsPerMinute, burstSize int) {
	if s.rateLimiter != nil {
		s.rateLimiter.Stop()
	}
	s.rateLimiter = NewRateLimiter(requestsPerMinute, burstSize)
	s.buildHandler()
}

func (s *Server) Stop() {
	if s.rateLimiter != nil {
		s.rateLimiter.Stop()
	}
}

func (s *Server) SetMonitoredRepo(repo string) {
	s.monitoredRepo = repo
}

func (s *Server) getMonitoredRepo() string {
	return s.monitoredRepo
}

func (s *Server) SetAPILimits(defaultLimit, maxLimit int) {
	s.defaultLimit = defaultLimit
	s.maxLimit = maxLimit
}

func (s *Server) SetSSEInterval(interval time.Duration) {
	s.sseInterval = interval
}

func (s *Server) SetLogger(log *logger.Logger) {
	s.logger = log
	s.buildHandler()
}

func (s *Server) SetObservability(o *observability.Provider) {
	s.observability = o
	s.buildHandler()
}

func (s *Server) buildHandler() {
	handler := http.Handler(s.mux)
	
	// Apply middleware in reverse order (innermost first)
	handler = CacheMiddleware(handler)
	handler = CompressionMiddleware(handler)
	
	if s.corsEnabled {
		handler = CORSMiddleware(s.corsOrigins)(handler)
	}
	
	if s.rateLimiter != nil {
		handler = s.rateLimiter.Middleware(handler)
	}
	
	// Add OpenTelemetry instrumentation if available
	if s.observability != nil {
		handler = observability.HTTPMiddleware(s.observability, "chronixpkgs")(handler)
	}
	
	handler = LoggingMiddleware(s.logger)(handler)
	
	s.handler = handler
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.handler.ServeHTTP(w, r)
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get the repository we're monitoring
	repo := s.getMonitoredRepo()

	// Parse since parameter
	var since time.Time
	sinceStr := r.URL.Query().Get("since")
	if sinceStr != "" {
		parsedTime, err := time.Parse(time.RFC3339, sinceStr)
		if err != nil {
			http.Error(w, "invalid since parameter, expected RFC3339 format", http.StatusBadRequest)
			return
		}
		// Reject future dates
		if parsedTime.After(time.Now()) {
			http.Error(w, "since parameter cannot be in the future", http.StatusBadRequest)
			return
		}
		since = parsedTime
	}

	// Parse limit parameter
	limit := s.defaultLimit
	limitStr := r.URL.Query().Get("limit")
	if limitStr != "" {
		parsedLimit, err := strconv.Atoi(limitStr)
		if err != nil || parsedLimit <= 0 {
			http.Error(w, "invalid limit parameter", http.StatusBadRequest)
			return
		}
		if parsedLimit > s.maxLimit {
			parsedLimit = s.maxLimit
		}
		limit = parsedLimit
	}

	// Parse event types filter
	eventTypes := r.URL.Query()["type"]
	// Validate event types (limit to reasonable number)
	if len(eventTypes) > 50 {
		http.Error(w, "too many event types specified (max 50)", http.StatusBadRequest)
		return
	}

	// Fetch events from storage
	var events []storage.Event
	var err error

	if len(eventTypes) > 0 {
		filter := storage.EventFilter{
			Repo:       repo,
			Since:      since,
			EventTypes: eventTypes,
			Limit:      limit,
		}
		events, err = s.store.GetEventsFiltered(r.Context(), filter)
	} else {
		events, err = s.store.GetEvents(r.Context(), repo, since, limit)
	}

	if err != nil {
		http.Error(w, fmt.Sprintf("failed to fetch events: %v", err), http.StatusInternalServerError)
		return
	}

	// Return JSON response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(events); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}
}

func (s *Server) handleEventStream(w http.ResponseWriter, r *http.Request) {
	// Check if client accepts SSE
	if r.Header.Get("Accept") != "text/event-stream" {
		http.Error(w, "SSE not supported", http.StatusNotAcceptable)
		return
	}

	repo := s.getMonitoredRepo()

	// Get last event ID for resumption
	lastEventID := r.URL.Query().Get("last_event_id")

	// Also check the Last-Event-ID header (standard SSE resumption)
	if lastEventID == "" {
		lastEventID = r.Header.Get("Last-Event-ID")
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // Disable nginx buffering

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// Send initial events if resuming
	if lastEventID != "" {
		events, err := s.store.GetEventsAfterId(r.Context(), repo, lastEventID, 100)
		if err == nil && len(events) > 0 {
			// Send missed events immediately
			for i := len(events) - 1; i >= 0; i-- { // Reverse to send oldest first
				event := events[i]
				data, _ := json.Marshal(event)
				fmt.Fprintf(w, "id: %s\nevent: event\ndata: %s\n\n", event.ID, data)
			}
			flusher.Flush()

			// Update lastEventID to the newest event
			lastEventID = events[0].ID
		}
	}

	// Send initial ping with current time
	fmt.Fprintf(w, "event: ping\ndata: {\"time\": \"%s\"}\n\n", time.Now().Format(time.RFC3339))
	flusher.Flush()

	// Poll for new events
	ticker := time.NewTicker(s.sseInterval)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			var events []storage.Event
			var err error

			if lastEventID == "" {
				// First time, just get recent events
				events, err = s.store.GetEvents(r.Context(), repo, time.Now().Add(-1*time.Minute), 100)
			} else {
				// Get events after the last known ID
				events, err = s.store.GetEventsAfterId(r.Context(), repo, lastEventID, 100)
			}

			if err != nil {
				fmt.Fprintf(w, "event: error\ndata: {\"error\": \"%s\"}\n\n", err.Error())
				flusher.Flush()
				continue
			}

			// Send events in chronological order (oldest first)
			for i := len(events) - 1; i >= 0; i-- {
				event := events[i]
				data, _ := json.Marshal(event)
				fmt.Fprintf(w, "id: %s\nevent: event\ndata: %s\n\n", event.ID, data)
			}

			if len(events) > 0 {
				lastEventID = events[0].ID // Remember the newest event ID
				flusher.Flush()
			}
		}
	}
}

func (s *Server) handleEventTypes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	repo := s.getMonitoredRepo()
	eventTypes, err := s.store.ListEventTypes(r.Context(), repo)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to list event types: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(eventTypes); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	// Check database health
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	// Try to fetch a single event to verify database connectivity
	_, err := s.store.GetEvents(ctx, s.getMonitoredRepo(), time.Now().Add(-24*time.Hour), 1)

	status := "ok"
	statusCode := http.StatusOK

	if err != nil && err != context.DeadlineExceeded {
		// Don't fail health check on timeout, but log it
		status = "degraded"
		statusCode = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]string{
		"status": status,
		"time":   time.Now().Format(time.RFC3339),
	})
}

