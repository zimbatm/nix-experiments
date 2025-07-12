package webhook

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/zimbatm/nix-experiments/chronixpkgs/internal/utils"
)

type Handler struct {
	secret         string
	eventFetcher   EventFetcher
	allowedIPs     []string
	maxPayloadSize int64
}

type EventFetcher interface {
	FetchEvents(ctx context.Context, repo string) error
	ResetPollingTimer()
}

type WebhookPayload struct {
	Repository struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
	Action string `json:"action"`
}

func NewHandler(secret string, fetcher EventFetcher) *Handler {
	if secret == "" {
		log.Println("WARNING: Webhook secret is empty, webhook signature verification will fail")
	}
	return &Handler{
		secret:         secret,
		eventFetcher:   fetcher,
		maxPayloadSize: 10 * 1024 * 1024, // 10MB default
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check IP allowlist if configured
	if !h.isIPAllowed(r) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Limit request body size
	r.Body = http.MaxBytesReader(w, r.Body, h.maxPayloadSize)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}

	// Verify signature
	signature := r.Header.Get("X-Hub-Signature-256")
	if !h.verifySignature(body, signature) {
		http.Error(w, "Invalid signature", http.StatusUnauthorized)
		return
	}

	// Parse payload to get repository
	var payload WebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Convert repository full name to repo
	repo := fmt.Sprintf("github.com/%s", payload.Repository.FullName)

	// Reset polling timer since we received a webhook
	h.eventFetcher.ResetPollingTimer()

	// Trigger event fetch asynchronously with timeout
	go func() {
		// Use a timeout context to prevent hanging goroutines
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		if err := h.eventFetcher.FetchEvents(ctx, repo); err != nil {
			// Log error but don't fail the webhook
			if err == context.DeadlineExceeded {
				log.Printf("Fetch timeout for %s after 5 minutes", repo)
			} else {
				log.Printf("Failed to fetch events for %s: %v", repo, err)
			}
		}
	}()

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusAccepted)
	w.Write([]byte("OK"))
}

func (h *Handler) verifySignature(payload []byte, signature string) bool {
	if signature == "" {
		return false
	}

	parts := strings.SplitN(signature, "=", 2)
	if len(parts) != 2 || parts[0] != "sha256" {
		return false
	}

	expectedMAC, err := hex.DecodeString(parts[1])
	if err != nil {
		return false
	}

	mac := hmac.New(sha256.New, []byte(h.secret))
	mac.Write(payload)
	actualMAC := mac.Sum(nil)

	return hmac.Equal(expectedMAC, actualMAC)
}

// isIPAllowed checks if the request IP is in the allowed list
func (h *Handler) isIPAllowed(r *http.Request) bool {
	// If no IPs are configured, allow all
	if len(h.allowedIPs) == 0 {
		return true
	}

	// Extract client IP
	host := utils.GetClientIP(r)

	// Check against allowed IPs
	for _, allowedIP := range h.allowedIPs {
		if allowedIP == host {
			return true
		}
		// Check CIDR ranges
		if strings.Contains(allowedIP, "/") {
			_, ipnet, err := net.ParseCIDR(allowedIP)
			if err == nil && ipnet.Contains(net.ParseIP(host)) {
				return true
			}
		}
	}

	return false
}
