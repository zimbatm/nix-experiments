package fetcher

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/zimbatm/nix-experiments/chronixpkgs/internal/storage"
)

type GitHubFetcher struct {
	client       *api.RESTClient
	store        storage.Store
	pollingTimer *time.Timer
	timerMu      sync.Mutex
	repository   string
	pollInterval time.Duration
}

type GitHubEvent struct {
	ID        string          `json:"id"`
	Type      string          `json:"type"`
	Actor     Actor           `json:"actor"`
	Repo      Repository      `json:"repo"`
	Payload   json.RawMessage `json:"payload"`
	Public    bool            `json:"public"`
	CreatedAt string          `json:"created_at"`
}

type Actor struct {
	Login string `json:"login"`
}

type Repository struct {
	Name string `json:"name"`
}

func NewGitHubFetcher(token string, store storage.Store) (*GitHubFetcher, error) {
	opts := api.ClientOptions{}
	if token != "" {
		opts.AuthToken = token
	}

	client, err := api.NewRESTClient(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create GitHub client: %w", err)
	}

	return &GitHubFetcher{
		client:       client,
		store:        store,
		repository:   "", // must be set via SetRepository
		pollInterval: 5 * time.Minute, // default
	}, nil
}

func (f *GitHubFetcher) SetRepository(repo string) {
	f.repository = repo
}

func (f *GitHubFetcher) SetPollInterval(interval time.Duration) {
	f.pollInterval = interval
}

func (f *GitHubFetcher) FetchEvents(ctx context.Context, repo string) error {
	// Check context before starting
	select {
	case <-ctx.Done():
		return fmt.Errorf("context cancelled before fetch: %w", ctx.Err())
	default:
	}

	// Extract owner/repo from repo string (format: github.com/owner/repo)
	if !strings.HasPrefix(repo, "github.com/") {
		return fmt.Errorf("invalid repo format: %s (expected github.com/owner/repo)", repo)
	}

	repoPath := strings.TrimPrefix(repo, "github.com/")
	parts := strings.Split(repoPath, "/")
	if len(parts) != 2 {
		return fmt.Errorf("invalid repo format: %s (expected github.com/owner/repo)", repo)
	}
	owner, repoName := parts[0], parts[1]

	// Get last fetch state
	state, err := f.store.GetFetchState(ctx, repo)
	if err != nil {
		return fmt.Errorf("failed to get fetch state: %w", err)
	}

	// Fetch events from GitHub with timeout
	path := fmt.Sprintf("repos/%s/%s/events", owner, repoName)

	var githubEvents []GitHubEvent

	// Create a channel to receive the result
	type result struct {
		events []GitHubEvent
		err    error
	}
	resultCh := make(chan result, 1)

	// Run the API call in a goroutine
	go func() {
		var events []GitHubEvent
		err := f.client.Get(path, &events)
		resultCh <- result{events: events, err: err}
	}()

	// Wait for result or context cancellation
	select {
	case <-ctx.Done():
		return fmt.Errorf("context cancelled during fetch: %w", ctx.Err())
	case res := <-resultCh:
		if res.err != nil {
			return fmt.Errorf("failed to fetch events: %w", res.err)
		}
		githubEvents = res.events
	}

	// Convert to storage events
	var events []storage.Event
	var latestEventID string

	for _, ghEvent := range githubEvents {
		// Check context periodically
		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled during processing: %w", ctx.Err())
		default:
		}

		// Skip if we've seen this event before
		if state != nil && ghEvent.ID == state.LastEventID {
			break
		}

		// Skip private events
		if !ghEvent.Public {
			continue
		}

		// Parse created_at time
		createdAt, err := time.Parse(time.RFC3339, ghEvent.CreatedAt)
		if err != nil {
			return fmt.Errorf("failed to parse time for event %s: %w", ghEvent.ID, err)
		}

		events = append(events, storage.Event{
			ID:        ghEvent.ID,
			Repo:      repo,
			Type:      ghEvent.Type,
			Actor:     ghEvent.Actor.Login,
			CreatedAt: createdAt,
			Payload:   ghEvent.Payload,
		})

		if latestEventID == "" {
			latestEventID = ghEvent.ID
		}
	}

	// Save events (newest first, so reverse the order)
	if len(events) > 0 {
		// Reverse the slice
		for i, j := 0, len(events)-1; i < j; i, j = i+1, j-1 {
			events[i], events[j] = events[j], events[i]
		}

		if err := f.store.SaveEvents(ctx, events); err != nil {
			return fmt.Errorf("failed to save events: %w", err)
		}

		// Update fetch state
		newState := &storage.FetchState{
			Repo:        repo,
			LastEventID: latestEventID,
			LastFetchAt: time.Now(),
		}
		if err := f.store.UpdateFetchState(ctx, newState); err != nil {
			return fmt.Errorf("failed to update fetch state: %w", err)
		}

		log.Printf("Fetched %d new events for %s", len(events), repo)
	}

	return nil
}

func (f *GitHubFetcher) StartPollingTimer(ctx context.Context) {
	f.timerMu.Lock()
	f.resetTimer()
	f.timerMu.Unlock()

	for {
		f.timerMu.Lock()
		timer := f.pollingTimer
		f.timerMu.Unlock()

		select {
		case <-ctx.Done():
			f.timerMu.Lock()
			if f.pollingTimer != nil {
				f.pollingTimer.Stop()
			}
			f.timerMu.Unlock()
			return
		case <-timer.C:
			log.Printf("Polling timer expired, fetching events for %s...\n", f.repository)
			if err := f.FetchEvents(ctx, f.repository); err != nil {
				log.Printf("Failed to fetch events during polling: %v", err)
			}
			f.timerMu.Lock()
			f.resetTimer()
			f.timerMu.Unlock()
		}
	}
}

func (f *GitHubFetcher) ResetPollingTimer() {
	f.timerMu.Lock()
	defer f.timerMu.Unlock()

	if f.pollingTimer != nil {
		f.pollingTimer.Stop()
	}
	f.resetTimer()
}

func (f *GitHubFetcher) resetTimer() {
	// Note: This should only be called when timerMu is already locked
	f.pollingTimer = time.NewTimer(f.pollInterval)
}

// StartPolling starts the polling timer
func (f *GitHubFetcher) StartPolling(ctx context.Context) {
	go f.StartPollingTimer(ctx)
}
