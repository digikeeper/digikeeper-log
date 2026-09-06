package commandstore

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/digikeeper/digikeeper-journal/internal/domain/appmetric"
	"github.com/digikeeper/digikeeper-journal/internal/domain/core"
	"github.com/digikeeper/digikeeper-journal/internal/domain/errs"
	"github.com/digikeeper/digikeeper-journal/internal/infrastructure/index"
	"github.com/digikeeper/digikeeper-journal/internal/infrastructure/jsonlstore"
	"github.com/digikeeper/digikeeper-journal/pkg/flock"
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

	jsonJournalDir := filepath.Join(dataPath, "dk_journal")
	if err := os.MkdirAll(jsonJournalDir, 0o755); err != nil {
		_ = flock.Release()
		return nil, fmt.Errorf("store: mkdir %s: %w", jsonJournalDir, err)
	}

	st := &Store{
		flock:    flock,
		rawStore: jsonlstore.NewJSONLWriter(jsonJournalDir, "journal"),
		idx:      idx,
	}
	st.recoverCompaction(jsonJournalDir)

	return st, nil
}

func (s *Store) Append(ctx context.Context, record core.Record) error {
	relPath := s.rawStore.BuildRelPath(core.PartitionFromTime(record.Timestamp))
	guard, err := s.partitionLock(relPath).SharedLock()
	if err != nil {
		return fmt.Errorf("store: partition lock: %w", err)
	}
	defer func() { _ = guard.Release() }()

	key, err := s.rawStore.Append(record)
	if err != nil {
		return fmt.Errorf("store: write: %w", err)
	}
	if err := s.idx.Insert(ctx, index.Row{
		File:      key,
		Tags:      record.Tags,
		Types:     []string{record.Type},
		Timestamp: record.Timestamp,
	}); err != nil {
		return fmt.Errorf("store: index failed: %w, %w", err, errs.ErrIndexFailed)
	}
	appmetric.RecordsAppended.Add(1)

	return nil
}

// recoverCompaction removes orphaned .compact.tmp
//
// Layout: dk_journal/{YYYY}/{YYYY-MM-DD}_journal.jsonl.compact.tmp
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

// ReadPartition reads all records from the given partition. Satisfies compaction.JournalStorage.
func (s *Store) ReadPartition(_ context.Context, p core.Partition) ([]core.Record, error) {
	relPath := s.rawStore.BuildRelPath(p)
	records, err := s.rawStore.Read(relPath)
	if err != nil {
		return nil, fmt.Errorf("store: read partition %s: %w", p, err)
	}
	return records, nil
}

// ReadRecord scans one partition for the requested record. Satisfies candidate.JournalStorage.
func (s *Store) ReadRecord(ctx context.Context, recordID string, p core.Partition) (core.Record, error) {
	records, err := s.ReadPartition(ctx, p)
	if err != nil {
		return core.Record{}, err
	}
	for _, record := range records {
		if err := ctx.Err(); err != nil {
			return core.Record{}, err
		}
		if record.ID == recordID {
			return record, nil
		}
	}
	return core.Record{}, fmt.Errorf("store: record %s partition %s: %w", recordID, p, errs.ErrRecordNotFound)
}

// ReplacePartition atomically rewrites the partition with records. Satisfies compaction.JournalStorage.
func (s *Store) ReplacePartition(_ context.Context, p core.Partition, records []core.Record) error {
	relPath := s.rawStore.BuildRelPath(p)
	if err := s.rawStore.ReplaceFile(relPath, records); err != nil {
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
