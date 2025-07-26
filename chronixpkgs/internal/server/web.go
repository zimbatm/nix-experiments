package server

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/zimbatm/nix-experiments/chronixpkgs/internal/storage"
)

// EventWithJSON wraps Event with formatted JSON for templates
type EventWithJSON struct {
	storage.Event
	PayloadJSON string
	PayloadMap  map[string]interface{}
	RepoOwner   string
	RepoName    string
}

// RegisterWebRoutes adds the simplified web UI routes to the server
func (s *Server) RegisterWebRoutes() {
	// Serve static files
	s.mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// Web UI routes - simplified
	s.mux.HandleFunc("/", s.handleHome)
	s.mux.HandleFunc("/events-partial", s.handleEventsPartial)
	s.mux.HandleFunc("/stream", s.handleStreamPage)
}

func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	// Parse the simple home template
	templates := template.New("home_simple.html")
	_, err := templates.ParseFiles("templates/base.html", "templates/home_simple.html")
	if err != nil {
		http.Error(w, "failed to load templates", http.StatusInternalServerError)
		return
	}

	err = templates.ExecuteTemplate(w, "home_simple.html", nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (s *Server) handleEventsPartial(w http.ResponseWriter, r *http.Request) {
	repo := s.getMonitoredRepo()
	ctx := r.Context()

	// Parse parameters
	limit := 50 // Default to 50 events
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	// Get recent events from the last 7 days
	filter := storage.EventFilter{
		Repo:  repo,
		Since: time.Now().AddDate(0, 0, -7),
		Limit: limit + 1,
	}
	events, err := s.store.GetEventsFiltered(ctx, filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Check if there are more events
	hasMore := len(events) > limit
	if hasMore {
		events = events[:limit]
	}

	// Convert events to include formatted JSON
	eventsWithJSON := make([]EventWithJSON, len(events))
	for i, event := range events {
		var payload map[string]interface{}
		json.Unmarshal(event.Payload, &payload)

		jsonBytes, _ := json.MarshalIndent(payload, "", "  ")

		// Extract repo owner and name from repo field (format: github.com/owner/name)
		repoParts := strings.Split(event.Repo, "/")
		owner, name := "", ""
		if len(repoParts) >= 3 {
			owner = repoParts[1]
			name = repoParts[2]
		}

		eventsWithJSON[i] = EventWithJSON{
			Event:       event,
			PayloadJSON: string(jsonBytes),
			PayloadMap:  payload,
			RepoOwner:   owner,
			RepoName:    name,
		}
	}

	// Prepare data for template
	data := struct {
		Events  []EventWithJSON
		HasMore bool
	}{
		Events:  eventsWithJSON,
		HasMore: hasMore,
	}

	// Add template functions
	funcMap := template.FuncMap{
		"lower": func(s string) string {
			return strings.ToLower(s)
		},
		"refName": func(ref string) string {
			// Extract branch/tag name from refs/heads/branch or refs/tags/tag
			if len(ref) > 11 && ref[:11] == "refs/heads/" {
				return ref[11:]
			}
			if len(ref) > 10 && ref[:10] == "refs/tags/" {
				return ref[10:]
			}
			return ref
		},
		"trunc": func(length int, s string) string {
			if len(s) <= length {
				return s
			}
			return s[:length]
		},
		"timeAgo": func(t time.Time) string {
			diff := time.Since(t)

			switch {
			case diff < time.Minute:
				return "just now"
			case diff < time.Hour:
				mins := int(diff.Minutes())
				if mins == 1 {
					return "1 minute ago"
				}
				return fmt.Sprintf("%d minutes ago", mins)
			case diff < 24*time.Hour:
				hours := int(diff.Hours())
				if hours == 1 {
					return "1 hour ago"
				}
				return fmt.Sprintf("%d hours ago", hours)
			case diff < 30*24*time.Hour:
				days := int(diff.Hours() / 24)
				if days == 1 {
					return "1 day ago"
				}
				return fmt.Sprintf("%d days ago", days)
			case diff < 365*24*time.Hour:
				months := int(diff.Hours() / 24 / 30)
				if months == 1 {
					return "1 month ago"
				}
				return fmt.Sprintf("%d months ago", months)
			default:
				years := int(diff.Hours() / 24 / 365)
				if years == 1 {
					return "1 year ago"
				}
				return fmt.Sprintf("%d years ago", years)
			}
		},
	}

	// Parse the events cards template
	templates := template.New("events_cards.html").Funcs(funcMap)
	_, err = templates.ParseFiles("templates/events_cards.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = templates.ExecuteTemplate(w, "events_cards.html", data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (s *Server) handleStreamPage(w http.ResponseWriter, r *http.Request) {
	// Parse the HTMX stream template
	templates := template.New("stream_htmx.html")
	_, err := templates.ParseFiles("templates/base.html", "templates/stream_htmx.html")
	if err != nil {
		http.Error(w, "failed to load templates", http.StatusInternalServerError)
		return
	}

	err = templates.ExecuteTemplate(w, "stream_htmx.html", nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
