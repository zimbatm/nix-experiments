# Chronixpkgs - Nixpkgs Event Chronicle

A public event stream service for nixpkgs that enables third-party tools to react to repository events without needing webhook access.

## Problem Statement

GitHub webhooks can only be configured by repository owners, making it difficult for third-party developers to build reactive tooling. Additionally, the GitHub Events API has limitations:
- Only retains 90 days of history
- Limited to 300 events per request
- Requires pagination for bulk access

## Solution

Chronixpkgs acts as a public event broker:
1. We configure the webhook on NixOS/nixpkgs (with proper authorization)
2. Chronixpkgs receives all public events via webhook + polling fallback
3. Events are stored permanently (beyond GitHub's 90-day limit)
4. A public API provides unrestricted access to these events

## Features

- Real-time event collection via GitHub webhooks
- 5-minute polling fallback for reliability
- Permanent event storage (no 90-day limit)
- Public REST API with event type filtering
- Server-sent events (SSE) for real-time streaming
- Event type indexing for efficient queries
- Configurable repository monitoring (defaults to nixpkgs)

## Architecture

The service uses a hybrid approach:
1. GitHub webhooks trigger immediate event fetching
2. If no webhook is received for 5 minutes, polling kicks in
3. Events are stored in SQLite for indexing and JSONL files for raw storage
4. Public API allows querying events by repo and time

## API Endpoints

### GET /events
Retrieve events for a repository.

Query parameters:
- `repo`: Repository in format `github.com/owner/repo` (required)
- `since`: RFC3339 timestamp to filter events after this time
- `type`: Event type filter (can be specified multiple times)
- `limit`: Maximum number of events to return (default: 100, max: 1000)

Examples:
```bash
# Get all recent events
curl "http://localhost:8080/events?limit=50"

# Get events since a specific time
curl "http://localhost:8080/events?since=2024-01-01T00:00:00Z"

# Filter by event types
curl "http://localhost:8080/events?type=PushEvent&type=PullRequestEvent"
```

### GET /events/stream
Server-sent events stream for real-time updates with automatic resumption support.

Query parameters:
- `last_event_id` (optional): Resume from after this event ID

Headers:
- `Last-Event-ID`: Alternative way to specify the last event ID (standard SSE)

Features:
- Each event includes an `id` field for resumption
- If connection drops, clients can reconnect with the last received event ID
- Server will send all missed events before continuing with live updates
- Events are sent in chronological order (oldest first)

Examples:
```bash
# Start streaming from current time
curl -H "Accept: text/event-stream" \
  "http://localhost:8080/events/stream"

# Resume from a specific event ID
curl -H "Accept: text/event-stream" \
  "http://localhost:8080/events/stream?last_event_id=12345678"

# Using EventSource in JavaScript (automatic resumption)
const eventSource = new EventSource('/events/stream');
eventSource.addEventListener('event', (e) => {
  const event = JSON.parse(e.data);
  console.log('Received event:', event);
  // Browser automatically tracks e.lastEventId for resumption
});
```

### GET /health
Health check endpoint.

Example:
```bash
curl http://localhost:8080/health
# {"status":"ok"}
```

## Installation

### Using Nix
```bash
nix run github:zimbatm/nix-experiments?dir=chronixpkgs -- --help
```

### From Source
```bash
git clone https://github.com/zimbatm/nix-experiments
cd nix-experiments/chronixpkgs
go build .
```

## Usage

```bash
# Set required environment variables
export GITHUB_TOKEN="ghp_your_token_here"
export WEBHOOK_SECRET="your_webhook_secret"

# Run with defaults (monitors NixOS/nixpkgs)
./chronixpkgs

# Run with custom options
./chronixpkgs \
  --repo "owner/repo" \
  --listen ":8080" \
  --data-dir "./data" \
  --retention-days 90 \
  --poll-interval 5m \
  --enable-cors \
  --enable-rate-limit
```

### Command-line Options

| Flag | Description | Default |
|------|-------------|---------|
| `--github-token` | GitHub API token (or use GITHUB_TOKEN env) | Required |
| `--webhook-secret` | GitHub webhook secret (or use WEBHOOK_SECRET env) | Required |
| `--repo` | GitHub repository to monitor | NixOS/nixpkgs |
| `--listen` | HTTP server address | :8080 |
| `--data-dir` | Storage directory | ./data |
| `--retention-days` | Days to retain events | 90 |
| `--poll-interval` | GitHub polling interval | 5m |
| `--vacuum-interval` | Database maintenance interval | 24h |
| `--enable-cors` | Enable CORS support | false |
| `--cors-origins` | Allowed CORS origins (comma-separated) | * |
| `--enable-rate-limit` | Enable rate limiting | false |
| `--rate-limit` | Requests per minute per IP | 60 |

## Deployment

See [docs/DEPLOYMENT.md](docs/DEPLOYMENT.md) for production deployment instructions.

## Example Applications

```bash
curl "http://localhost:8080/health"
```

### GET /event-types
List all event types for nixpkgs.

Example:
```bash
curl "http://localhost:8080/event-types"
```

### POST /webhook/github
GitHub webhook endpoint. Configure this in your repository settings.

### POST /graphql
GraphQL endpoint for flexible querying. GraphiQL interface available at `/graphql` in browser.

#### Query Examples

```graphql
# Get recent events
query RecentEvents {
  events(limit: 50) {
    events {
      id
      type
      actor
      createdAt
      payload
    }
    totalCount
    hasMore
  }
}

# Get events by type since a specific time
query FilteredEvents {
  events(
    types: ["PushEvent", "PullRequestEvent"]
    since: "2024-01-01T00:00:00Z"
    limit: 100
  ) {
    events {
      id
      type
      actor
      createdAt
    }
    hasMore
  }
}

# Resume from a specific event ID
query ResumeEvents {
  events(
    afterId: "12345678"
    limit: 50
  ) {
    events {
      id
      type
      createdAt
    }
  }
}

# List event types
query Metadata {
  eventTypes
}
```

GraphQL provides several advantages:
- Flexible field selection (only fetch what you need)
- Multiple queries in a single request
- Strong typing with schema introspection
- Better tooling support

## Setup

### Building

```bash
go build -o chronixpkgs ./cmd/chronixpkgs
```

### Configuration

The service is configured via TOML file. See `config.example.toml` for a complete example.

```bash
# Copy and edit the example config
cp config.example.toml config.toml

# Run with config file
./chronixpkgs --config config.toml
```

Key configuration sections:
- **server**: HTTP server settings, CORS, rate limiting
- **storage**: Data directory, retention, backup settings  
- **github**: API token, rate limits, retry configuration
- **repositories**: List of repos to monitor with individual settings
- **webhook**: Webhook path, validation, IP allowlisting

### GitHub Webhook Configuration

1. Go to your repository settings → Webhooks
2. Add webhook:
   - Payload URL: `https://your-domain/webhook/github`
   - Content type: `application/json`
   - Secret: Your webhook secret
   - Events: Select "Send me everything"

### Nix Deployment

The project includes a NixOS module for easy deployment:

```nix
{
  imports = [ chronixpkgs.nixosModules.default ];

  services.chronixpkgs = {
    enable = true;
    listenAddress = "127.0.0.1:8080";
    webhookSecretFile = "/run/secrets/github-webhook-secret";
    githubTokenFile = "/run/secrets/github-token"; # Optional
  };
}
```

## Development

```bash
nix develop
go run ./cmd/chronixpkgs --addr :8080 --data ./data
```

## Data Storage

Events are stored in two formats:
- SQLite database (`data/events.db`) for indexing and queries
- JSONL files (`data/raw/YYYY/MM/DD/repo.jsonl`) for raw event storage

## Security Notes

- Webhook signatures are validated using HMAC-SHA256
- Private events are filtered out
- Sensitive data (tokens, emails) should be filtered before storage
- Use HTTPS in production for webhook endpoints

## Future Plans

### Parquet/S3 Storage
For long-term durability and analytics, we plan to:
1. Export historical data as Parquet files
2. Store on S3-compatible object storage
3. Provide bulk download capabilities
4. Enable efficient analytical queries

### Event ID Considerations
GitHub event IDs are monotonically increasing but not sequential (they're global across all GitHub events). We use them for:
- Deduplication during fetching
- Resumption tokens
- Ordering within API responses

However, we index by timestamp for user queries since:
- IDs have gaps making range queries inefficient
- Timestamps are more intuitive for users
- Time-based queries are the primary use case

## License

MIT