# Configuration Reference

This document details all configuration options for chronixpkgs components.

## Configuration Methods

Chronixpkgs can be configured using:

1. **Configuration file** (TOML format) - Recommended for production
2. **Environment file** (.env) - Good for local development and secrets
3. **Environment variables** - Good for containers and CI/CD
4. **Command-line flags** - Override specific settings

Settings are applied in this order (later overrides earlier):
1. Default values
2. .env file (if present)
3. Configuration file (TOML)
4. Environment variables  
5. Command-line flags

## Environment File (.env)

### Generate Example .env

```bash
chronixpkgs config init-env
```

This creates a `.env.example` file. Copy it to `.env` and fill in your values:

```bash
cp .env.example .env
vim .env
```

### Example .env File

```bash
# Repository to monitor
CHRONIXPKGS_REPO=NixOS/nixpkgs

# Data directory
CHRONIXPKGS_DATA_DIR=./data

# GitHub API token (required for fetcher)
GITHUB_TOKEN=ghp_your_token_here

# S3 Configuration (optional)
AWS_ACCESS_KEY_ID=your_access_key
AWS_SECRET_ACCESS_KEY=your_secret_key
S3_ENDPOINT=https://s3.amazonaws.com
S3_BUCKET=chronixpkgs-events
```

### .env File Locations

The .env file is searched in these locations (in order):
- `./.env`
- `./.env.local`
- `~/.config/chronixpkgs/.env`

## Configuration File (TOML)

### Generate Example Config

```bash
chronixpkgs config init
```

This creates a `chronixpkgs.toml` file with all available options.

### Config File Locations

The configuration file is searched in these locations (in order):
- `./chronixpkgs.toml`
- `./config.toml`
- `~/.config/chronixpkgs/config.toml`
- `/etc/chronixpkgs/config.toml`

Or specify a custom path:
```bash
chronixpkgs --config /path/to/config.toml [command]
```

### Example Configuration

```toml
# Repository to monitor (required)
repository = "NixOS/nixpkgs"

[storage]
data_dir = "./data"
retention_days = 0  # 0 = keep forever
vacuum_interval = "24h"

[server]
listen = ":8080"
enable_rate_limit = true
rate_limit_per_minute = 60
rate_limit_burst = 10
default_limit = 100
max_limit = 1000
sse_interval = "10s"

[fetcher]
# github_token = "ghp_..."  # Better to use GITHUB_TOKEN env var
poll_interval = "1m"
once = false

[s3]
enabled = false
endpoint = "https://s3.amazonaws.com"
bucket = "chronixpkgs-events"
# access_key = ""  # Use AWS_ACCESS_KEY_ID env var
# secret_key = ""  # Use AWS_SECRET_ACCESS_KEY env var
export_interval = "1h"

[observability]
# otel_endpoint = "localhost:4317"
service_name = "chronixpkgs"
```

## Environment Variables

| Variable | Description | Required | Used By |
|----------|-------------|----------|---------|
| `GITHUB_TOKEN` | GitHub personal access token for API requests | Yes | fetcher |
| `CHRONIXPKGS_REPO` | Repository to monitor | No | all |
| `CHRONIXPKGS_DATA_DIR` | Data directory path | No | all |
| `CHRONIXPKGS_LISTEN` | Server listen address | No | server |
| `AWS_ACCESS_KEY_ID` | S3 access key (standard AWS env var) | No | fetcher, archive |
| `AWS_SECRET_ACCESS_KEY` | S3 secret key (standard AWS env var) | No | fetcher, archive |

## Command Line Arguments

### Server Command (Read-only API)

The server provides read-only access to stored events.

| Flag | Description | Default |
|------|-------------|---------|
| `--listen` | Listen address | `:8080` |
| `--data-dir` | Data directory path | `./data` |
| `--enable-rate-limit` | Enable rate limiting | `true` |
| `--rate-limit` | Requests per minute per IP | `100` |
| `--enable-cors` | Enable CORS headers | `true` |
| `--cors-origins` | Allowed CORS origins | `["*"]` |

Example:
```bash
./chronixpkgs server --listen :8080 --data-dir /var/lib/chronixpkgs
```

### Fetch Command (Data ingestion)

The fetcher polls GitHub Events API and maintains the database.

| Flag | Description | Default |
|------|-------------|---------|
| `--repo` | Repository to monitor (format: `owner/repo`) | `NixOS/nixpkgs` |
| `--data-dir` | Data directory path | `./data` |
| `--poll-interval` | Interval between API polls | `1m` |
| `--once` | Fetch once and exit | `false` |
| `--retention-days` | Days to retain events in SQLite | `30` |
| `--vacuum-interval` | Database vacuum interval | `24h` |
| `--s3-export` | Enable automatic S3 export | `false` |
| `--s3-endpoint` | S3 endpoint URL (overrides env) | env: `S3_ENDPOINT` |
| `--s3-bucket` | S3 bucket name (overrides env) | env: `S3_BUCKET` |
| `--export-interval` | S3 export interval | `24h` |

Example:
```bash
export GITHUB_TOKEN=ghp_xxxxxxxxxxxx
./chronixpkgs fetch \
  --repo NixOS/nixpkgs \
  --poll-interval 1m \
  --retention-days 90 \
  --s3-export \
  --s3-bucket chronixpkgs-archive
```

### Archive Command (Manual maintenance)

The archive command provides manual database maintenance and export capabilities.

| Flag | Description | Default |
|------|-------------|---------|
| `--data-dir` | Data directory path | `./data` |
| `--retention-days` | Days to retain before archiving | `30` |
| `--vacuum` | Run database vacuum | `true` |
| `--s3-export` | Export to S3 | `false` |
| `--s3-endpoint` | S3 endpoint URL (overrides env) | env: `S3_ENDPOINT` |
| `--s3-bucket` | S3 bucket name (overrides env) | env: `S3_BUCKET` |
| `--repo` | Repository to archive | `NixOS/nixpkgs` |
| `--start-date` | Start date (YYYY-MM-DD) | - |
| `--end-date` | End date (YYYY-MM-DD) | - |
| `--days-back` | Days back from today to export | `7` |

Examples:
```bash
# Vacuum database
./chronixpkgs archive --vacuum --data-dir /var/lib/chronixpkgs

# Export last 30 days to S3
./chronixpkgs archive \
  --s3-export \
  --days-back 30 \
  --repo NixOS/nixpkgs

# Export specific date range
./chronixpkgs archive \
  --s3-export \
  --start-date 2025-01-01 \
  --end-date 2025-01-31
```

## Storage Configuration

### SQLite Database

- Location: `{data-dir}/events.db`
- Partitioned by date for efficient queries
- Automatic indexes on event type and timestamp
- WAL mode enabled for concurrent access

### S3 Archive Storage

When S3 export is enabled, events are archived as:
- Path format: `s3://{bucket}/{repo}/{year}/{month}/{day}/events.json.gz`
- Compressed JSON Lines format
- One file per day of events

## Rate Limiting

Server rate limiting configuration:
- Default: 100 requests/minute per IP
- Burst allowance: 10 requests
- Headers: `X-RateLimit-Limit`, `X-RateLimit-Remaining`, `X-RateLimit-Reset`

## Security Configuration

### API Server
- Read-only access only
- No authentication required (public data)
- CORS enabled by default
- Rate limiting enabled by default

### GitHub Token
- Required for fetcher component only
- Needs `public_repo` scope
- Store securely (environment variable or secret file)

### S3 Credentials
- Optional, only needed for archival
- Use IAM roles when possible
- Minimum permissions: `s3:PutObject`, `s3:PutObjectAcl`