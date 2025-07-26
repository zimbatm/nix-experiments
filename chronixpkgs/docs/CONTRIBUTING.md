# Contributing to Chronixpkgs

## Development Setup

### Prerequisites

- Go 1.21 or later
- Nix (optional, for development shell)
- SQLite 3
- Git

### Getting Started

1. **Clone the repository:**
```bash
git clone https://github.com/zimbatm/nix-experiments.git
cd nix-experiments/chronixpkgs
```

2. **Enter development shell (with Nix):**
```bash
nix develop
```

Or manually install dependencies:
```bash
go mod download
```

3. **Run tests:**
```bash
go test ./...
```

4. **Run locally:**
```bash
# Set required environment variables
export GITHUB_TOKEN="your_github_token"

# Run both server and fetcher
./dev.sh

# Or run components separately:
# Terminal 1 - Run server (read-only)
go run . server --listen :8080

# Terminal 2 - Run fetcher (write operations)
go run . fetch --repo NixOS/nixpkgs --poll-interval 1m
```

## Code Structure

```
chronixpkgs/
├── main.go                    # Main application entry point
├── cmd/
│   └── chronixpkgs/          # Command implementations
│       ├── main.go           # Command dispatcher
│       ├── server.go         # Server command (read-only)
│       ├── fetch.go          # Fetcher command (write)
│       └── archive.go        # Archive command (maintenance)
├── internal/                 # Private application code
│   ├── fetcher/             # GitHub API polling client
│   ├── storage/             # Storage layer (SQLite + S3)
│   ├── server/              # HTTP server and API
│   ├── metrics/             # Prometheus metrics
│   └── logger/              # Structured logging
├── docs/                    # Documentation
└── templates/               # Web UI templates
```

## Development Guidelines

### Code Style

- Follow standard Go conventions
- Use `gofmt` and `goimports`
- Run `golangci-lint` before committing

```bash
# Format code
go fmt ./...

# Run linter
golangci-lint run

# Run with auto-fix
golangci-lint run --fix
```

### Testing

Write tests for all new functionality:

```go
// storage/sqlite_test.go
func TestPartitionCreation(t *testing.T) {
    store := newTestStore(t)
    defer store.Close()
    
    // Test partition is created for current month
    err := store.ensurePartition(time.Now())
    assert.NoError(t, err)
    
    // Verify partition exists
    stats, err := store.GetPartitionStats()
    assert.NoError(t, err)
    assert.Len(t, stats, 1)
}
```

Run tests with coverage:
```bash
go test -v -race -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Commit Messages

Follow conventional commits:

```
feat: add new REST endpoint for flexible queries
fix: correct event deduplication in batch insert
docs: add deployment guide
perf: optimize partition queries with better indexes
refactor: extract webhook validation to middleware
test: add integration tests for SSE endpoint
```

## Making Changes

### Adding a New Feature

1. **Create an issue** describing the feature
2. **Branch** from `main`: `git checkout -b feat/your-feature`
3. **Implement** with tests
4. **Document** API changes
5. **Submit PR** with description

### Example: Adding a New API Endpoint

```go
// 1. Add handler in internal/server/api.go
func (s *Server) handleNewEndpoint(w http.ResponseWriter, r *http.Request) {
    // Implementation
}

// 2. Register route
func NewServer(store storage.Store, webhookHandler *webhook.Handler) *Server {
    // ...
    s.mux.HandleFunc("/new-endpoint", s.handleNewEndpoint)
    // ...
}

// 3. Add tests in internal/server/api_test.go
func TestNewEndpoint(t *testing.T) {
    // Test implementation
}

// 4. Document in docs/API.md
```

### Performance Improvements

When optimizing performance:

1. **Benchmark** before and after
2. **Profile** to identify bottlenecks
3. **Document** the improvement

```go
// benchmark_test.go
func BenchmarkEventInsert(b *testing.B) {
    store := newTestStore(b)
    defer store.Close()
    
    events := generateTestEvents(1000)
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        err := store.SaveEvents(events)
        require.NoError(b, err)
    }
}
```

Run benchmarks:
```bash
go test -bench=. -benchmem ./...
```

## Pull Request Process

1. **Update documentation** for any API changes
2. **Add tests** for new functionality
3. **Ensure CI passes** (tests, linting, building)
4. **Request review** from maintainers
5. **Address feedback** promptly

### PR Checklist

- [ ] Tests added/updated
- [ ] Documentation updated
- [ ] No linting errors
- [ ] Benchmarks run (for performance changes)
- [ ] Breaking changes documented
- [ ] Example config updated (if needed)

## Testing the Polling System

The system uses polling instead of webhooks. To test:

```bash
# Run with short polling interval for testing
go run . fetch --repo NixOS/nixpkgs --poll-interval 30s --once

# Or use the development script which includes both server and fetcher
./dev.sh
```

To test the archive command:
```bash
# Test vacuum
go run . archive --vacuum --data-dir ./data

# Test S3 export
go run . archive --s3-export --days-back 1
```

## Debugging

### Enable Debug Logging

```toml
[server]
log_level = "debug"
log_format = "json"
```

### Use Delve Debugger

```bash
# Debug the main binary
dlv debug ./cmd/chronixpkgs

# Set breakpoint
(dlv) break main.main
(dlv) continue
```

### SQL Query Debugging

```go
// Enable query logging
db.LogMode(true)

// Or use query explain
EXPLAIN QUERY PLAN
SELECT * FROM events_2024_01 
WHERE repo = ? AND created_at > ?;
```

## Release Process

1. **Update version** in relevant files
2. **Update CHANGELOG.md**
3. **Tag release**: `git tag v1.2.3`
4. **Push tag**: `git push origin v1.2.3`
5. **CI builds and publishes**

## Getting Help

- **Issues**: https://github.com/zimbatm/nix-experiments/issues
- **Discussions**: https://github.com/zimbatm/nix-experiments/discussions
- **Matrix**: #nixos-dev:matrix.org

## Code of Conduct

- Be respectful and constructive
- Help others learn
- Focus on what is best for the community
- Show empathy towards others

Thank you for contributing to Chronixpkgs!