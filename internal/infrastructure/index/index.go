package index

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
)

// SearchParams filters a [Store.Search] query.
// Tags and Types use OR semantics: a file matches if it contains any of the listed values.
type SearchParams struct {
	Tags  []string
	Types []string
	From  time.Time
	To    time.Time
}

// Store is a file-level metadata index over JSONL files backed by SQLite.
// Each row represents one JSONL file with aggregated tags, types, and time range.
type Store struct {
	db *sql.DB
}

// New opens (or creates) a SQLite database at path and runs migrations.
func New(path string) (*Store, error) {
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("sqlite: open: %w", err)
	}
	if err := db.PingContext(context.Background()); err != nil {
		return nil, fmt.Errorf("sqlite: ping: %w", err)
	}

	s := &Store{db: db}
	if err := s.migrate(context.Background()); err != nil {
		return nil, fmt.Errorf("sqlite: migrate: %w", err)
	}
	return s, nil
}

func (s *Store) migrate(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS file_index (
			file   TEXT NOT NULL PRIMARY KEY,
			tags   TEXT NOT NULL DEFAULT '[]',
			types  TEXT NOT NULL DEFAULT '[]',
			min_ts TEXT NOT NULL,
			max_ts TEXT NOT NULL
		);
	`)
	return err
}

// Row is the input to [Store.Insert].
// It carries metadata from a single append that gets merged into the file's row.
type Row struct {
	File      string
	Tags      []string
	Types     []string
	Timestamp time.Time
}

// Insert upserts file-level metadata: merges tags/types and expands the time range.
func (s *Store) Insert(ctx context.Context, row Row) error {
	start := time.Now()
	defer func() { appmetric.RecordIndexLatency(time.Since(start)) }()

	ts := row.Timestamp.UTC().Format(time.RFC3339)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("sqlite: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var existingTags, existingTypes []string
	var rawTags, rawTypes string
	err = tx.QueryRowContext(ctx,
		`SELECT tags, types FROM file_index WHERE file = ?`, row.File,
	).Scan(&rawTags, &rawTypes)

	switch {
	case errors.Is(err, sql.ErrNoRows):
		// first insert — nothing to merge
	case err != nil:
		return fmt.Errorf("sqlite: select: %w", err)
	default:
		if err := json.Unmarshal([]byte(rawTags), &existingTags); err != nil {
			return fmt.Errorf("sqlite: unmarshal tags: %w", err)
		}
		if err := json.Unmarshal([]byte(rawTypes), &existingTypes); err != nil {
			return fmt.Errorf("sqlite: unmarshal types: %w", err)
		}
	}

	tagsJSON, err := json.Marshal(mergeStrings(existingTags, row.Tags))
	if err != nil {
		return fmt.Errorf("sqlite: marshal tags: %w", err)
	}
	typesJSON, err := json.Marshal(mergeStrings(existingTypes, row.Types))
	if err != nil {
		return fmt.Errorf("sqlite: marshal types: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO file_index (file, tags, types, min_ts, max_ts) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(file) DO UPDATE SET
			tags   = ?,
			types  = ?,
			min_ts = MIN(file_index.min_ts, excluded.min_ts),
			max_ts = MAX(file_index.max_ts, excluded.max_ts)
	`, row.File, string(tagsJSON), string(typesJSON), ts, ts,
		string(tagsJSON), string(typesJSON))
	if err != nil {
		return fmt.Errorf("sqlite: upsert: %w", err)
	}
	return tx.Commit()
}

// Result is a single file returned by [Store.Search].
type Result struct {
	File  string    `json:"file"`
	Tags  []string  `json:"tags"`
	Types []string  `json:"types"`
	MinTS time.Time `json:"min_ts"`
	MaxTS time.Time `json:"max_ts"`
}

// maxFilesPerSearch caps the number of JSONL files scanned per query.
// Each file is one day of logs; 366 covers a full rolling year.
const maxFilesPerSearch = 366

// Search finds JSONL files whose metadata matches the given filters.
// Tags and Types use OR semantics: a file matches if it contains any listed value.
func (s *Store) Search(ctx context.Context, p SearchParams) ([]Result, error) {
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
	if len(p.Tags) > 0 {
		where = append(where, jsonAnyOf("tags", len(p.Tags)))
		for _, t := range p.Tags {
			args = append(args, t)
		}
	}
	if len(p.Types) > 0 {
		where = append(where, jsonAnyOf("types", len(p.Types)))
		for _, t := range p.Types {
			args = append(args, t)
		}
	}

	q := fmt.Sprintf(
		`SELECT file, tags, types, min_ts, max_ts
		   FROM file_index
		  WHERE %s
		  ORDER BY max_ts DESC
		  LIMIT %d`,
		strings.Join(where, " AND "),
		maxFilesPerSearch,
	)

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("sqlite: query: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var results []Result
	for rows.Next() {
		var r Result
		var minTS, maxTS, tagsRaw, typesRaw string
		if err := rows.Scan(&r.File, &tagsRaw, &typesRaw, &minTS, &maxTS); err != nil {
			return nil, fmt.Errorf("sqlite: scan: %w", err)
		}
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
		if err := json.Unmarshal([]byte(typesRaw), &r.Types); err != nil {
			return nil, fmt.Errorf("sqlite: unmarshal types: %w", err)
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// Close closes the underlying database connection pool.
func (s *Store) Close() error {
	return s.db.Close()
}

// jsonAnyOf builds an EXISTS clause that matches if any value in the named
// JSON array column equals one of n placeholders.
func jsonAnyOf(col string, n int) string {
	return fmt.Sprintf(
		`EXISTS (SELECT 1 FROM json_each(%s) WHERE value IN (%s))`,
		col, strings.Repeat("?,", n)[:n*2-1],
	)
}

func mergeStrings(existing, incoming []string) []string {
	set := make(map[string]struct{}, len(existing)+len(incoming))
	for _, v := range existing {
		set[v] = struct{}{}
	}
	for _, v := range incoming {
		set[v] = struct{}{}
	}
	merged := make([]string, 0, len(set))
	for v := range set {
		merged = append(merged, v)
	}
	return merged
}
