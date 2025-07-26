# API Reference

Chronixpkgs is a single-repository event monitoring service. Each instance monitors one specific GitHub repository configured at startup.

## Base URL

```
https://events.nixos.org
```

## Authentication

Currently, all endpoints are public and do not require authentication.

## Rate Limiting

- 60 requests per minute per IP (burst: 10)
- Rate limit headers included in responses:
  - `X-RateLimit-Limit`
  - `X-RateLimit-Remaining`
  - `X-RateLimit-Reset`

## Endpoints

### Events

#### GET /events

Retrieve events from the monitored repository. This endpoint supports both regular JSON responses and Server-Sent Events (SSE) streaming based on the Accept header.

**Query Parameters:**
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| since | string | no | Filter events after this time (RFC3339) or event ID |
| type | string[] | no | Filter by event types (can be repeated) |
| actor | string | no | Filter by GitHub username |
| pr | integer | no | Filter by pull request number |
| issue | integer | no | Filter by issue number |
| limit | integer | no | Max events to return (default: 100, max: 1000) - non-SSE only |
| offset | integer | no | Pagination offset - non-SSE only |
| last_event_id | string | no | Resume SSE stream from after this event ID |

**Response:**
```json
[
  {
    "id": "12345678",
    "repository": "nixpkgs",
    "type": "PushEvent",
    "actor": "user123",
    "created_at": "2024-01-15T10:30:00Z",
    "payload": {...}
  }
]
```

**Headers:**
- `Accept: text/event-stream` - Request SSE streaming (optional)
- `Last-Event-ID: <id>` - Resume SSE from specific event (optional)

**Example (JSON):**
```bash
# Get recent events
curl "https://events.nixos.org/events"

# Get events since a specific time
curl "https://events.nixos.org/events?since=2024-01-01T00:00:00Z"

# Get events since a specific event ID
curl "https://events.nixos.org/events?since=12345678"

# Filter by event type
curl "https://events.nixos.org/events?type=PullRequestEvent&type=IssuesEvent"

# Filter by actor
curl "https://events.nixos.org/events?actor=alice"

# Filter by PR number
curl "https://events.nixos.org/events?pr=12345"

# Filter by issue number
curl "https://events.nixos.org/events?issue=67890"

# Combined filters with pagination
curl "https://events.nixos.org/events?type=PullRequestEvent&actor=alice&limit=50&offset=100"
```

**Example (SSE):**
```bash
# Stream all events
curl -H "Accept: text/event-stream" "https://events.nixos.org/events"

# Stream filtered events
curl -H "Accept: text/event-stream" "https://events.nixos.org/events?type=PushEvent&actor=bob"

# Resume streaming from last event
curl -H "Accept: text/event-stream" -H "Last-Event-ID: 12345678" "https://events.nixos.org/events"
```

**SSE Response Format:**
When using `Accept: text/event-stream`, the response format is:
```
id: 12345678
event: event
data: {"id":"12345678","type":"PushEvent",...}

id: 12345679
event: event
data: {"id":"12345679","type":"IssuesEvent",...}
```

**JavaScript SSE Example:**
```javascript
// Basic streaming
const eventSource = new EventSource('https://events.nixos.org/events');

eventSource.addEventListener('event', (e) => {
  const event = JSON.parse(e.data);
  console.log('New event:', event);
});

// Filtered streaming
const prStream = new EventSource(
  'https://events.nixos.org/events?type=PullRequestEvent&pr=12345'
);

prStream.addEventListener('event', (e) => {
  const event = JSON.parse(e.data);
  console.log('PR update:', event.payload.action);
});

// Browser automatically handles reconnection with last event ID
```

### Metadata

#### GET /event-types

List all available event types for the monitored repository.

**Query Parameters:**
None

**Response:**
```json
[
  "CommitCommentEvent",
  "CreateEvent",
  "DeleteEvent",
  "ForkEvent",
  "IssueCommentEvent",
  "IssuesEvent",
  "PullRequestEvent",
  "PullRequestReviewEvent",
  "PullRequestReviewCommentEvent",
  "PushEvent",
  "ReleaseEvent",
  "WatchEvent"
]
```

**Example:**
```bash
curl "https://events.nixos.org/event-types"
```

### Health

#### GET /health

Health check endpoint.

