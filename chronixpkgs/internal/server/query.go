package server

import (
	"fmt"
	"net/http"
	"strconv"
	"time"
)

func (s *Server) parseEventQueryParams(r *http.Request) eventQueryParams {
	params := eventQueryParams{}

	// Parse event types
	params.eventTypes = r.URL.Query()["type"]
	if len(params.eventTypes) > 50 {
		params.err = fmt.Errorf("too many event types specified (max 50)")
		return params
	}

	// Parse since parameter (supports both timestamp and event ID)
	sinceStr := r.URL.Query().Get("since")
	if sinceStr != "" {
		// Try parsing as timestamp first
		if parsedTime, err := time.Parse(time.RFC3339, sinceStr); err == nil {
			// Reject future dates
			if parsedTime.After(time.Now()) {
				params.err = fmt.Errorf("since parameter cannot be in the future")
				return params
			}
			params.sinceTime = &parsedTime
		} else {
			// Assume it's an event ID
			params.sinceID = sinceStr
		}
	}

	// Parse actor filter
	params.actor = r.URL.Query().Get("actor")

	// Parse PR number filter
	prStr := r.URL.Query().Get("pr")
	if prStr != "" {
		pr, err := strconv.Atoi(prStr)
		if err != nil || pr <= 0 {
			params.err = fmt.Errorf("invalid pr parameter")
			return params
		}
		params.prNumber = pr
	}

	// Parse issue number filter
	issueStr := r.URL.Query().Get("issue")
	if issueStr != "" {
		issue, err := strconv.Atoi(issueStr)
		if err != nil || issue <= 0 {
			params.err = fmt.Errorf("invalid issue parameter")
			return params
		}
		params.issueNumber = issue
	}

	return params
}
