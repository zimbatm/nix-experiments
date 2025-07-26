# Chronixpkgs Deployment Guide

> **Note**: This project is currently focused on development. Production deployment will be addressed in a future update.

## Development Setup

For development, please refer to [DEVELOPMENT.md](./DEVELOPMENT.md).

## Future Production Deployment

The following sections outline the planned production deployment approach:

### Prerequisites

- NixOS or system with Nix installed
- GitHub personal access token (for API polling)
- Domain with SSL certificate
- (Optional) S3-compatible storage for archives

### Using NixOS Module

```nix
# flake.nix
{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    chronixpkgs.url = "github:zimbatm/nix-experiments?dir=chronixpkgs";
  };

  outputs = { self, nixpkgs, chronixpkgs }: {
    nixosConfigurations.events-server = nixpkgs.lib.nixosSystem {
      system = "x86_64-linux";
      modules = [
        chronixpkgs.nixosModules.default
        ({ pkgs, ... }: {
          services.chronixpkgs = {
            enable = true;
            githubToken = "ghp_xxxxxxxxxxxx"; # Or use file
            repo = "NixOS/nixpkgs";
            
            # Server configuration (read-only)
            server = {
              listen = ":8080";
              enableCORS = true;
              corsOrigins = [ "*" ];
            };
            
            # Fetcher configuration (write operations)
            fetcher = {
              dataDir = "/var/lib/chronixpkgs";
              retentionDays = 90;
              pollInterval = "1m";
              vacuumInterval = "24h";
              # Optional S3 export
              s3Export = false;
              # s3Endpoint = "https://s3.example.com";
              # s3Bucket = "chronixpkgs-archive";
            };
          };

          # Reverse proxy with nginx
          services.nginx = {
            enable = true;
            virtualHosts."events.nixos.org" = {
              enableACME = true;
              forceSSL = true;
              locations."/" = {
                proxyPass = "http://127.0.0.1:8080";
                proxyWebsockets = true;
                extraConfig = ''
                  proxy_buffering off;
                  proxy_set_header X-Real-IP $remote_addr;
                  proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
                  proxy_set_header X-Forwarded-Proto $scheme;
                '';
              };
            };
          };
        })
      ];
    };
  };
}
```

### Manual Deployment

1. **Build the binary:**
```bash
nix build .#chronixpkgs
# or
go build -o chronixpkgs .
```

2. **Set up environment:**
```bash
# Create secrets
echo "ghp_xxxxxxxxxxxx" > /run/secrets/github-token
echo "your-webhook-secret" > /run/secrets/webhook-secret
chmod 600 /run/secrets/*
```

3. **Create systemd service:**
```ini
[Unit]
Description=Chronixpkgs - Nixpkgs Event Chronicle
After=network.target

[Service]
Type=simple
# Run both server and fetcher
ExecStartPre=/usr/local/bin/chronixpkgs fetch \
  --data-dir /var/lib/chronixpkgs \
  --retention-days 90 \
  --poll-interval 1m \
  --vacuum-interval 24h &
ExecStart=/usr/local/bin/chronixpkgs server \
  --listen :8080 \
  --data-dir /var/lib/chronixpkgs \
  --enable-cors
Restart=always
RestartSec=10s
User=chronixpkgs
Group=chronixpkgs

# Security
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/chronixpkgs

# Environment - secrets via env vars
EnvironmentFile=/etc/chronixpkgs/env
# Or directly:
# Environment="GITHUB_TOKEN=ghp_xxxxxxxxxxxx"
# Environment="S3_ACCESS_KEY_ID=your-access-key"
# Environment="S3_SECRET_ACCESS_KEY=your-secret-key"

[Install]
WantedBy=multi-user.target
```

## Production Configuration

### 1. Create Secrets

```bash
# Add GitHub token (required for API access)
echo "ghp_xxxxxxxxxxxx" > /run/secrets/github-token
chmod 600 /run/secrets/github-token

# Optional: S3 credentials for archival
echo "your-access-key" > /run/secrets/s3-access-key
echo "your-secret-key" > /run/secrets/s3-secret-key
chmod 600 /run/secrets/s3-*
```

### 2. Configure Services

The system now uses a polling-only approach:
- No webhook configuration needed
- Fetcher polls GitHub API at configured intervals
- Server provides read-only access to stored events

### 3. Configure Reverse Proxy

