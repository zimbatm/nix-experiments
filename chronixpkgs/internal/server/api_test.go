package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/zimbatm/nix-experiments/chronixpkgs/internal/storage"
)

// mockStore implements storage.Store for testing
type mockStore struct {
	events     []storage.Event
	eventTypes []string
	fetchState *storage.FetchState
	saveError  error
	getError   error
}

func (m *mockStore) SaveEvents(ctx context.Context, events []storage.Event) error {
	if m.saveError != nil {
		return m.saveError
	}
	m.events = append(m.events, events...)
	return nil
}

func (m *mockStore) GetEvents(ctx context.Context, repo string, since time.Time, limit int) ([]storage.Event, error) {
	if m.getError != nil {
		return nil, m.getError
	}

	var filtered []storage.Event
	for _, e := range m.events {
		if e.Repo == repo && e.CreatedAt.After(since) {
			filtered = append(filtered, e)
		}
	}

	if len(filtered) > limit {
		filtered = filtered[:limit]
	}
	return filtered, nil
}

func (m *mockStore) GetEventsFiltered(ctx context.Context, filter storage.EventFilter) ([]storage.Event, error) {
	if m.getError != nil {
		return nil, m.getError
	}

	var filtered []storage.Event
	for _, e := range m.events {
		// Check repo
		if e.Repo != filter.Repo {
			continue
		}

		// Check time filter
		if !filter.Since.IsZero() && e.CreatedAt.Before(filter.Since) {
			continue
		}

		// Check event type filter
		if len(filter.EventTypes) > 0 {
			found := false
			for _, t := range filter.EventTypes {
				if e.Type == t {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// Check actor filter
		if filter.Actor != "" && e.Actor != filter.Actor {
			continue
		}

		// Check PR number filter (would need payload parsing in real implementation)
		if filter.PRNumber > 0 {
			// Mock doesn't support this - skip all events
			continue
		}

		// Check issue number filter (would need payload parsing in real implementation)
		if filter.IssueNumber > 0 {
			// Mock doesn't support this - skip all events
			continue
		}

		filtered = append(filtered, e)
	}

	// Apply offset
	if filter.Offset > 0 && filter.Offset < len(filtered) {
		filtered = filtered[filter.Offset:]
	}

	// Apply limit
	if filter.Limit > 0 && len(filtered) > filter.Limit {
		filtered = filtered[:filter.Limit]
	}
	return filtered, nil
}

func (m *mockStore) GetEventsAfterId(ctx context.Context, repo, afterId string, limit int) ([]storage.Event, error) {
	if m.getError != nil {
		return nil, m.getError
	}

	var filtered []storage.Event
	found := false
	for _, e := range m.events {
		if e.ID == afterId {
			found = true
			continue
		}
		if found && e.Repo == repo {
			filtered = append(filtered, e)
		}
	}

	if len(filtered) > limit {
		filtered = filtered[:limit]
	}
	return filtered, nil
}

func (m *mockStore) ListEventTypes(ctx context.Context, repo string) ([]string, error) {
	if m.getError != nil {
		return nil, m.getError
	}
	return m.eventTypes, nil
}

func (m *mockStore) GetFetchState(ctx context.Context, repo string) (*storage.FetchState, error) {
	if m.fetchState == nil {
		return nil, fmt.Errorf("not found")
	}
	return m.fetchState, nil
}

func (m *mockStore) SaveFetchState(ctx context.Context, state *storage.FetchState) error {
	m.fetchState = state
	return nil
}

func (m *mockStore) UpdateFetchState(ctx context.Context, state *storage.FetchState) error {
	m.fetchState = state
	return nil
}

func (m *mockStore) Close() error {
	return nil
}

func (m *mockStore) GetEventCount(ctx context.Context, repo string) (int64, error) {
	count := 0
	for _, e := range m.events {
		if e.Repo == repo {
			count++
		}
	}
	return int64(count), nil
}

// TestHealthEndpoint tests the /health endpoint
func TestHealthEndpoint(t *testing.T) {
	store := &mockStore{}
	server := NewServer(store, nil)
	server.SetMonitoredRepo("github.com/NixOS/nixpkgs")

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	server.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var result map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if result["status"] != "ok" {
		t.Errorf("Expected status 'ok', got '%s'", result["status"])
	}
}

// TestGetEvents tests the /events endpoint
func TestGetEvents(t *testing.T) {
	now := time.Now().UTC()
	store := &mockStore{
		events: []storage.Event{
			{
				ID:        "1",
				Repo:      "github.com/NixOS/nixpkgs",
				Type:      "PushEvent",
				Actor:     "user1",
				CreatedAt: now.Add(-1 * time.Hour),
				Payload:   json.RawMessage(`{"commits": 1}`),
			},
			{
				ID:        "2",
				Repo:      "github.com/NixOS/nixpkgs",
				Type:      "IssuesEvent",
				Actor:     "user2",
				CreatedAt: now,
				Payload:   json.RawMessage(`{"action": "opened"}`),
			},
		},
	}

	server := NewServer(store, nil)
	server.SetMonitoredRepo("github.com/NixOS/nixpkgs")

	tests := []struct {
		name           string
		query          string
		expectedCount  int
		expectedStatus int
	}{
		{
			name:           "no parameters",
			query:          "",
			expectedCount:  2,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "with since parameter",
			query:          "?since=" + now.Add(-30*time.Minute).Format(time.RFC3339),
			expectedCount:  1,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "with type filter",
			query:          "?type=PushEvent",
			expectedCount:  1,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "with limit",
			query:          "?limit=1",
			expectedCount:  1,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "since with event ID",
			query:          "?since=12345",
			expectedCount:  2,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "filter by actor",
			query:          "?actor=user1",
			expectedCount:  1,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "filter by PR number",
			query:          "?pr=123",
			expectedCount:  0,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "filter by issue number",
			query:          "?issue=456",
			expectedCount:  0,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "combined filters",
			query:          "?type=PushEvent&actor=user1",
			expectedCount:  1,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "pagination with offset",
			query:          "?limit=1&offset=1",
			expectedCount:  1,
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/events"+tt.query, nil)
			w := httptest.NewRecorder()

			server.ServeHTTP(w, req)

			resp := w.Result()
			if resp.StatusCode != tt.expectedStatus {
				body, _ := io.ReadAll(resp.Body)
				t.Errorf("Expected status %d, got %d. Body: %s", tt.expectedStatus, resp.StatusCode, body)
				return
			}

			if tt.expectedStatus == http.StatusOK {
				var events []storage.Event
				if err := json.NewDecoder(resp.Body).Decode(&events); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				if len(events) != tt.expectedCount {
					t.Errorf("Expected %d events, got %d", tt.expectedCount, len(events))
				}
			}
		})
	}
}

// TestEventStream tests the /events endpoint with SSE
func TestEventStream(t *testing.T) {
	store := &mockStore{
		events: []storage.Event{
			{
				ID:        "1",
				Repo:      "github.com/NixOS/nixpkgs",
				Type:      "PushEvent",
				Actor:     "user1",
				CreatedAt: time.Now(),
				Payload:   json.RawMessage(`{}`),
			},
		},
	}

	server := NewServer(store, nil)
	server.SetMonitoredRepo("github.com/NixOS/nixpkgs")
	server.SetSSEInterval(100 * time.Millisecond) // Fast interval for testing

	// Create a request with a cancellable context
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest("GET", "/events", nil).WithContext(ctx)
	req.Header.Set("Accept", "text/event-stream")

	// Use a custom response recorder
	w := httptest.NewRecorder()

	// Run server in goroutine
	done := make(chan bool)
	go func() {
		server.ServeHTTP(w, req)
		done <- true
	}()

	// Give it time to send initial ping
	time.Sleep(50 * time.Millisecond)

	// Cancel the request context
	cancel()

	// Wait for handler to finish
	select {
	case <-done:
		// Good, handler finished
	case <-time.After(1 * time.Second):
		t.Error("Handler didn't finish within timeout")
	}

	// The handler may return before writing headers if context is cancelled early
	// So we'll just check that it didn't crash
	resp := w.Result()

	// If we got a response, check headers
	if resp.StatusCode == http.StatusOK {
		if !strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
			t.Error("Expected Content-Type to contain text/event-stream")
		}
	}
}

// TestEventTypes tests the /event-types endpoint
func TestEventTypes(t *testing.T) {
	store := &mockStore{
		eventTypes: []string{"PushEvent", "IssuesEvent", "PullRequestEvent"},
	}

	server := NewServer(store, nil)
	server.SetMonitoredRepo("github.com/NixOS/nixpkgs")

	req := httptest.NewRequest("GET", "/event-types", nil)
	w := httptest.NewRecorder()

	server.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var types []string
	if err := json.NewDecoder(resp.Body).Decode(&types); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(types) != 3 {
		t.Errorf("Expected 3 event types, got %d", len(types))
	}
}

// TestCORSMiddleware tests CORS functionality
func TestCORSMiddleware(t *testing.T) {
	store := &mockStore{}
	server := NewServer(store, nil)
	server.SetMonitoredRepo("github.com/NixOS/nixpkgs")
	// CORS is always enabled now with all origins allowed

	// Test preflight request
	req := httptest.NewRequest("OPTIONS", "/events", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "GET")
	w := httptest.NewRecorder()

	server.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("Expected status 204 for preflight, got %d", resp.StatusCode)
	}

	if resp.Header.Get("Access-Control-Allow-Origin") != "*" {
		t.Error("Expected CORS header allowing all origins")
	}

	// Test actual request
	req = httptest.NewRequest("GET", "/events", nil)
	req.Header.Set("Origin", "https://example.com")
	w = httptest.NewRecorder()

	server.ServeHTTP(w, req)

	resp = w.Result()
	if resp.Header.Get("Access-Control-Allow-Origin") != "*" {
		t.Error("Expected CORS header allowing all origins on actual request")
	}
}

