package store

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/gitrus/digikeeper-log/internal/domain/appmetric"
	"github.com/gitrus/digikeeper-log/internal/domain/errs"
	"github.com/gitrus/digikeeper-log/internal/domain/model"
	"github.com/gitrus/digikeeper-log/internal/infrastructure/jsonlstore"
	"github.com/gitrus/digikeeper-log/pkg/flock"
)

type Store struct {
	flock     *flock.Lock
	rawStore  *jsonlstore.JSONLWriter
	metaStore *SQLiteIndex
}

func NewStore(dataPath string) (*Store, error) {
	if err := os.MkdirAll(dataPath, 0o755); err != nil {
		return nil, fmt.Errorf("store: mkdir %s: %w", dataPath, err)
	}

	flock, err := flock.Acquire(filepath.Join(dataPath, "server.lock"))
	if err != nil {
		return nil, err
	}

	jsonLogsDir := filepath.Join(dataPath, "dk_logs")
	if err := os.MkdirAll(jsonLogsDir, 0o755); err != nil {
		_ = flock.Release()
		return nil, fmt.Errorf("store: mkdir %s: %w", jsonLogsDir, err)
	}

	indexDBPath := filepath.Join(dataPath, "index.db")
	metaStore, err := NewSQLiteIndex(indexDBPath)
	if err != nil {
		_ = flock.Release()
		return nil, fmt.Errorf("store: open index: %w", err)
	}

	st := &Store{
		flock:     flock,
		rawStore:  jsonlstore.NewJSONLWriter(jsonLogsDir, "logs"),
		metaStore: metaStore,
	}
	st.recoverCompaction(jsonLogsDir)

	return st, nil
}

// recoverCompaction removes orphaned .compact.tmp files left by a
// previous crash during compaction. The original partition is always
// intact before rename completes, so temp files can be safely deleted.
//
// Layout: dk_logs/{YYYY}/{YYYY-MM-DD}_logs.jsonl.compact.tmp
func (s *Store) recoverCompaction(dir string) {
	matches, _ := filepath.Glob(filepath.Join(dir, "*", "*.compact.tmp"))

	for _, tmp := range matches {
		if err := os.Remove(tmp); err != nil && !os.IsNotExist(err) {
			slog.Warn("failed to remove orphaned compaction temp file",
				slog.String("file", tmp), slog.Any("error", err))
		} else if err == nil {
			slog.Info("removed orphaned compaction temp file", slog.String("file", tmp))
		}
	}
}

func (s *Store) Append(ctx context.Context, entry model.Entry) error {
	relPath := s.rawStore.BuildRelPath(entry.Timestamp)
	guard, err := s.partitionLock(relPath).SharedLock()
	if err != nil {
		return fmt.Errorf("store: partition lock: %w", err)
	}
	defer func() { _ = guard.Release() }()

	key, err := s.rawStore.Append(entry)
	if err != nil {
		return fmt.Errorf("store: write: %w", err)
	}
	if err := s.metaStore.Insert(ctx, IndexRow{
		File:      key,
		Tags:      entry.Tags,
		Timestamp: entry.Timestamp,
	}); err != nil {
		return fmt.Errorf("store: index failed: %w, %w", err, errs.IndexFailed)
	}
	appmetric.RecordsAppended.Add(1)

	return nil
}

// partitionLock returns the flock-based lock for the given partition.
func (s *Store) partitionLock(relPath string) *flock.PartitionLock {
	lockPath := filepath.Join(s.rawStore.Dir(), relPath+".lock")
	return flock.NewPartitionLock(lockPath)
}

// RawStore returns the underlying JSONLWriter, for use by the compactor.
func (s *Store) RawStore() *jsonlstore.JSONLWriter {
	return s.rawStore
}

func (s *Store) Search(ctx context.Context, p model.SearchParams) ([]string, error) {
	fileResults, err := s.metaStore.Search(ctx, p)
	if err != nil {
		return nil, err
	}
	keys := make([]string, len(fileResults))
	for i, fr := range fileResults {
		keys[i] = fr.File
	}
	return keys, nil
}

func (s *Store) Read(ctx context.Context, keys []string) ([]model.Entry, error) {
	var entries []model.Entry
	for _, key := range keys {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("store: read cancelled: %w", err)
		}
		fileEntries, err := s.rawStore.Read(key)
		if err != nil {
			return nil, fmt.Errorf("store: read %s: %w", key, err)
		}
		entries = append(entries, fileEntries...)
	}
	return entries, nil
}

// Close closes both stores and releases the process lock.
func (s *Store) Close() error {
	return errors.Join(s.metaStore.Close(), s.rawStore.Close(), s.flock.Release())
}
