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
  'https://events.nixos.org/events/stream'
);

eventSource.onmessage = (e) => {
  const event = JSON.parse(e.data);
  console.log('New event:', event.type, event.actor);
};
```

### Query with GraphQL

Visit https://events.nixos.org/graphql for interactive GraphiQL interface.

```graphql
{
  events(limit: 10) {
    events {
      id
      type
      actor
      createdAt
    }
  }
}
```

## For Operators

### Quick Deployment

```bash
# 1. Build
nix build .#chronixpkgs

# 2. Set environment variables (or use flags)
export GITHUB_TOKEN="ghp_your_token_here"
export WEBHOOK_SECRET="your_webhook_secret"

# 3. Run with defaults
./result/bin/chronixpkgs

# Or with custom options
./result/bin/chronixpkgs \
  --repo "NixOS/nixpkgs" \
  --listen ":8080" \
  --data-dir "./data" \
  --poll-interval 5m \
  --enable-cors
```

### Configure GitHub Webhook

1. Go to: https://github.com/NixOS/nixpkgs/settings/hooks
2. Add webhook:
   - URL: `https://your-domain/webhook/github`
   - Secret: `your_webhook_secret`
   - Events: "Send me everything"

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

```graphql
# GraphQL: Events by type in last 24h
{
  events(
    since: "2024-01-14T00:00:00Z"
    limit: 1000
  ) {
    events {
      type
      actor
    }
  }
}
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
- Check webhook is configured correctly
- Verify webhook secret matches
- Look at webhook delivery history in GitHub

### Connection drops?
- SSE will auto-reconnect with last event ID
- Check for rate limiting (60 req/min)

### Need help?
- API Docs: `/docs/API.md`
- Issues: https://github.com/zimbatm/nix-experiments/issues