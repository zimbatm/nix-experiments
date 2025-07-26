# Configuration Reference

This document details all configuration options for chronixpkgs components.

## Environment Variables

| Variable | Description | Required | Used By |
|----------|-------------|----------|---------|
| `GITHUB_TOKEN` | GitHub personal access token for API requests | Yes | fetcher |
| `S3_ENDPOINT` | S3-compatible endpoint URL | No | fetcher, archive |
| `S3_BUCKET` | S3 bucket name for archives | No | fetcher, archive |
| `S3_ACCESS_KEY_ID` | S3 access key | No | fetcher, archive |
| `S3_SECRET_ACCESS_KEY` | S3 secret key | No | fetcher, archive |

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