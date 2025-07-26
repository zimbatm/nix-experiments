# Storage Architecture: SQLite to Parquet Pipeline

## Overview

The system uses a two-tier storage architecture optimized for both real-time access and long-term analytics:

1. **Hot Storage** (SQLite): Recent events for real-time queries
2. **Cold Storage** (Parquet on S3): Historical events for analytics and archival

## Storage Lifecycle

```
GitHub Events → SQLite (Monthly Partitions) → Parquet Files → S3/Object Storage
     ↓              ↓                            ↓                    ↓
 Real-time     0-90 days                   90+ days            Forever
```

## Monthly Partitioning Strategy

### Why Monthly Partitions?

1. **Natural Alignment**: GitHub events naturally cluster by time
2. **Clean Archival Unit**: One month = one Parquet file
3. **Query Optimization**: Most queries are time-based
4. **Maintenance Simplicity**: Easy to drop/archive entire months

### Partition Naming

- SQLite tables: `events_2024_01`, `events_2024_02`, etc.
- Parquet files: `events_2024_01.parquet`, `events_2024_02.parquet`
- S3 structure: `s3://bucket/events/archive/2024/01/events_2024_01.parquet`

## Parquet Benefits

### 1. **Columnar Storage**
- 10-100x compression for JSON payloads
- Efficient analytics queries
- Perfect for time-series data

### 2. **Cost Efficiency**
- Parquet files are 5-10x smaller than raw JSON
- S3 storage costs ~$0.023/GB/month
- 1TB of events → ~100GB Parquet → ~$2.30/month

### 3. **Analytics Ready**
- Direct query with Athena, Spark, DuckDB
- Partitioned by year/month/day columns
- Dictionary encoding for event types and actors

## Migration Flow

### Automatic Monthly Archival

The fetcher command handles automatic archival:

```go
// Periodically (based on vacuum-interval):
1. Archive old partitions based on retention
2. If S3 export is enabled:
   - Export daily events to JSONL
   - Upload to S3 bucket
   - Verify upload
3. Clean up old SQLite partitions
```

For manual archival, use the archive command:
```bash
./chronixpkgs archive --s3-export --start-date 2025-01-01 --end-date 2025-01-31
```

### Query Federation

For queries spanning hot and cold data:

```sql
-- Future implementation could query both:
SELECT * FROM (
  -- Hot data from SQLite
  SELECT * FROM events WHERE created_at > '90 days ago'
  UNION ALL
  -- Cold data from Parquet (via external tool)
  SELECT * FROM parquet_events WHERE created_at <= '90 days ago'
)
```

## Parquet Schema

```
message Event {
  required binary id (UTF8);
  required binary repo (UTF8);
  required binary event_type (UTF8);           # Dictionary encoded
  required binary actor (UTF8);                # Dictionary encoded  
  required int64 created_at (TIMESTAMP_MILLIS);
  required int32 year;                         # Partition column
  required int32 month;                        # Partition column
  required int32 day;                          # Partition column
  required binary payload (UTF8);              # GZIP compressed
}
```

## Access Patterns

### Real-time Access (SQLite)
- Recent events (< 90 days)
- Low latency queries
- Event streaming (SSE)
- Polling-based updates (no webhooks)

### Analytical Access (Parquet)
- Historical analysis
- Bulk exports
- Machine learning datasets
- Compliance/audit queries

## Configuration Example

```toml
[storage]
data_dir = "/var/lib/chronixpkgs"
retention_days = 90  # Keep in SQLite

[storage.archive]
enabled = true
format = "parquet"
compression = "snappy"

[storage.s3]
bucket = "github-events-archive"
prefix = "events/archive"
region = "us-east-1"
# Or use MinIO/R2/etc
endpoint = "https://s3.example.com"
```

## Current Implementation

1. **Polling Architecture**: No webhooks, reliable polling-based fetching
2. **Separation of Concerns**: Fetcher handles writes, server is read-only
3. **Integrated Maintenance**: Vacuum and S3 export built into fetcher
4. **Manual Archive Command**: For on-demand exports and maintenance
5. **JSONL Format**: Simple, efficient format for S3 storage

## Future Enhancements

1. **Query Federation**: Seamlessly query across hot and cold storage
2. **Automatic Rehydration**: Pull archived data back to SQLite for specific queries
3. **Parquet Support**: Convert JSONL to Parquet for better compression
4. **Delta Lake Format**: For ACID transactions on S3
5. **Materialized Views**: Pre-aggregate common queries

## Cost Analysis

For nixpkgs (estimated):
- ~10,000 events/day
- ~300,000 events/month
- ~50MB/month raw JSON

Storage costs:
- SQLite (90 days): ~150MB local disk
- Parquet (forever): ~5MB/month compressed
- S3 cost: ~$0.12/year per month of data

This architecture provides the best of both worlds: real-time access for recent data and cost-effective long-term storage for historical analysis.