package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// File system permissions
const (
	dirPerms  = 0755
	filePerms = 0644
)

// Database configuration
type dbConfig struct {
	CacheSize       int
	BusyTimeout     int // milliseconds
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

var defaultDBConfig = dbConfig{
	CacheSize:       10000,
	BusyTimeout:     5000,
	MaxOpenConns:    25,
	MaxIdleConns:    5,
	ConnMaxLifetime: 5 * time.Minute,
}

// Processing configuration
const (
	eventBatchSize              = 100
)

type SQLiteStore struct {
	db            *sql.DB
	dataDir       string
	preparedStmts map[string]*sql.Stmt
	mu            sync.RWMutex
}

func NewSQLiteStore(dataDir string) (*SQLiteStore, error) {
	if err := os.MkdirAll(dataDir, dirPerms); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	dbPath := filepath.Join(dataDir, "events.db")
	// Enable WAL mode and other optimizations
	db, err := sql.Open("sqlite3", fmt.Sprintf("%s?_journal_mode=WAL&_synchronous=NORMAL&_cache_size=%d&_busy_timeout=%d",
		dbPath, defaultDBConfig.CacheSize, defaultDBConfig.BusyTimeout))
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test the connection with retries
	var lastErr error
	for i := 0; i < 3; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err = db.PingContext(ctx)
		cancel()

		if err == nil {
			break
		}
		lastErr = err
		log.Printf("Database connection attempt %d failed: %v", i+1, err)
		time.Sleep(time.Duration(i+1) * time.Second)
	}

	if lastErr != nil {
		db.Close()
		return nil, fmt.Errorf("failed to connect to database after retries: %w", lastErr)
	}

	// Set connection pool settings
	db.SetMaxOpenConns(defaultDBConfig.MaxOpenConns)
	db.SetMaxIdleConns(defaultDBConfig.MaxIdleConns)
	db.SetConnMaxLifetime(defaultDBConfig.ConnMaxLifetime)

	store := &SQLiteStore{
		db:            db,
		dataDir:       dataDir,
		preparedStmts: make(map[string]*sql.Stmt),
	}

	if err := store.createTables(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create tables: %w", err)
	}

	// Enable query optimizer
	if _, err := db.Exec("PRAGMA optimize"); err != nil {
		return nil, fmt.Errorf("failed to optimize database: %w", err)
	}

	return store, nil
}

func (s *SQLiteStore) createTables() error {
	queries := []string{
		// Create partition tracking table
		`CREATE TABLE IF NOT EXISTS event_partitions (
			partition_name TEXT PRIMARY KEY,
			min_date DATE,
			max_date DATE,
			event_count INTEGER DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// Create fetch state table
		`CREATE TABLE IF NOT EXISTS fetch_state (
			repo TEXT PRIMARY KEY,
			last_event_id TEXT,
			last_fetch_at TIMESTAMP,
			etag TEXT
		)`,

		// Create a view that unions all partitions (will be updated as partitions are added)
		`CREATE VIEW IF NOT EXISTS events AS SELECT * FROM (SELECT NULL as id, NULL as repo, NULL as event_type, NULL as actor, NULL as created_at, NULL as payload, NULL as raw_file, NULL as raw_offset WHERE 0)`,
	}

	for _, query := range queries {
		if _, err := s.db.Exec(query); err != nil {
			return err
		}
	}

	// Ensure current month partition exists with retry
	var partitionErr error
	for i := 0; i < 3; i++ {
		if err := s.ensurePartition(time.Now()); err != nil {
			partitionErr = err
			log.Printf("Failed to create partition (attempt %d): %v", i+1, err)
			time.Sleep(time.Duration(i+1) * time.Second)
			continue
		}
		partitionErr = nil
		break
	}

	if partitionErr != nil {
		return fmt.Errorf("failed to create current partition after retries: %w", partitionErr)
	}

	return nil
}

// ensurePartition creates a partition table for the given month if it doesn't exist
func (s *SQLiteStore) ensurePartition(t time.Time) error {
	partitionName := fmt.Sprintf("events_%d_%02d", t.Year(), t.Month())

	// Check if partition exists
	var exists bool
	err := s.db.QueryRow("SELECT EXISTS(SELECT 1 FROM event_partitions WHERE partition_name = ?)", partitionName).Scan(&exists)
	if err != nil {
		return err
	}

	if exists {
		return nil
	}

	// Create partition table
	createTableQuery := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
		id TEXT PRIMARY KEY,
		repo TEXT NOT NULL,
		event_type TEXT NOT NULL,
		actor TEXT,
		created_at TIMESTAMP,
		payload TEXT,
		raw_file TEXT,
		raw_offset INTEGER
	)`, partitionName)

	if _, err := s.db.Exec(createTableQuery); err != nil {
		return err
	}

	// Create indexes on partition
	indexes := []string{
		fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_%s_repo_created ON %s(repo, created_at)", partitionName, partitionName),
		fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_%s_repo_type ON %s(repo, event_type)", partitionName, partitionName),
		fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_%s_type ON %s(event_type)", partitionName, partitionName),
		fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_%s_repo_id ON %s(repo, id)", partitionName, partitionName),
	}

	for _, idx := range indexes {
		if _, err := s.db.Exec(idx); err != nil {
			return err
		}
	}

	// Record partition metadata
	firstDay := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
	lastDay := firstDay.AddDate(0, 1, -1)

	_, err = s.db.Exec(`INSERT INTO event_partitions (partition_name, min_date, max_date) VALUES (?, ?, ?)`,
		partitionName, firstDay, lastDay)
	if err != nil {
		return err
	}

	// Update the events view to include this partition
	return s.updateEventsView()
}

