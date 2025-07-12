# Disaster Recovery Plan

## Overview

The monthly partitioning + Parquet archival strategy enables full service recovery from:
1. Archived Parquet files in S3
2. GitHub's 90-day event history API

## Recovery Scenarios

### Complete Data Loss Recovery

```bash
# Step 1: Restore from Parquet archives
./chronixpkgs restore \
  --from-s3 s3://chronixpkgs-archive/events/archive/ \
  --data-dir /var/lib/chronixpkgs

# Step 2: Identify the gap
# Last Parquet: events_2024_10.parquet (October 2024)
# Current date: December 15, 2024
# Gap: November 1 - December 15

# Step 3: Backfill from GitHub API
./chronixpkgs backfill \
  --repo NixOS/nixpkgs \
  --from 2024-11-01 \
  --to 2024-12-15
```

### Partial Recovery (Last 3 Months Lost)

```bash
# GitHub provides 90 days of events
# We can recover the last ~3 months directly from the API
./chronixpkgs backfill \
  --repo NixOS/nixpkgs \
  --days 90
```

## Recovery Implementation

```go
// RestoreFromParquet imports Parquet files back to SQLite partitions
func (s *SQLiteStore) RestoreFromParquet(parquetPath string) error {
    // Parse partition name from filename (events_2024_10.parquet)
    partitionName := extractPartitionName(parquetPath)
    
    // Create the partition
    partitionDate := parsePartitionDate(partitionName)
    if err := s.ensurePartition(partitionDate); err != nil {
        return err
    }
    
    // Read Parquet file
    events, err := readParquetFile(parquetPath)
    if err != nil {
        return err
    }
    
    // Batch insert into partition
    return s.SaveEvents(events)
}

// BackfillFromGitHub fetches historical events from GitHub API for nixpkgs
func (f *GitHubFetcher) BackfillFromGitHub(from, to time.Time) error {
    // GitHub returns events in pages of 100, newest first
    // Maximum 300 events per repo (GitHub limitation)
    
    page := 1
    allEvents := []Event{}
    
    for {
        events, hasMore, err := f.fetchEventPage("nixpkgs", page)
        if err != nil {
            return err
        }
        
        // Filter by date range
        for _, event := range events {
            if event.CreatedAt.After(from) && event.CreatedAt.Before(to) {
                allEvents = append(allEvents, event)
            }
        }
        
        if !hasMore || page >= 3 { // Max 300 events
            break
        }
        page++
    }
    
    // Save to appropriate partitions
    return f.store.SaveEvents(allEvents)
}
```

## Recovery Timeline

### Scenario: Complete system failure on Dec 15, 2024

1. **Hour 0-1**: Provision new infrastructure
2. **Hour 1-2**: Restore service with empty database
3. **Hour 2-4**: Download and import Parquet archives
   - Each month takes ~5-10 minutes to import
   - Can parallelize across multiple months
4. **Hour 4-5**: Backfill recent events from GitHub
   - Limited to 300 events per API call
   - May need multiple passes for high-volume periods
5. **Hour 5-6**: Verify data integrity and resume normal operations

Total recovery time: ~6 hours for complete restoration

## Backup Strategy

### Continuous Backups

```toml
[storage.backup]
# Local SQLite backups (for recent data)
enabled = true
interval = "6h"
retention_days = 7

# S3 Parquet archives (for historical data)
[storage.archive]
enabled = true
trigger = "monthly"
retention = "forever"
```

### Backup Locations

```
Primary Storage:
└── /var/lib/chronixpkgs/
    ├── events.db              # Current month + last 90 days
    ├── events_2024_12/        # Current month partition
    └── events_2024_11/        # Last month partition

Archive Storage (S3):
└── s3://chronixpkgs-archive/
    └── events/
        └── archive/
            ├── 2024/
            │   ├── 01/events_2024_01.parquet
            │   ├── 02/events_2024_02.parquet
            │   └── ...
            └── metadata/
                └── last_archived.json
```

## Validation Process

After recovery:

```bash
# 1. Check partition continuity
./chronixpkgs admin check-partitions

# 2. Verify event counts
./chronixpkgs admin verify-counts \
  --compare-with-s3

# 3. Test recent event ingestion
curl http://localhost:8080/health

# 4. Validate event continuity (no gaps)
./chronixpkgs admin check-gaps
```

## Key Advantages

1. **No Data Loss**: Between Parquet archives and GitHub's 90-day history, we can recover 100% of events
2. **Fast Recovery**: Parquet files load quickly into SQLite
3. **Point-in-Time Recovery**: Can restore to any month boundary
4. **Minimal GitHub API Usage**: Only need to fetch the gap period
5. **Cost Effective**: S3 storage is cheap, recovery is rare

## Automation

The recovery process can be fully automated:

```yaml
# kubernetes CronJob for monthly archival
apiVersion: batch/v1
kind: CronJob
metadata:
  name: chronixpkgs-archive
spec:
  schedule: "0 2 1 * *"  # 2 AM on the 1st of each month
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: archiver
            image: chronixpkgs:latest
            command: ["./chronixpkgs", "archive", "--previous-month"]
```

This architecture ensures that the service can be fully restored from backups + GitHub API, making it highly resilient to failures!