package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestNewSQLiteStore tests the creation of a new SQLite store
func TestNewSQLiteStore(t *testing.T) {
	tempDir := t.TempDir()

	store, err := NewSQLiteStore(tempDir)
	if err != nil {
		t.Fatalf("Failed to create SQLite store: %v", err)
	}
	defer store.Close()

	// Check that database file was created
	dbPath := filepath.Join(tempDir, "events.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("Database file was not created")
	}

	// Check that we can ping the database
	ctx := context.Background()
	if err := store.db.PingContext(ctx); err != nil {
		t.Errorf("Failed to ping database: %v", err)
	}
}

// TestSaveAndGetEvents tests saving and retrieving events
func TestSaveAndGetEvents(t *testing.T) {
	tempDir := t.TempDir()
	store, err := NewSQLiteStore(tempDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Create test events
	now := time.Now().UTC()
	testEvents := []Event{
		{
			ID:        "12345",
			Repo:      "github.com/NixOS/nixpkgs",
			Type:      "PushEvent",
			Actor:     "testuser",
			CreatedAt: now.Add(-2 * time.Hour),
			Payload:   json.RawMessage(`{"commits": 1}`),
		},
		{
			ID:        "12346",
			Repo:      "github.com/NixOS/nixpkgs",
			Type:      "IssuesEvent",
			Actor:     "testuser2",
			CreatedAt: now.Add(-1 * time.Hour),
			Payload:   json.RawMessage(`{"action": "opened"}`),
		},
	}

	// Save events
	if err := store.SaveEvents(ctx, testEvents); err != nil {
		t.Fatalf("Failed to save events: %v", err)
	}

	// Retrieve events
	retrieved, err := store.GetEvents(ctx, "github.com/NixOS/nixpkgs", now.Add(-3*time.Hour), 10)
	if err != nil {
		t.Fatalf("Failed to get events: %v", err)
	}

	// Verify we got both events
	if len(retrieved) != 2 {
		t.Errorf("Expected 2 events, got %d", len(retrieved))
	}

	// Verify events are in correct order (newest first)
	if len(retrieved) >= 2 {
		if retrieved[0].ID != "12346" {
			t.Errorf("Expected first event ID to be 12346, got %s", retrieved[0].ID)
		}
		if retrieved[1].ID != "12345" {
			t.Errorf("Expected second event ID to be 12345, got %s", retrieved[1].ID)
		}
	}
}

// TestEventDeduplication tests that duplicate events are not stored twice
func TestEventDeduplication(t *testing.T) {
	tempDir := t.TempDir()
	store, err := NewSQLiteStore(tempDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Create a test event
	event := Event{
		ID:        "unique123",
		Repo:      "github.com/NixOS/nixpkgs",
		Type:      "PushEvent",
		Actor:     "testuser",
		CreatedAt: time.Now().UTC(),
		Payload:   json.RawMessage(`{}`),
	}

	// Save the same event twice
	if err := store.SaveEvents(ctx, []Event{event}); err != nil {
		t.Fatalf("Failed to save event first time: %v", err)
	}

	if err := store.SaveEvents(ctx, []Event{event}); err != nil {
		t.Fatalf("Failed to save event second time: %v", err)
	}

	// Retrieve events - use a time before the event was created
	retrieved, err := store.GetEvents(ctx, event.Repo, event.CreatedAt.Add(-1*time.Hour), 10)
	if err != nil {
		t.Fatalf("Failed to get events: %v", err)
	}

	// Should only have one event
	if len(retrieved) != 1 {
		t.Errorf("Expected 1 event after deduplication, got %d", len(retrieved))
	}
}

// TestPartitioning tests that events are correctly partitioned by month
func TestPartitioning(t *testing.T) {
	tempDir := t.TempDir()
	store, err := NewSQLiteStore(tempDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Create events in different months
	events := []Event{
		{
			ID:        "jan1",
			Repo:      "github.com/NixOS/nixpkgs",
			Type:      "PushEvent",
			Actor:     "user1",
			CreatedAt: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
			Payload:   json.RawMessage(`{}`),
		},
		{
			ID:        "feb1",
			Repo:      "github.com/NixOS/nixpkgs",
			Type:      "PushEvent",
			Actor:     "user2",
			CreatedAt: time.Date(2024, 2, 15, 10, 0, 0, 0, time.UTC),
			Payload:   json.RawMessage(`{}`),
		},
	}

	// Save events
	if err := store.SaveEvents(ctx, events); err != nil {
		t.Fatalf("Failed to save events: %v", err)
	}

	// Check that partition tables were created
	var count int
	err = store.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM sqlite_master 
		WHERE type='table' AND name LIKE 'events_%'
	`).Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query partition tables: %v", err)
	}

	if count < 2 {
		t.Errorf("Expected at least 2 partition tables, found %d", count)
	}
}

// TestGetEventsFiltered tests filtered event retrieval
func TestGetEventsFiltered(t *testing.T) {
	tempDir := t.TempDir()
	store, err := NewSQLiteStore(tempDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	now := time.Now().UTC()

	// Create events of different types
	events := []Event{
		{
			ID:        "push1",
			Repo:      "github.com/NixOS/nixpkgs",
			Type:      "PushEvent",
			Actor:     "user1",
			CreatedAt: now.Add(-2 * time.Hour),
			Payload:   json.RawMessage(`{}`),
		},
		{
			ID:        "issue1",
			Repo:      "github.com/NixOS/nixpkgs",
			Type:      "IssuesEvent",
			Actor:     "user2",
			CreatedAt: now.Add(-1 * time.Hour),
			Payload:   json.RawMessage(`{}`),
		},
		{
			ID:        "push2",
			Repo:      "github.com/NixOS/nixpkgs",
			Type:      "PushEvent",
			Actor:     "user3",
			CreatedAt: now,
			Payload:   json.RawMessage(`{}`),
		},
	}

	// Save events
	if err := store.SaveEvents(ctx, events); err != nil {
		t.Fatalf("Failed to save events: %v", err)
	}

	// Filter for PushEvent only
	filter := EventFilter{
		Repo:       "github.com/NixOS/nixpkgs",
		Since:      now.Add(-3 * time.Hour),
		EventTypes: []string{"PushEvent"},
		Limit:      10,
	}

	filtered, err := store.GetEventsFiltered(ctx, filter)
	if err != nil {
		t.Fatalf("Failed to get filtered events: %v", err)
	}

	// Should only get PushEvents
	if len(filtered) != 2 {
		t.Errorf("Expected 2 PushEvents, got %d", len(filtered))
	}

	for _, event := range filtered {
		if event.Type != "PushEvent" {
			t.Errorf("Expected only PushEvents, got %s", event.Type)
		}
	}
}

// TestGetEventsAfterId tests pagination using event ID
func TestGetEventsAfterId(t *testing.T) {
	tempDir := t.TempDir()
	store, err := NewSQLiteStore(tempDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	now := time.Now().UTC()

	// Create sequential events
	events := make([]Event, 5)
	for i := 0; i < 5; i++ {
		events[i] = Event{
			ID:        fmt.Sprintf("event%d", i),
			Repo:      "github.com/NixOS/nixpkgs",
			Type:      "PushEvent",
			Actor:     "user",
			CreatedAt: now.Add(time.Duration(i-5) * time.Hour),
			Payload:   json.RawMessage(`{}`),
		}
	}

	// Save events
	if err := store.SaveEvents(ctx, events); err != nil {
		t.Fatalf("Failed to save events: %v", err)
	}

	// Get events after event2
	afterEvents, err := store.GetEventsAfterId(ctx, "github.com/NixOS/nixpkgs", "event2", 10)
	if err != nil {
		t.Fatalf("Failed to get events after ID: %v", err)
	}

	// Should get event3 and event4 (newer than event2)
	if len(afterEvents) != 2 {
		t.Errorf("Expected 2 events after event2, got %d", len(afterEvents))
	}

	if len(afterEvents) >= 2 {
		if afterEvents[0].ID != "event4" || afterEvents[1].ID != "event3" {
			t.Errorf("Expected events 4 and 3, got %s and %s", afterEvents[0].ID, afterEvents[1].ID)
		}
	}
}

// TestListEventTypes tests listing unique event types
func TestListEventTypes(t *testing.T) {
	tempDir := t.TempDir()
	store, err := NewSQLiteStore(tempDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Create events with different types
	events := []Event{
		{ID: "1", Repo: "github.com/NixOS/nixpkgs", Type: "PushEvent", Actor: "user1", CreatedAt: time.Now()},
		{ID: "2", Repo: "github.com/NixOS/nixpkgs", Type: "IssuesEvent", Actor: "user2", CreatedAt: time.Now()},
		{ID: "3", Repo: "github.com/NixOS/nixpkgs", Type: "PushEvent", Actor: "user3", CreatedAt: time.Now()},
		{ID: "4", Repo: "github.com/NixOS/nixpkgs", Type: "PullRequestEvent", Actor: "user4", CreatedAt: time.Now()},
	}

	// Save events
	if err := store.SaveEvents(ctx, events); err != nil {
		t.Fatalf("Failed to save events: %v", err)
	}

	// List event types
	types, err := store.ListEventTypes(ctx, "github.com/NixOS/nixpkgs")
	if err != nil {
		t.Fatalf("Failed to list event types: %v", err)
	}

	// Should have 3 unique types
	if len(types) != 3 {
		t.Errorf("Expected 3 unique event types, got %d", len(types))
	}

	// Check that all expected types are present
	typeMap := make(map[string]bool)
	for _, t := range types {
		typeMap[t] = true
	}

	expectedTypes := []string{"PushEvent", "IssuesEvent", "PullRequestEvent"}
	for _, expected := range expectedTypes {
		if !typeMap[expected] {
			t.Errorf("Expected event type %s not found", expected)
		}
	}
}

// TestContextCancellation tests that operations respect context cancellation
func TestContextCancellation(t *testing.T) {
	tempDir := t.TempDir()
	store, err := NewSQLiteStore(tempDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Create a context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Try to save events with cancelled context
	events := []Event{{ID: "1", Repo: "test", Type: "PushEvent", Actor: "user", CreatedAt: time.Now()}}
	err = store.SaveEvents(ctx, events)
	if err == nil {
		t.Error("Expected error when saving with cancelled context")
	}

	// Try to get events with cancelled context
	_, err = store.GetEvents(ctx, "test", time.Now().Add(-1*time.Hour), 10)
	if err == nil {
		t.Error("Expected error when getting with cancelled context")
	}
}

// TestConcurrentAccess tests that the store handles concurrent access safely
func TestConcurrentAccess(t *testing.T) {
	tempDir := t.TempDir()
	store, err := NewSQLiteStore(tempDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	done := make(chan bool, 2)
	errors := make(chan error, 2)

	// Goroutine 1: Save events
	go func() {
		for i := 0; i < 10; i++ {
			event := Event{
				ID:        fmt.Sprintf("writer1-%d", i),
				Repo:      "github.com/NixOS/nixpkgs",
				Type:      "PushEvent",
				Actor:     "writer1",
				CreatedAt: time.Now(),
				Payload:   json.RawMessage(`{}`),
			}
			if err := store.SaveEvents(ctx, []Event{event}); err != nil {
				errors <- err
				done <- true
				return
			}
		}
		done <- true
	}()

	// Goroutine 2: Save different events
	go func() {
		for i := 0; i < 10; i++ {
			event := Event{
				ID:        fmt.Sprintf("writer2-%d", i),
				Repo:      "github.com/NixOS/nixpkgs",
				Type:      "IssuesEvent",
				Actor:     "writer2",
				CreatedAt: time.Now(),
				Payload:   json.RawMessage(`{}`),
			}
			if err := store.SaveEvents(ctx, []Event{event}); err != nil {
				errors <- err
				done <- true
				return
			}
		}
		done <- true
	}()

	// Wait for both goroutines
	<-done
	<-done

	// Check for errors
	select {
	case err := <-errors:
		t.Fatalf("Concurrent access error: %v", err)
	default:
		// No errors
	}

	// Verify all events were saved
	events, err := store.GetEvents(ctx, "github.com/NixOS/nixpkgs", time.Now().Add(-1*time.Hour), 100)
	if err != nil {
		t.Fatalf("Failed to get events: %v", err)
	}

	if len(events) != 20 {
		t.Errorf("Expected 20 events from concurrent writes, got %d", len(events))
	}
}
