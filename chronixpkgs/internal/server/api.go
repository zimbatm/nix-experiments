package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/zimbatm/nix-experiments/chronixpkgs/internal/logger"
	"github.com/zimbatm/nix-experiments/chronixpkgs/internal/observability"
	"github.com/zimbatm/nix-experiments/chronixpkgs/internal/storage"
)

type Server struct {
	store         storage.Store
	mux           *http.ServeMux
	handler       http.Handler
	monitoredRepo string
	rateLimiter   *RateLimiter
	defaultLimit  int
	maxLimit      int
	sseInterval   time.Duration
	logger        *logger.Logger
	observability *observability.Provider
}

type eventQueryParams struct {
	eventTypes  []string
	sinceTime   *time.Time
	sinceID     string
	actor       string
	prNumber    int
	issueNumber int
	err         error
}

func NewServer(store storage.Store, _ interface{}) *Server {
	s := &Server{
		store:         store,
		mux:           http.NewServeMux(),
		monitoredRepo: "", // must be set via SetMonitoredRepo
		defaultLimit:  100,
		maxLimit:      1000,
		sseInterval:   10 * time.Second,
		logger:        logger.Default(),
	}

	// Register routes
	s.mux.HandleFunc("/", s.handleRoot)
	s.mux.HandleFunc("/events", s.handleEvents)
	s.mux.HandleFunc("/event-types", s.handleEventTypes)
	s.mux.HandleFunc("/health", s.handleHealth)

	// Build default handler
	s.buildHandler()

	return s
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

	// Always enable CORS with all origins allowed
	handler = CORSMiddleware(nil)(handler)

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

// sendJSONError sends a JSON error response
func sendJSONError(w http.ResponseWriter, message string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

// sendJSON sends a JSON response
func sendJSON(w http.ResponseWriter, data interface{}) error {
	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(data)
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		sendJSONError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check if client wants SSE
	if r.Header.Get("Accept") == "text/event-stream" {
		s.handleEventStream(w, r)
		return
	}

	// Get the repository we're monitoring
	repo := s.getMonitoredRepo()

	// Parse query parameters
	params := s.parseEventQueryParams(r)
	if params.err != nil {
		sendJSONError(w, params.err.Error(), http.StatusBadRequest)
		return
	}

	// Parse pagination parameters for non-SSE requests
	limit := s.defaultLimit
	limitStr := r.URL.Query().Get("limit")
	if limitStr != "" {
		parsedLimit, err := strconv.Atoi(limitStr)
		if err != nil || parsedLimit <= 0 {
			sendJSONError(w, "invalid limit parameter", http.StatusBadRequest)
			return
		}
		if parsedLimit > s.maxLimit {
			parsedLimit = s.maxLimit
		}
		limit = parsedLimit
	}

	offset := 0
	offsetStr := r.URL.Query().Get("offset")
	if offsetStr != "" {
		parsedOffset, err := strconv.Atoi(offsetStr)
		if err != nil || parsedOffset < 0 {
			sendJSONError(w, "invalid offset parameter", http.StatusBadRequest)
			return
		}
		offset = parsedOffset
	}
	// Build filter for storage query
	filter := storage.EventFilter{
		Repo:        repo,
		SinceID:     params.sinceID,
		EventTypes:  params.eventTypes,
		Actor:       params.actor,
		PRNumber:    params.prNumber,
		IssueNumber: params.issueNumber,
		Limit:       limit,
		Offset:      offset,
	}

	// Set Since time if provided
	if params.sinceTime != nil {
		filter.Since = *params.sinceTime
	}

	// Fetch events from storage
	events, err := s.store.GetEventsFiltered(r.Context(), filter)

	if err != nil {
		sendJSONError(w, fmt.Sprintf("failed to fetch events: %v", err), http.StatusInternalServerError)
		return
	}

	// Return JSON response
	if err := sendJSON(w, events); err != nil {
		sendJSONError(w, "failed to encode response", http.StatusInternalServerError)
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

	// Parse query parameters
	params := s.parseEventQueryParams(r)
	if params.err != nil {
		http.Error(w, params.err.Error(), http.StatusBadRequest)
		return
	}

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
		// Build filter for resumption
		filter := storage.EventFilter{
			Repo:        repo,
			SinceID:     lastEventID,
			EventTypes:  params.eventTypes,
			Actor:       params.actor,
			PRNumber:    params.prNumber,
			IssueNumber: params.issueNumber,
			Limit:       100,
		}
		events, err := s.store.GetEventsFiltered(r.Context(), filter)
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
			// Build filter for real-time streaming
			filter := storage.EventFilter{
				Repo:        repo,
				SinceID:     lastEventID,
				EventTypes:  params.eventTypes,
				Actor:       params.actor,
				PRNumber:    params.prNumber,
				IssueNumber: params.issueNumber,
				Limit:       100,
			}

			// If no lastEventID, get events from last minute
			if lastEventID == "" {
				filter.Since = time.Now().Add(-1 * time.Minute)
				filter.SinceID = ""
			}

			events, err := s.store.GetEventsFiltered(r.Context(), filter)

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

func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	// Check if client wants HTML
	acceptHeader := r.Header.Get("Accept")
	if strings.Contains(acceptHeader, "text/html") {
		s.handleRootHTML(w, r)
		return
	}

	// Default to JSON
	if err := sendJSON(w, map[string]interface{}{
		"service": "chronixpkgs",
		"version": "dev",
		"endpoints": map[string]string{
			"events":      "/events",
			"event_types": "/event-types",
			"health":      "/health",
		},
		"repository": s.getMonitoredRepo(),
		"note":       "For SSE streaming, use /events with Accept: text/event-stream header",
	}); err != nil {
		sendJSONError(w, "failed to encode response", http.StatusInternalServerError)
	}
}

func (s *Server) handleEventTypes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		sendJSONError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	repo := s.getMonitoredRepo()
	eventTypes, err := s.store.ListEventTypes(r.Context(), repo)
	if err != nil {
		sendJSONError(w, fmt.Sprintf("failed to list event types: %v", err), http.StatusInternalServerError)
		return
	}

	if err := sendJSON(w, eventTypes); err != nil {
		sendJSONError(w, "failed to encode response", http.StatusInternalServerError)
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

	w.WriteHeader(statusCode)
	sendJSON(w, map[string]string{
		"status": status,
		"time":   time.Now().Format(time.RFC3339),
	})
}
