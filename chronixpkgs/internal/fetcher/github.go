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
		repository:   "",              // must be set via SetRepository
		pollInterval: 5 * time.Minute, // default
	}, nil
}

func (f *GitHubFetcher) SetRepository(repo string) {
	f.repository = repo
}

func (f *GitHubFetcher) SetPollInterval(interval time.Duration) {
	f.pollInterval = interval
}

// FetchEvents fetches all events from GitHub until it catches up with the local database
// This is used for both initial backfill and regular polling
func (f *GitHubFetcher) FetchEvents(ctx context.Context, repo string) error {
	log.Printf("FetchEvents called for repo: '%s'", repo)

	// Check if repo is empty
	if repo == "" {
		return fmt.Errorf("repository is empty")
	}

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

	// Get the latest event in our database
	state, err := f.store.GetFetchState(ctx, repo)
	if err != nil {
		return fmt.Errorf("failed to get fetch state: %w", err)
	}

	var targetEventID string
	if state != nil && state.LastEventID != "" {
		targetEventID = state.LastEventID
		log.Printf("Found existing events, will fetch until event ID: %s", targetEventID)
	} else {
		// If database is empty, we'll fetch all available pages
		log.Printf("Database is empty, will fetch all available events")
	}

	page := 1
	allEvents := []storage.Event{}
	foundTarget := false
	seenIDs := make(map[string]bool) // Track seen IDs to handle duplicates

	for {
		// Check context
		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled during fetch: %w", ctx.Err())
		default:
		}

		// Fetch a page of events
		path := fmt.Sprintf("repos/%s/%s/events?page=%d&per_page=100", owner, repoName, page)
		log.Printf("Fetching page %d of events", page)

		var githubEvents []GitHubEvent
		err := f.client.Get(path, &githubEvents)
		if err != nil {
			return fmt.Errorf("failed to fetch events page %d: %w", page, err)
		}

		// If we get an empty page, we've reached the end
		if len(githubEvents) == 0 {
			log.Printf("Reached end of available events at page %d", page)
			break
		}

		// Process events from this page
		pageEvents := []storage.Event{}
		for _, ghEvent := range githubEvents {
			// Skip if we've already seen this ID (handles duplicates across pages)
			if seenIDs[ghEvent.ID] {
				continue
			}
			seenIDs[ghEvent.ID] = true

			// If we found our target event, stop processing
			if targetEventID != "" && ghEvent.ID == targetEventID {
				foundTarget = true
				break
			}

			// Skip private events
			if !ghEvent.Public {
				continue
			}

			// Parse created_at time
			createdAt, err := time.Parse(time.RFC3339, ghEvent.CreatedAt)
			if err != nil {
				log.Printf("Failed to parse time for event %s: %v", ghEvent.ID, err)
				continue
			}

			pageEvents = append(pageEvents, storage.Event{
				ID:        ghEvent.ID,
				Repo:      repo,
				Type:      ghEvent.Type,
				Actor:     ghEvent.Actor.Login,
				CreatedAt: createdAt,
				Payload:   ghEvent.Payload,
			})
		}

		// Add page events to all events (in reverse order for chronological ordering)
		for i := len(pageEvents) - 1; i >= 0; i-- {
			allEvents = append(allEvents, pageEvents[i])
		}

		if foundTarget {
			log.Printf("Found target event on page %d, stopping fetch", page)
			break
		}

		// GitHub limits to 10 pages (300 events total) for unauthenticated requests
		// With authentication, we can go up to 10 pages of 100 events = 1000 events
		if page >= 10 {
			log.Printf("Reached maximum page limit (10 pages)")
			break
		}

		// If this is the first page and we have a target, we're up to date
		if page == 1 && targetEventID != "" && len(pageEvents) == 0 {
			log.Printf("No new events found")
			break
		}

		page++

		// Small delay to be nice to GitHub API
		time.Sleep(100 * time.Millisecond)
	}

	// Save all events in batches
	if len(allEvents) > 0 {
		log.Printf("Saving %d events", len(allEvents))

		// Save in batches of 100 to avoid overwhelming the database
		batchSize := 100
		for i := 0; i < len(allEvents); i += batchSize {
			end := i + batchSize
			if end > len(allEvents) {
				end = len(allEvents)
			}

			batch := allEvents[i:end]
			if err := f.store.SaveEvents(ctx, batch); err != nil {
				return fmt.Errorf("failed to save events batch: %w", err)
			}
		}

		// Update fetch state with the newest event
		if len(allEvents) > 0 {
			newestEvent := allEvents[len(allEvents)-1]
			newState := &storage.FetchState{
				Repo:        repo,
				LastEventID: newestEvent.ID,
				LastFetchAt: time.Now(),
			}
			if err := f.store.UpdateFetchState(ctx, newState); err != nil {
				return fmt.Errorf("failed to update fetch state: %w", err)
			}
		}

		log.Printf("Fetch completed successfully, saved %d events", len(allEvents))
	} else {
		log.Printf("No new events to save")
	}

	return nil
}

func (f *GitHubFetcher) StartPollingTimer(ctx context.Context) {
	log.Printf("Starting polling timer for repository: %s with interval: %v", f.repository, f.pollInterval)

	// Fetch immediately on start (this handles both backfill and regular fetch)
	log.Printf("Performing initial fetch for %s...", f.repository)
	if err := f.FetchEvents(ctx, f.repository); err != nil {
		log.Printf("Failed to fetch events on start: %v", err)
	}

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
