package storage

import (
	"context"
	"encoding/json"
	"time"
)

type Event struct {
	ID        string          `json:"id"`
	Repo      string          `json:"repo"`
	Type      string          `json:"type"`
	Actor     string          `json:"actor"`
	CreatedAt time.Time       `json:"created_at"`
	Payload   json.RawMessage `json:"payload"`
	RawFile   string          `json:"-"`
	RawOffset int64           `json:"-"`
}

type FetchState struct {
	Repo        string    `json:"repo"`
	LastEventID string    `json:"last_event_id"`
	LastFetchAt time.Time `json:"last_fetch_at"`
	ETag        string    `json:"etag"`
}

type EventFilter struct {
	Repo       string
	Since      time.Time
	EventTypes []string
	Limit      int
}

type Store interface {
	SaveEvents(ctx context.Context, events []Event) error
	GetEvents(ctx context.Context, repo string, since time.Time, limit int) ([]Event, error)
	GetEventsFiltered(ctx context.Context, filter EventFilter) ([]Event, error)
	GetEventsAfterId(ctx context.Context, repo string, afterID string, limit int) ([]Event, error)
	GetFetchState(ctx context.Context, repo string) (*FetchState, error)
	UpdateFetchState(ctx context.Context, state *FetchState) error
	ListEventTypes(ctx context.Context, repo string) ([]string, error)
	Close() error
}

// MaintenanceStore provides database maintenance operations
type MaintenanceStore interface {
	Store
	ArchiveOldPartitions(ctx context.Context, days int) error
	Optimize(ctx context.Context) error
}
