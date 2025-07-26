# Chronixpkgs Quick Start Guide

## For Users

### Get Recent Events (REST API)

```bash
# Last 50 events
curl "https://events.nixos.org/events?limit=50"

# Filter by type
curl "https://events.nixos.org/events?type=PushEvent"

# Events since timestamp
curl "https://events.nixos.org/events?since=2024-01-15T00:00:00Z"
```

### Stream Real-time Events (SSE)

```javascript
const eventSource = new EventSource(
  'https://events.nixos.org/events'
);

eventSource.onmessage = (e) => {
  const event = JSON.parse(e.data);
  console.log('New event:', event.type, event.actor);
};
```

### Advanced Filtering

```bash
# Filter by username
curl "https://events.nixos.org/events?actor=alice"

# Filter by PR number
curl "https://events.nixos.org/events?pr=12345"

# Filter by issue number  
curl "https://events.nixos.org/events?issue=67890"

# Combined filters
curl "https://events.nixos.org/events?type=PullRequestEvent&actor=bob&since=2024-01-15T00:00:00Z"
```

## For Developers

### CLI Usage (No Server Required)

Query events directly from the database:

```bash
# Get recent events
chronixpkgs events --repo NixOS/nixpkgs --data-dir ./data

# Filter by event type
chronixpkgs events --repo NixOS/nixpkgs --type PullRequestEvent

# Filter by actor
chronixpkgs events --repo NixOS/nixpkgs --actor alice

# Filter by PR number
chronixpkgs events --repo NixOS/nixpkgs --pr 12345

# Output as JSON Lines for processing
chronixpkgs events --repo NixOS/nixpkgs --format jsonl | jq '.actor' | sort | uniq -c

# Pretty print for debugging
chronixpkgs events --repo NixOS/nixpkgs --limit 5 --format pretty
```

## For Operators

### Quick Deployment

```bash
# 1. Build
nix build .#chronixpkgs

# 2. Set environment variables (or use flags)
export GITHUB_TOKEN="ghp_your_token_here"

# 3. Run server (read-only API)
./result/bin/chronixpkgs server \
  --listen ":8080" \
  --data-dir "./data" \
  --enable-cors &

# 4. Run fetcher (polling + maintenance)
./result/bin/chronixpkgs fetch \
  --repo "NixOS/nixpkgs" \
  --data-dir "./data" \
  --poll-interval 1m \
  --vacuum-interval 24h
```

### No Webhook Configuration Needed!

Chronixpkgs uses a polling-only approach:
- No webhook setup required
- Fetcher polls GitHub API regularly
- More reliable than webhooks
- Works behind firewalls

## Common Use Cases

### Monitor Pull Requests

```javascript
// Watch for new PRs
eventSource.addEventListener('event', (e) => {
  const event = JSON.parse(e.data);
  if (event.type === 'PullRequestEvent' && 
      event.payload.action === 'opened') {
    notifyNewPR(event.payload.pull_request);
  }
});
```

### Track Releases

```bash
# Get all release events
curl "https://events.nixos.org/events?type=ReleaseEvent"
```

### Analyze Activity

```bash
# REST API: Events by type in last 24h
curl "https://events.nixos.org/events?since=2024-01-14T00:00:00Z&limit=1000" | \
  jq 'group_by(.type) | map({type: .[0].type, count: length})'
```

### Build Dashboard

```python
import requests
import json

# Fetch recent events
resp = requests.get(
    'https://events.nixos.org/events',
    params={
        'limit': 100
    }
)
events = resp.json()

# Count by type
by_type = {}
for event in events:
    by_type[event['type']] = by_type.get(event['type'], 0) + 1

print(json.dumps(by_type, indent=2))
```

## Event Types Reference

| Type | Description | Key Fields |
|------|-------------|------------|
| `PushEvent` | Commits pushed | `commits[]`, `ref` |
| `PullRequestEvent` | PR activity | `action`, `pull_request` |
| `IssuesEvent` | Issue activity | `action`, `issue` |
| `ReleaseEvent` | New release | `release`, `action` |
| `CreateEvent` | Branch/tag created | `ref_type`, `ref` |

## Troubleshooting

### No events appearing?
- Check GitHub token is valid
- Verify fetcher is running
- Check fetcher logs for API errors
- Ensure polling interval isn't too long

### Connection drops?
- SSE will auto-reconnect with last event ID
- Check for rate limiting (60 req/min)

### Need help?
- API Docs: `/docs/API.md`
- Issues: https://github.com/zimbatm/nix-experiments/issues