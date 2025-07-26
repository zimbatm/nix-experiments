# Development Guide

## Quick Start with Hivemind

The easiest way to run chronixpkgs in development is using hivemind:

```bash
# Start all development services
./dev.sh

# This will run both chronixpkgs (with gorefresh auto-reload) and the API tester
```

## Process Compose Services

### Available Processes

1. **chronixpkgs** - Combined server and fetcher with auto-reload via gorefresh
   - Server runs on http://localhost:8081 (read-only)
   - Fetcher polls GitHub every minute
   - Automatic vacuum and S3 export if configured
   - Health check at `/health`
   - Web UI at http://localhost:8081
   - API documentation at http://localhost:8081/swagger

2. **api-tester** - Automated API testing
   - Tests all endpoints every 60 seconds
   - Shows event counts and status

3. **archive** - Manual maintenance operations
   - Run with `./chronixpkgs archive` for manual vacuum
   - Use `--s3-export` flag for manual S3 exports
   - Specify date ranges with `--start-date` and `--end-date`

### Hivemind Commands

While hivemind is running:
- `Ctrl+C` - Stop all processes and exit

The output is color-coded by process name for easy reading.

## Configuration Files

- `Procfile` - Process definitions for hivemind
- `.env` - Environment variables (loaded automatically)

## Manual Development

If you prefer to run services manually:

```bash
# Enter development shell
nix develop
# or with direnv:
direnv allow

# Run with gorefresh for auto-reload
gorefresh -p 8081 -- \
  --listen ":8081" \
  --data-dir "./data" \
  --log-level "debug" \
  --poll-interval 1m

# In another terminal, run manual archive
./chronixpkgs archive --vacuum --data-dir "./data"

# Or with S3 export
./chronixpkgs archive --s3-export --data-dir "./data" --days-back 7

# Test API
curl http://localhost:8081/health
curl http://localhost:8081/events?limit=10
```

## Environment Variables

Create a `.env` file with:

```bash
# Required for GitHub API access
GITHUB_TOKEN=ghp_xxxxxxxxxxxx

# Optional: S3-compatible storage (MinIO, R2, etc)
S3_ENDPOINT=https://your-s3-endpoint.com
S3_BUCKET=chronixpkgs-archive
S3_ACCESS_KEY_ID=your-access-key
S3_SECRET_ACCESS_KEY=your-secret-key
```

## Debugging

1. **Enable debug logs**: Use `--log-level debug`
2. **View SQLite data**: `sqlite3 data/events.db`
3. **Web UI**: Visit http://localhost:8081 for visual debugging
4. **Process logs**: All processes output to the terminal with color coding

## Testing

```bash
# Run Go tests
go test ./...

# Test archive command with specific date
./chronixpkgs archive --s3-export --data-dir ./data --days-back 1

# Or with date range
./chronixpkgs archive --s3-export --start-date 2025-01-01 --end-date 2025-01-31

# Use the Web UI API Playground
# Visit http://localhost:8081/api-playground
```

## Common Tasks

### Reset local data
```bash
rm -rf data/
mkdir data/
```

### Force immediate S3 export
```bash
./chronixpkgs archive --s3-export --data-dir ./data --days-back 7
```

### Run manual vacuum
```bash
./chronixpkgs archive --vacuum --data-dir ./data --retention-days 30
```

### Change polling interval
Edit the `chronixpkgs` line in `Procfile` and restart with `./dev.sh`

### View process logs
In process-compose: Select process and press `l`

## Troubleshooting

- **Build fails**: Run `go mod tidy` or `nix develop -c go mod tidy`
- **No events**: Check GitHub token permissions and that fetcher is running
- **Export fails**: Verify S3 credentials in `.env`
- **Port in use**: Default port changed to 8081 to avoid conflicts
- **Templates not found**: Ensure you're running from the project root directory
- **Process won't stop**: Press Ctrl+C twice to force stop all processes
- **Fetcher not polling**: Check logs for API rate limits or authentication errors
- **Archive command fails**: Ensure database is not locked by running processes

## Web UI Features

- **Dashboard**: Real-time event stream and statistics
- **Event Types Explorer**: Browse and filter available event types
- **API Playground**: Test API endpoints interactively
- **Live Stream**: Server-Sent Events viewer

## Development Tools

The Nix development shell includes:
- `go` - Go compiler and tools
- `gopls` - Go language server
- `golangci-lint` - Go linter
- `gorefresh` - Auto-reloading for Go applications
- `hivemind` - Process manager
- `sqlite` - For database inspection
- `jq` - JSON processing
- `curl` - API testing