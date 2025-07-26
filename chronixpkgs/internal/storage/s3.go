package storage

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"path/filepath"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// S3Exporter exports events to S3-compatible storage
type S3Exporter struct {
	client *minio.Client
	bucket string
	prefix string
}

// NewS3Exporter creates a new S3 exporter
func NewS3Exporter(endpoint, bucket, accessKey, secretKey string) (*S3Exporter, error) {
	// Parse the endpoint URL to extract host and determine secure flag
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to parse endpoint URL: %w", err)
	}

	// Extract just the host:port from the URL
	host := u.Host
	if host == "" {
		// If no scheme was provided, assume it's just host:port
		host = endpoint
	}

	// Determine if we should use SSL based on the scheme
	secure := true
	if u.Scheme == "http" {
		secure = false
	}

	// Initialize MinIO client with provided credentials
	client, err := minio.New(host, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: secure,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create MinIO client: %w", err)
	}

	return &S3Exporter{
		client: client,
		bucket: bucket,
		prefix: "events",
	}, nil
}

// ExportDailyEvents exports events for a specific day to S3
func (e *S3Exporter) ExportDailyEvents(ctx context.Context, repo string, date time.Time, events []Event) error {
	// Create path like: events/2025/01/25/github.com/NixOS/nixpkgs.jsonl.gz
	year, month, day := date.Date()
	key := fmt.Sprintf("%s/%04d/%02d/%02d/%s.jsonl.gz", e.prefix, year, month, day, repo)

	// Create gzipped JSONL content
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)

	for _, event := range events {
		data, err := json.Marshal(event)
		if err != nil {
			return fmt.Errorf("failed to marshal event: %w", err)
		}
		if _, err := gw.Write(data); err != nil {
			return fmt.Errorf("failed to write event: %w", err)
		}
		if _, err := gw.Write([]byte("\n")); err != nil {
			return fmt.Errorf("failed to write newline: %w", err)
		}
	}

	if err := gw.Close(); err != nil {
		return fmt.Errorf("failed to close gzip writer: %w", err)
	}

	// Upload to S3
	metadata := map[string]string{
		"repo":        repo,
		"date":        date.Format("2006-01-02"),
		"event-count": fmt.Sprintf("%d", len(events)),
	}

	_, err := e.client.PutObject(ctx, e.bucket, key, bytes.NewReader(buf.Bytes()), int64(buf.Len()),
		minio.PutObjectOptions{
			ContentType:  "application/x-gzip",
			UserMetadata: metadata,
		})

	if err != nil {
		return fmt.Errorf("failed to upload to S3: %w", err)
	}

	return nil
}

// ExportMetadata exports a metadata file with information about available data
func (e *S3Exporter) ExportMetadata(ctx context.Context, metadata map[string]interface{}) error {
	key := fmt.Sprintf("%s/metadata.json", e.prefix)

	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	_, err = e.client.PutObject(ctx, e.bucket, key, bytes.NewReader(data), int64(len(data)),
		minio.PutObjectOptions{
			ContentType: "application/json",
		})

	if err != nil {
		return fmt.Errorf("failed to upload metadata: %w", err)
	}

	return nil
}

// ListExportedDates returns a list of dates for which data has been exported
func (e *S3Exporter) ListExportedDates(ctx context.Context, repo string) ([]time.Time, error) {
	prefix := fmt.Sprintf("%s/", e.prefix)

	// List objects with prefix
	objectsCh := e.client.ListObjects(ctx, e.bucket, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	})

	dates := make(map[string]bool)

	for object := range objectsCh {
		if object.Err != nil {
			return nil, fmt.Errorf("failed to list objects: %w", object.Err)
		}

		// Extract date from key path
		if key := object.Key; len(key) > len(prefix)+10 {
			// Parse year/month/day from path
			dir := filepath.Dir(key)
			base := filepath.Base(dir)
			if base != "" {
				parent := filepath.Dir(dir)
				month := filepath.Base(parent)
				year := filepath.Base(filepath.Dir(parent))
				dateStr := fmt.Sprintf("%s-%s-%s", year, month, base)
				dates[dateStr] = true
			}
		}
	}

	// Convert to time.Time slice
	result := make([]time.Time, 0, len(dates))
	for dateStr := range dates {
		if t, err := time.Parse("2006-01-02", dateStr); err == nil {
			result = append(result, t)
		}
	}

	return result, nil
}

// GetExportedEvents retrieves events for a specific date from S3
func (e *S3Exporter) GetExportedEvents(ctx context.Context, repo string, date time.Time) ([]Event, error) {
	year, month, day := date.Date()
	key := fmt.Sprintf("%s/%04d/%02d/%02d/%s.jsonl.gz", e.prefix, year, month, day, repo)

	object, err := e.client.GetObject(ctx, e.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get object: %w", err)
	}
	defer object.Close()

	// Decompress
	gr, err := gzip.NewReader(object)
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gr.Close()

	// Read and parse events
	var events []Event
	decoder := json.NewDecoder(gr)

	for {
		var event Event
		if err := decoder.Decode(&event); err == io.EOF {
			break
		} else if err != nil {
			return nil, fmt.Errorf("failed to decode event: %w", err)
		}
		events = append(events, event)
	}

	return events, nil
}
