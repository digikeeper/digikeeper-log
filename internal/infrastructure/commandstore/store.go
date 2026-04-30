package commandstore

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/gitrus/digikeeper-log/internal/domain/appmetric"
	"github.com/gitrus/digikeeper-log/internal/domain/core"
	"github.com/gitrus/digikeeper-log/internal/domain/errs"
	"github.com/gitrus/digikeeper-log/internal/infrastructure/index"
	"github.com/gitrus/digikeeper-log/internal/infrastructure/jsonlstore"
	"github.com/gitrus/digikeeper-log/pkg/flock"
)

const lockRetryDelay = 10 * time.Millisecond

type Store struct {
	flock    *flock.Lock
	rawStore *jsonlstore.JSONLWriter
	idx      *index.Store
}

func NewStore(dataPath string, idx *index.Store) (*Store, error) {
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

	st := &Store{
		flock:    flock,
		rawStore: jsonlstore.NewJSONLWriter(jsonLogsDir, "logs"),
		idx:      idx,
	}
	st.recoverCompaction(jsonLogsDir)

	return st, nil
}

func (s *Store) Append(ctx context.Context, entry core.Entry) error {
	relPath := s.rawStore.BuildRelPath(core.PartitionFromTime(entry.Timestamp))
	guard, err := s.partitionLock(relPath).SharedLock()
	if err != nil {
		return fmt.Errorf("store: partition lock: %w", err)
	}
	defer func() { _ = guard.Release() }()

	key, err := s.rawStore.Append(entry)
	if err != nil {
		return fmt.Errorf("store: write: %w", err)
	}
	if err := s.idx.Insert(ctx, index.Row{
		File:      key,
		Tags:      entry.Tags,
		Types:     []string{entry.Type},
		Timestamp: entry.Timestamp,
	}); err != nil {
		return fmt.Errorf("store: index failed: %w, %w", err, errs.ErrIndexFailed)
	}
	appmetric.RecordsAppended.Add(1)

	return nil
}

// recoverCompaction removes orphaned .compact.tmp
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

func (s *Store) partitionLock(relPath string) *flock.RWLock {
	lockPath := filepath.Join(s.rawStore.Dir(), relPath+".lock")
	return flock.NewRWLock(lockPath)
}

// ReadPartition reads all entries from the given partition. Satisfies compaction.LogStorage.
func (s *Store) ReadPartition(_ context.Context, p core.Partition) ([]core.Entry, error) {
	relPath := s.rawStore.BuildRelPath(p)
	entries, err := s.rawStore.Read(relPath)
	if err != nil {
		return nil, fmt.Errorf("store: read partition %s: %w", p, err)
	}
	return entries, nil
}

// ReadEntry scans one partition for the requested entry. Satisfies candidate.LogStorage.
func (s *Store) ReadEntry(ctx context.Context, entryID string, p core.Partition) (core.Entry, error) {
	entries, err := s.ReadPartition(ctx, p)
	if err != nil {
		return core.Entry{}, err
	}
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return core.Entry{}, err
		}
		if entry.ID == entryID {
			return entry, nil
		}
	}
	return core.Entry{}, fmt.Errorf("store: entry %s partition %s: %w", entryID, p, errs.ErrEntryNotFound)
}

// ReplacePartition atomically rewrites the partition with entries. Satisfies compaction.LogStorage.
func (s *Store) ReplacePartition(_ context.Context, p core.Partition, entries []core.Entry) error {
	relPath := s.rawStore.BuildRelPath(p)
	if err := s.rawStore.ReplaceFile(relPath, entries); err != nil {
		return fmt.Errorf("store: replace partition %s: %w", p, err)
	}
	return nil
}

// ExclusiveLock acquires an exclusive flock on the partition. Satisfies compaction.PartitionLocker.
func (s *Store) ExclusiveLock(ctx context.Context, p core.Partition) (func(), error) {
	relPath := s.rawStore.BuildRelPath(p)
	guard, err := lockWithContext(ctx, s.partitionLock(relPath).TryExclusiveLock)
	if err != nil {
		return nil, fmt.Errorf("store: exclusive lock %s: %w", p, err)
	}
	return func() { _ = guard.Release() }, nil
}

// Close closes the raw store and releases the process lock.
// The index store is owned by the caller and must be closed separately.
func (s *Store) Close() error {
	return errors.Join(s.rawStore.Close(), s.flock.Release())
}

func lockWithContext(ctx context.Context, tryLock func() (*flock.Guard, error)) (*flock.Guard, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	ticker := time.NewTicker(lockRetryDelay)
	defer ticker.Stop()

	for {
		guard, err := tryLock()
		switch {
		case err == nil:
			return guard, nil
		case !errors.Is(err, flock.ErrLocked):
			return nil, err
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}
