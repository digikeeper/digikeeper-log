package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"encoding/json/v2"

	_ "modernc.org/sqlite" // register "sqlite" driver

	"github.com/gitrus/digikeeper-log/internal/domain/appmetric"
	"github.com/gitrus/digikeeper-log/internal/domain/model"
)

// SQLiteIndex is a file-level metadata index over JSONL files.
// Each row represents one JSONL file with aggregated tags and time range.
type SQLiteIndex struct {
	db *sql.DB
}

// NewSQLiteIndex opens (or creates) a SQLite database at path and runs migrations.
func NewSQLiteIndex(path string) (*SQLiteIndex, error) {
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("sqlite: open: %w", err)
	}
	if err := db.PingContext(context.Background()); err != nil {
		return nil, fmt.Errorf("sqlite: ping: %w", err)
	}

	idx := &SQLiteIndex{db: db}
	if err := idx.migrate(context.Background()); err != nil {
		return nil, fmt.Errorf("sqlite: migrate: %w", err)
	}
	return idx, nil
}

func (idx *SQLiteIndex) migrate(ctx context.Context) error {
	_, err := idx.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS file_index (
			file   TEXT NOT NULL PRIMARY KEY,
			tags   TEXT NOT NULL DEFAULT '[]',
			min_ts TEXT NOT NULL,
			max_ts TEXT NOT NULL
		);
	`)
	return err
}

// IndexRow is the input to [SQLiteIndex.Insert].
// It carries metadata from a single append that gets merged into the file's row.
type IndexRow struct {
	File      string
	Tags      []string
	Timestamp time.Time
}

// Insert upserts file-level metadata: merges tags and expands the time range.
func (idx *SQLiteIndex) Insert(ctx context.Context, row IndexRow) error {
	start := time.Now()
	defer func() { appmetric.RecordIndexLatency(time.Since(start)) }()

	ts := row.Timestamp.UTC().Format(time.RFC3339)

	tx, err := idx.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("sqlite: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Read existing tags for union merge (no-op if row doesn't exist yet)
	var existingTags []string
	var rawTags string
	err = tx.QueryRowContext(ctx,
		`SELECT tags FROM file_index WHERE file = ?`, row.File,
	).Scan(&rawTags)

	switch {
	case errors.Is(err, sql.ErrNoRows):
		// first insert — no existing tags to merge
	case err != nil:
		return fmt.Errorf("sqlite: select tags: %w", err)
	default:
		if err := json.Unmarshal([]byte(rawTags), &existingTags); err != nil {
			return fmt.Errorf("sqlite: unmarshal existing tags: %w", err)
		}
	}

	mergedTags := mergeTags(existingTags, row.Tags)
	tagsJSON, err := json.Marshal(mergedTags)
	if err != nil {
		return fmt.Errorf("sqlite: marshal tags: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO file_index (file, tags, min_ts, max_ts) VALUES (?, ?, ?, ?)
		ON CONFLICT(file) DO UPDATE SET
			tags   = ?,
			min_ts = MIN(file_index.min_ts, excluded.min_ts),
			max_ts = MAX(file_index.max_ts, excluded.max_ts)
	`, row.File, string(tagsJSON), ts, ts, string(tagsJSON))
	if err != nil {
		return fmt.Errorf("sqlite: upsert: %w", err)
	}
	return tx.Commit()
}

type FileResult struct {
	File  string    `json:"file"`
	Tags  []string  `json:"tags"`
	MinTS time.Time `json:"min_ts"`
	MaxTS time.Time `json:"max_ts"`
}

// maxFilesPerSearch caps the number of JSONL files scanned per query.
// Each file is one day of logs; 366 covers a full rolling year.
// Entry-level filtering in the service layer applies the caller's Limit on top.
const maxFilesPerSearch = 366

// Search finds JSONL files matching the given filters.
func (idx *SQLiteIndex) Search(ctx context.Context, p model.SearchParams) ([]FileResult, error) {
	start := time.Now()
	defer func() { appmetric.RecordIndexLatency(time.Since(start)) }()

	where := []string{"1=1"}
	args := []any{}

	if !p.From.IsZero() {
		where = append(where, "max_ts >= ?")
		args = append(args, p.From.UTC().Format(time.RFC3339))
	}
	if !p.To.IsZero() {
		where = append(where, "min_ts <= ?")
		args = append(args, p.To.UTC().Format(time.RFC3339))
	}
	if p.Tag != "" {
		where = append(where, `EXISTS (SELECT 1 FROM json_each(tags) WHERE value = ?)`)
		args = append(args, p.Tag)
	}

	query := fmt.Sprintf(
		`SELECT file, tags, min_ts, max_ts
		   FROM file_index
		  WHERE %s
		  ORDER BY max_ts DESC
		  LIMIT %d`,
		strings.Join(where, " AND "),
		maxFilesPerSearch,
	)

	rows, err := idx.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("sqlite: query: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var results []FileResult
	for rows.Next() {
		var r FileResult
		var minTS, maxTS, tagsRaw string
		if err := rows.Scan(&r.File, &tagsRaw, &minTS, &maxTS); err != nil {
			return nil, fmt.Errorf("sqlite: scan: %w", err)
		}
		var err error
		r.MinTS, err = time.Parse(time.RFC3339, minTS)
		if err != nil {
			return nil, fmt.Errorf("sqlite: parse min_ts %q: %w", minTS, err)
		}
		r.MaxTS, err = time.Parse(time.RFC3339, maxTS)
		if err != nil {
			return nil, fmt.Errorf("sqlite: parse max_ts %q: %w", maxTS, err)
		}
		if err := json.Unmarshal([]byte(tagsRaw), &r.Tags); err != nil {
			return nil, fmt.Errorf("sqlite: unmarshal tags: %w", err)
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// Close closes the underlying database connection pool.
func (idx *SQLiteIndex) Close() error {
	return idx.db.Close()
}

func mergeTags(existing, incoming []string) []string {
	set := make(map[string]struct{}, len(existing)+len(incoming))
	for _, t := range existing {
		set[t] = struct{}{}
	}
	for _, t := range incoming {
		set[t] = struct{}{}
	}
	merged := make([]string, 0, len(set))
	for t := range set {
		merged = append(merged, t)
	}
	return merged
}