// updateEventsView recreates the events view to include all partitions
func (s *SQLiteStore) updateEventsView() error {
	// Get all partitions
	rows, err := s.db.Query("SELECT partition_name FROM event_partitions ORDER BY min_date")
	if err != nil {
		return err
	}
	defer rows.Close()

	var partitions []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return err
		}
		partitions = append(partitions, name)
	}

	if err = rows.Err(); err != nil {
		return err
	}

	if len(partitions) == 0 {
		return nil
	}

	// Build UNION ALL query
	var unions []string
	for _, partition := range partitions {
		unions = append(unions, fmt.Sprintf("SELECT * FROM %s", partition))
	}

	// Drop and recreate view
	if _, err := s.db.Exec("DROP VIEW IF EXISTS events"); err != nil {
		return err
	}

	viewQuery := fmt.Sprintf("CREATE VIEW events AS %s", strings.Join(unions, " UNION ALL "))
	_, err = s.db.Exec(viewQuery)
	return err
}

func (s *SQLiteStore) SaveEvents(ctx context.Context, events []Event) error {
	if len(events) == 0 {
		return nil
	}

	// Check context before processing
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context cancelled: %w", err)
	}

	// Group events by month for partitioning
	eventsByMonth := make(map[string][]Event)
	monthsToEnsure := make(map[time.Time]bool)

	for _, event := range events {
		// Determine partition
		monthKey := fmt.Sprintf("%d_%02d", event.CreatedAt.Year(), event.CreatedAt.Month())
		eventsByMonth[monthKey] = append(eventsByMonth[monthKey], event)

		// Track months that need partitions
		monthTime := time.Date(event.CreatedAt.Year(), event.CreatedAt.Month(), 1, 0, 0, 0, 0, time.UTC)
		monthsToEnsure[monthTime] = true
	}

	// Ensure all needed partitions exist
	for month := range monthsToEnsure {
		if err := s.ensurePartition(month); err != nil {
			return fmt.Errorf("failed to ensure partition: %w", err)
		}
	}

	// Begin transaction with context
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Process each month's events
	for monthKey, monthEvents := range eventsByMonth {
		// Group by date for file storage
		eventsByDate := make(map[string][]Event)
		for _, event := range monthEvents {
			date := event.CreatedAt.Format("2006/01/02")
			eventsByDate[date] = append(eventsByDate[date], event)
		}

		// Determine partition table name
		partitionName := fmt.Sprintf("events_%s", monthKey)

		// Use batch insert for better performance
		insertQuery := fmt.Sprintf(`INSERT OR IGNORE INTO %s 
			(id, repo, event_type, actor, created_at, payload, raw_file, raw_offset) 
			VALUES `, partitionName)

		// Save to JSONL files and database
		for date, dateEvents := range eventsByDate {
			rawFile := filepath.Join(s.dataDir, "raw", date,
				fmt.Sprintf("%s.jsonl", strings.ReplaceAll(dateEvents[0].Repo, "/", "_")))

			if err := s.appendToJSONL(ctx, rawFile, dateEvents); err != nil {
				return fmt.Errorf("failed to write to JSONL: %w", err)
			}

			// Process events in batches
			for i := 0; i < len(dateEvents); i += eventBatchSize {
				// Check context cancellation
				if err := ctx.Err(); err != nil {
					return err
				}

				end := i + eventBatchSize
				if end > len(dateEvents) {
					end = len(dateEvents)
				}
				batch := dateEvents[i:end]

				// Build batch insert query
				values := make([]string, 0, len(batch))
				args := make([]interface{}, 0, len(batch)*8)

				for _, event := range batch {
					values = append(values, "(?, ?, ?, ?, ?, ?, ?, ?)")
					payloadJSON, _ := json.Marshal(event.Payload)
					args = append(args,
						event.ID,
						event.Repo,
						event.Type,
						event.Actor,
						event.CreatedAt,
						string(payloadJSON),
						rawFile,
						0, // offset tracking not implemented for JSONL files
					)
				}

				batchQuery := insertQuery + strings.Join(values, ", ")
				if _, err := tx.ExecContext(ctx, batchQuery, args...); err != nil {
					// Check if it's a context cancellation
					if err == context.Canceled || err == context.DeadlineExceeded {
						return fmt.Errorf("context cancelled during batch insert: %w", err)
					}
					return fmt.Errorf("failed to insert batch: %w", err)
				}
			}
		}

		// Update partition event count
		if _, err := tx.ExecContext(ctx,
			"UPDATE event_partitions SET event_count = event_count + ? WHERE partition_name = ?",
			len(monthEvents), partitionName,
		); err != nil {
			return fmt.Errorf("failed to update partition count: %w", err)
		}
	}

	return tx.Commit()
}