**Query Parameters:**
None

**Response:**
```json
{
  "status": "ok"
}
```

## Event Types

Common GitHub event types you'll encounter:

| Event Type | Description |
|------------|-------------|
| PushEvent | Commits pushed to a branch |
| PullRequestEvent | PR opened, closed, merged, etc. |
| IssuesEvent | Issue opened, closed, etc. |
| IssueCommentEvent | Comment on an issue |
| PullRequestReviewEvent | PR review submitted |
| PullRequestReviewCommentEvent | Comment on PR code |
| CreateEvent | Branch or tag created |
| DeleteEvent | Branch or tag deleted |
| ForkEvent | Repository forked |
| ReleaseEvent | Release published |
| WatchEvent | Repository starred |

## Payload Structure

Event payloads follow GitHub's webhook payload format. See [GitHub's webhook documentation](https://docs.en/webhooks/webhook-events-and-payloads) for detailed payload schemas.

## Error Responses

```json
{
  "error": "Invalid request parameters",
  "code": "INVALID_REQUEST"
}
```

**HTTP Status Codes:**
- 200: Success
- 400: Bad Request
- 404: Not Found
- 429: Rate Limited
- 500: Internal Server Error

## Client Libraries

### JavaScript/TypeScript
```javascript
import { ChronixpkgsClient } from '@chronixpkgs/client';

const client = new ChronixpkgsClient('https://events.nixos.org');
const events = await client.getEvents({
  types: ['PushEvent'],
  limit: 50
});
```

### Python
```python
from chronixpkgs_client import ChronixpkgsClient

client = ChronixpkgsClient('https://events.nixos.org')
events = client.get_events(
    types=['PushEvent'],
    limit=50
)
```

### Go
```go
import "github.com/zimbatm/chronixpkgs/client-go"

client := chronixpkgs.NewClient("https://events.nixos.org")
events, err := client.GetEvents(chronixpkgs.QueryOptions{
    Types: []string{"PushEvent"},
    Limit: 50,
})
```

## Examples

### Watch for New Pull Requests
```javascript
const eventSource = new EventSource(
  'https://events.nixos.org/events?type=PullRequestEvent'
);

eventSource.addEventListener('event', (e) => {
  const event = JSON.parse(e.data);
  if (event.payload.action === 'opened') {
    console.log(`New PR: ${event.payload.pull_request.title}`);
  }
});
```

### Monitor Specific PR Activity
```javascript
// Watch all activity on PR #12345
const prStream = new EventSource(
  'https://events.nixos.org/events?pr=12345'
);

prStream.addEventListener('event', (e) => {
  const event = JSON.parse(e.data);
  console.log(`PR #12345 activity: ${event.type} - ${event.payload.action}`);
});
```

### Track User Activity
```javascript
// Monitor all events from a specific user
const userStream = new EventSource(
  'https://events.nixos.org/events?actor=alice'
);

userStream.addEventListener('event', (e) => {
  const event = JSON.parse(e.data);
  console.log(`Alice's activity: ${event.type}`);
});
```

### Get Push Events from Last Hour
```bash
curl "https://events.nixos.org/events?type=PushEvent&since=$(date -u -d '1 hour ago' +%Y-%m-%dT%H:%M:%SZ)"
```

### Find Most Active Contributors
```bash
# Get events and process with jq to find active contributors
curl "https://events.nixos.org/events?since=2024-01-01T00:00:00Z&limit=1000" | \
  jq 'group_by(.actor) | map({actor: .[0].actor, count: length}) | sort_by(.count) | reverse'
```

## Real-time Updates

For real-time updates, use the `/events` endpoint with `Accept: text/event-stream` header. This provides a persistent connection that automatically receives new events as they're fetched from GitHub.

The system uses a polling-based architecture for reliability - no webhooks are required or supported.

## Single Repository Architecture

Each chronixpkgs instance monitors a single GitHub repository configured at startup. This design ensures:
- Optimized performance for high-traffic repositories
- Simplified caching and data management
- Predictable resource usage
- Easy horizontal scaling for multiple repositories

## Terms of Use

This is a public service for the nixpkgs community. Please:
- Respect rate limits
- Cache responses when appropriate
- Don't use for commercial purposes without permission
- Report issues to https://github.com/zimbatm/nix-experiments/issues