#### Nginx
```nginx
server {
    listen 443 ssl http2;
    server_name events.nixos.org;

    ssl_certificate /etc/letsencrypt/live/events.nixos.org/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/events.nixos.org/privkey.pem;

    # Increase timeouts for SSE
    proxy_read_timeout 24h;
    proxy_connect_timeout 1h;
    proxy_send_timeout 1h;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        
        # Headers for SSE
        proxy_set_header Connection '';
        proxy_set_header Cache-Control 'no-cache';
        proxy_set_header X-Accel-Buffering 'no';
        
        # Standard headers
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

#### Caddy
```
events.nixos.org {
    reverse_proxy localhost:8080 {
        flush_interval -1
        header_up X-Real-IP {remote_host}
    }
}
```

## Monitoring

### Prometheus Metrics

Available at `/metrics` endpoint:

```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'chronixpkgs'
    static_configs:
      - targets: ['localhost:8080']
```

Key metrics:
- `github_events_received_total` - Total events received
- `github_events_processed_total` - Successfully processed events
- `github_events_errors_total` - Processing errors
- `http_requests_total` - API request counts
- `http_request_duration_seconds` - Request latencies

### Health Checks

```yaml
# kubernetes
livenessProbe:
  httpGet:
    path: /health
    port: 8080
  initialDelaySeconds: 10
  periodSeconds: 30

readinessProbe:
  httpGet:
    path: /health
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 10
```

### Logs

```bash
# View logs
journalctl -u chronixpkgs -f

# Log aggregation
{
  "level": "info",
  "time": "2024-01-15T10:30:00Z",
  "msg": "Fetched 42 new events",
  "repository": "nixpkgs"
}
```

## Backup & Archive

### Automated Archival

The fetcher automatically handles archival:
- Vacuums old partitions based on retention period
- Exports to S3 if configured (via --s3-export flag)
- Runs at intervals specified by --vacuum-interval

### Manual Archival

```bash
# Run manual vacuum
./chronixpkgs archive --vacuum --data-dir /var/lib/chronixpkgs

# Export specific date range to S3
./chronixpkgs archive \
  --s3-export \
  --start-date 2025-01-01 \
  --end-date 2025-01-31 \
  --repo NixOS/nixpkgs

# Backup current database
sqlite3 /var/lib/chronixpkgs/events.db ".backup /backup/events.db"
```

## Performance Tuning

### Database Optimization

The SQLite database is automatically optimized during vacuum operations. For high-traffic instances, consider:
- Using faster storage (NVMe SSD)
- Increasing system file cache
- Running on a system with more RAM

### System Tuning

```bash
# Increase file descriptors
echo "chronixpkgs soft nofile 65536" >> /etc/security/limits.conf
echo "chronixpkgs hard nofile 65536" >> /etc/security/limits.conf

# Optimize disk I/O
echo "vm.dirty_ratio = 5" >> /etc/sysctl.conf
echo "vm.dirty_background_ratio = 2" >> /etc/sysctl.conf
```

## Troubleshooting

### Common Issues

1. **No new events**
   - Check GitHub token is valid
   - Verify token has necessary permissions
   - Check fetcher logs for API errors
   - Ensure polling interval is reasonable

2. **High memory usage**
   - Reduce cache sizes in config
   - Enable swap for burst capacity
   - Check for memory leaks with pprof

3. **Slow queries**
   - Run manual vacuum with archive command
   - Check partition statistics
   - Verify indexes are being used

4. **S3 export failures**
   - Verify S3 credentials are correct
   - Check S3 endpoint is accessible
   - Ensure bucket exists and is writable

### Debug Mode

```bash
# Run with verbose output
./chronixpkgs --help  # Show all available flags

# Test API endpoints
curl https://events.nixos.org/health
curl https://events.nixos.org/events?limit=10

# Check fetcher status
journalctl -u chronixpkgs-fetcher -f

# Run manual archive
./chronixpkgs archive --vacuum --data-dir /var/lib/chronixpkgs
```

## Security Considerations

1. **Network Security**
   - Always use HTTPS
   - Implement IP allowlisting for webhooks
   - Use firewall rules

2. **Access Control**
   - Run as non-root user
   - Limit file system access
   - Use systemd hardening options

3. **Secrets Management**
   - Never commit secrets to git
   - Use file-based secrets
   - Rotate webhook secrets periodically

## Scaling

### Horizontal Scaling

For very high traffic:

1. **Multiple Ingestion Nodes**
   - Load balance webhooks
   - Use shared storage (NFS/Ceph)
   - Coordinate with distributed locking

2. **Read Replicas**
   - SQLite read-only replicas
   - Cache layer (Redis/Memcached)
   - CDN for static responses

### Vertical Scaling

- Increase SQLite cache size
- Use NVMe storage for database
- Allocate more CPU cores
- Enable huge pages for memory