func (s *SQLiteStore) appendToJSONL(ctx context.Context, filename string, events []Event) error {
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, dirPerms); err != nil {
		return err
	}

	file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, filePerms)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	for _, event := range events {
		// Check context cancellation
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := encoder.Encode(event); err != nil {
			return err
		}
	}

	return nil
}



func (s *SQLiteStore) GetFetchState(ctx context.Context, repo string) (*FetchState, error) {
	var state FetchState
	err := s.db.QueryRowContext(ctx,
		"SELECT repo, last_event_id, last_fetch_at, etag FROM fetch_state WHERE repo = ?",
		repo,
	).Scan(&state.Repo, &state.LastEventID, &state.LastFetchAt, &state.ETag)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &state, nil
}

func (s *SQLiteStore) UpdateFetchState(ctx context.Context, state *FetchState) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT OR REPLACE INTO fetch_state (repo, last_event_id, last_fetch_at, etag)
		VALUES (?, ?, ?, ?)`,
		state.Repo, state.LastEventID, state.LastFetchAt, state.ETag,
	)
	return err
}

func (s *SQLiteStore) GetEventsFiltered(ctx context.Context, filter EventFilter) ([]Event, error) {
	query := `SELECT id, repo, event_type, actor, created_at, payload 
		FROM events 
		WHERE repo = ?`

	args := []interface{}{filter.Repo}

	// Add time-based filter
	if !filter.Since.IsZero() {
		query += " AND created_at > ?"
		args = append(args, filter.Since)
	}

	// Add ID-based filter (for pagination/resumption)
	if filter.SinceID != "" {
		// First get the timestamp of the since ID
		var sinceTime time.Time
		err := s.db.QueryRowContext(ctx,
			"SELECT created_at FROM events WHERE id = ?", filter.SinceID).Scan(&sinceTime)
		if err == nil {
			query += " AND (created_at > ? OR (created_at = ? AND id > ?))"
			args = append(args, sinceTime, sinceTime, filter.SinceID)
		}
	}

	// Add event type filter if specified
	if len(filter.EventTypes) > 0 {
		placeholders := make([]string, len(filter.EventTypes))
		for i := range placeholders {
			placeholders[i] = "?"
		}
		query += fmt.Sprintf(" AND event_type IN (%s)", strings.Join(placeholders, ","))
		for _, eventType := range filter.EventTypes {
			args = append(args, eventType)
		}
	}

	// Add actor filter
	if filter.Actor != "" {
		query += " AND actor = ?"
		args = append(args, filter.Actor)
	}

	// Add PR number filter
	if filter.PRNumber > 0 {
		query += ` AND (event_type = 'PullRequestEvent' OR event_type = 'PullRequestReviewEvent' 
			OR event_type = 'PullRequestReviewCommentEvent') 
			AND json_extract(payload, '$.pull_request.number') = ?`
		args = append(args, filter.PRNumber)
	}

	// Add issue number filter
	if filter.IssueNumber > 0 {
		query += ` AND (event_type = 'IssuesEvent' OR event_type = 'IssueCommentEvent') 
			AND json_extract(payload, '$.issue.number') = ?`
		args = append(args, filter.IssueNumber)
	}

	query += " ORDER BY created_at DESC, id DESC"

	// Add limit
	if filter.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, filter.Limit)
	}

	// Add offset for pagination
	if filter.Offset > 0 {
		query += " OFFSET ?"
		args = append(args, filter.Offset)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		var event Event
		var payloadStr string
		err := rows.Scan(&event.ID, &event.Repo, &event.Type,
			&event.Actor, &event.CreatedAt, &payloadStr)
		if err != nil {
			return nil, err
		}
		event.Payload = json.RawMessage(payloadStr)
		events = append(events, event)
	}

	return events, rows.Err()
}

func (s *SQLiteStore) ListEventTypes(ctx context.Context, repo string) ([]string, error) {
	query := `SELECT DISTINCT event_type FROM events WHERE repo = ? ORDER BY event_type`

	rows, err := s.db.QueryContext(ctx, query, repo)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var eventTypes []string
	for rows.Next() {
		var eventType string
		if err := rows.Scan(&eventType); err != nil {
			return nil, err
		}
		eventTypes = append(eventTypes, eventType)
	}

	return eventTypes, rows.Err()
}

func (s *SQLiteStore) GetEventCount(ctx context.Context, repo string) (int64, error) {
	query := `SELECT COUNT(*) FROM events WHERE repo = ?`

	var count int64
	err := s.db.QueryRowContext(ctx, query, repo).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}


// ArchiveOldPartitions archives partitions older than the retention period
func (s *SQLiteStore) ArchiveOldPartitions(ctx context.Context, retentionDays int) error {
	return s.ArchiveOldPartitionsWithConfig(ctx, retentionDays)
}

// ArchiveOldPartitionsWithConfig archives partitions
func (s *SQLiteStore) ArchiveOldPartitionsWithConfig(ctx context.Context, retentionDays int) error {
	cutoffDate := time.Now().AddDate(0, 0, -retentionDays)

	// Find partitions to archive
	rows, err := s.db.QueryContext(ctx,
		"SELECT partition_name, min_date FROM event_partitions WHERE max_date < ?",
		cutoffDate,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	type partitionInfo struct {
		name    string
		minDate time.Time
	}

	var partitionsToArchive []partitionInfo
	for rows.Next() {
		var pi partitionInfo
		if err := rows.Scan(&pi.name, &pi.minDate); err != nil {
			return err
		}
		partitionsToArchive = append(partitionsToArchive, pi)
	}

	if err = rows.Err(); err != nil {
		return err
	}

	if len(partitionsToArchive) == 0 {
		return nil
	}

	// Archive each partition
	for _, pi := range partitionsToArchive {
		log.Printf("Archiving partition %s (from %s)", pi.name, pi.minDate.Format("2006-01"))

		// Drop the partition table
		if _, err := s.db.ExecContext(ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", pi.name)); err != nil {
			return fmt.Errorf("failed to drop partition %s: %w", pi.name, err)
		}

		// Remove from partition tracking
		if _, err := s.db.ExecContext(ctx, "DELETE FROM event_partitions WHERE partition_name = ?", pi.name); err != nil {
			return fmt.Errorf("failed to remove partition metadata: %w", err)
		}

		log.Printf("Successfully archived partition %s", pi.name)
	}

	// Update the events view
	return s.updateEventsView()
}

// GetPartitionStats returns statistics about partitions
func (s *SQLiteStore) GetPartitionStats() ([]map[string]interface{}, error) {
	rows, err := s.db.Query(`
		SELECT partition_name, min_date, max_date, event_count, created_at 
		FROM event_partitions 
		ORDER BY min_date DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []map[string]interface{}
	for rows.Next() {
		var name string
		var minDate, maxDate, createdAt time.Time
		var count int

		if err := rows.Scan(&name, &minDate, &maxDate, &count, &createdAt); err != nil {
			return nil, err
		}

		stats = append(stats, map[string]interface{}{
			"partition_name": name,
			"min_date":       minDate,
			"max_date":       maxDate,
			"event_count":    count,
			"created_at":     createdAt,
		})
	}

	return stats, rows.Err()
}

// Optimize runs SQLite optimization
func (s *SQLiteStore) Optimize(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, "PRAGMA optimize")
	return err
}

func (s *SQLiteStore) Close() error {
	// Close prepared statements
	s.mu.Lock()
	for _, stmt := range s.preparedStmts {
		stmt.Close()
	}
	s.mu.Unlock()

	return s.db.Close()
}
