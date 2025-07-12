# API Reference

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

Retrieve events for the monitored repository.

**Query Parameters:**
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| since | string (RFC3339) | no | Filter events after this time |
| type | string[] | no | Filter by event types (can be repeated) |
| limit | integer | no | Max events to return (default: 100, max: 1000) |

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

**Example:**
```bash
# Get recent push and PR events
curl "https://events.nixos.org/events?type=PushEvent&type=PullRequestEvent&limit=50"
```

#### GET /events/stream

Server-sent events stream for real-time updates from the monitored repository.

**Query Parameters:**
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| last_event_id | string | no | Resume from after this event ID |

**Headers:**
- `Accept: text/event-stream` (required)
- `Last-Event-ID: <id>` (optional, for resumption)

**Response Format:**
```
id: 12345678
event: event
data: {"id":"12345678","type":"PushEvent",...}

id: 12345679
event: event
data: {"id":"12345679","type":"IssuesEvent",...}
```

**Example:**
```javascript
const eventSource = new EventSource('https://events.nixos.org/events/stream');

eventSource.addEventListener('event', (e) => {
  const event = JSON.parse(e.data);
  console.log('New event:', event);
});

// Browser automatically handles reconnection with last event ID
```

### Metadata

#### GET /event-types

List all event types seen in the monitored repository.

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

### GraphQL

#### POST /graphql

GraphQL endpoint for flexible querying.

**Request Body:**
```json
{
  "query": "...",
  "variables": {}
}
```

**Example Query:**
```graphql
query RecentPushEvents {
  events(
    types: ["PushEvent"]
    limit: 10
  ) {
    events {
      id
      actor
      createdAt
      payload
    }
    totalCount
    hasMore
  }
}
```

**GraphiQL Interface:**
Available at `https://events.nixos.org/graphql` in web browsers.

### Health

#### GET /health

Health check endpoint.

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
import { NixOSEvents } from '@nixos/events-client';

const client = new ChronixpkgsEvents();
const events = await client.getEvents({
  types: ['PushEvent'],
  limit: 50
});
```

### Python
```python
from chronixpkgs_events import EventsClient

client = EventsClient()
events = client.get_events(
    types=['PushEvent'],
    limit=50
)
```

### Go
```go
import "chronixpkgs/events-client-go"

client := events.NewClient()
events, err := client.GetEvents(events.QueryOptions{
    Types: []string{"PushEvent"},
    Limit: 50,
})
```

## Examples

### Watch for New Pull Requests
```javascript
const eventSource = new EventSource(
  'https://events.nixos.org/events/stream'
);

eventSource.addEventListener('event', (e) => {
  const event = JSON.parse(e.data);
  if (event.type === 'PullRequestEvent' && event.payload.action === 'opened') {
    console.log(`New PR: ${event.payload.pull_request.title}`);
  }
});
```

### Get Push Events from Last Hour
```bash
curl "https://events.nixos.org/events?type=PushEvent&since=$(date -u -d '1 hour ago' +%Y-%m-%dT%H:%M:%SZ)"
```

### GraphQL: Find Most Active Contributors
```graphql
query ActiveContributors {
  events(
    since: "2024-01-01T00:00:00Z"
    limit: 1000
  ) {
    events {
      actor
      type
    }
  }
}
```

## Webhooks

To receive webhooks when new events arrive:

1. Contact the service administrator
2. Provide your webhook endpoint URL
3. Specify which event types you want
4. Implement webhook signature validation

## Terms of Use

This is a public service for the nixpkgs community. Please:
- Respect rate limits
- Cache responses when appropriate
- Don't use for commercial purposes without permission
- Report issues to https://github.com/zimbatm/nix-experiments/issues