// TestRateLimit tests rate limiting functionality
func TestRateLimit(t *testing.T) {
	store := &mockStore{}
	server := NewServer(store, nil)
	server.SetMonitoredRepo("github.com/NixOS/nixpkgs")
	server.EnableRateLimit(2, 2) // 2 requests per minute, burst of 2

	// Make 3 requests rapidly
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("GET", "/health", nil)
		req.RemoteAddr = "127.0.0.1:12345" // Same IP
		w := httptest.NewRecorder()

		server.ServeHTTP(w, req)

		resp := w.Result()
		if i < 2 {
			// First 2 requests should succeed
			if resp.StatusCode != http.StatusOK {
				t.Errorf("Request %d: Expected status 200, got %d", i+1, resp.StatusCode)
			}
		} else {
			// Third request should be rate limited
			if resp.StatusCode != http.StatusTooManyRequests {
				t.Errorf("Request %d: Expected status 429, got %d", i+1, resp.StatusCode)
			}
		}
	}
}

// TestCompressionMiddleware tests gzip compression
func TestCompressionMiddleware(t *testing.T) {
	store := &mockStore{
		events: make([]storage.Event, 100), // Large response to trigger compression
	}

	// Fill with dummy events
	for i := 0; i < 100; i++ {
		store.events[i] = storage.Event{
			ID:        fmt.Sprintf("event%d", i),
			Repo:      "github.com/NixOS/nixpkgs",
			Type:      "PushEvent",
			Actor:     "user",
			CreatedAt: time.Now(),
			Payload:   json.RawMessage(`{"data": "This is some test data to make the response larger"}`),
		}
	}

	server := NewServer(store, nil)
	server.SetMonitoredRepo("github.com/NixOS/nixpkgs")

	req := httptest.NewRequest("GET", "/events", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()

	server.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Check if response is compressed
	if resp.Header.Get("Content-Encoding") != "gzip" {
		t.Error("Expected gzip content encoding")
	}
}

// TestAPILimits tests that API limits are properly enforced
func TestAPILimits(t *testing.T) {
	// Create more events than the limit
	events := make([]storage.Event, 150)
	for i := 0; i < 150; i++ {
		events[i] = storage.Event{
			ID:        fmt.Sprintf("event%d", i),
			Repo:      "github.com/NixOS/nixpkgs",
			Type:      "PushEvent",
			Actor:     "user",
			CreatedAt: time.Now(),
		}
	}

	store := &mockStore{events: events}
	server := NewServer(store, nil)
	server.SetMonitoredRepo("github.com/NixOS/nixpkgs")
	server.SetAPILimits(50, 100) // Default 50, max 100

	tests := []struct {
		name          string
		query         string
		expectedCount int
	}{
		{
			name:          "default limit",
			query:         "",
			expectedCount: 50,
		},
		{
			name:          "custom limit within max",
			query:         "?limit=75",
			expectedCount: 75,
		},
		{
			name:          "limit exceeds max",
			query:         "?limit=200",
			expectedCount: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/events"+tt.query, nil)
			w := httptest.NewRecorder()

			server.ServeHTTP(w, req)

			var result []storage.Event
			if err := json.NewDecoder(w.Result().Body).Decode(&result); err != nil {
				t.Fatalf("Failed to decode response: %v", err)
			}

			if len(result) != tt.expectedCount {
				t.Errorf("Expected %d events, got %d", tt.expectedCount, len(result))
			}
		})
	}
}
