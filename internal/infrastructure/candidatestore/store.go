package candidatestore

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gitrus/digikeeper-log/internal/domain/command/compaction"
	commandmodel "github.com/gitrus/digikeeper-log/internal/domain/command/model"
	"github.com/gitrus/digikeeper-log/internal/domain/core"
	"github.com/gitrus/digikeeper-log/internal/domain/errs"
	"github.com/gitrus/digikeeper-log/internal/jsonx"
	"github.com/gitrus/digikeeper-log/pkg/flock"
)

const (
	maxJSONLRecordSizeBytes = 10 * 1024 * 1024
	lockRetryDelay          = 10 * time.Millisecond
)

type Store struct {
	baseDir string
}

func New(dataPath string) (*Store, error) {
	baseDir := filepath.Join(dataPath, "dk_candidates")
	for _, dir := range []string{"pending", "applied", "denied", "candidateaudit", "locks"} {
		if err := os.MkdirAll(filepath.Join(baseDir, dir), 0o755); err != nil {
			return nil, fmt.Errorf("candidate store: mkdir %s: %w", dir, err)
		}
	}
	return &Store{baseDir: baseDir}, nil
}

func (s *Store) AppendCandidate(ctx context.Context, c commandmodel.Candidate) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	path := s.candidatePath("pending", core.PartitionFromTime(c.OriginalTimestamp))
	line, err := jsonx.Marshal(c)
	if err != nil {
		return fmt.Errorf("candidate store: marshal candidate: %w, %w", err, errs.ErrStorageCommon)
	}
	line = append(line, '\n')

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("candidate store: mkdir pending: %w, %w", err, errs.ErrStorageCommon)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("candidate store: open pending: %w, %w", err, errs.ErrStorageCommon)
	}
	defer func() { _ = f.Close() }()

	if _, err := f.Write(line); err != nil {
		return fmt.Errorf("candidate store: append pending: %w, %w", err, errs.ErrStorageCommon)
	}
	if err := f.Sync(); err != nil {
		return fmt.Errorf("candidate store: sync pending: %w, %w", err, errs.ErrStorageCommon)
	}
	return nil
}

func (s *Store) ListPending(ctx context.Context, partition core.Partition) ([]commandmodel.Candidate, error) {
	return readCandidates(ctx, s.candidatePath("pending", partition))
}

func (s *Store) ListApplied(ctx context.Context, partition core.Partition) ([]commandmodel.Candidate, error) {
	return readCandidates(ctx, s.candidatePath("applied", partition))
}

func (s *Store) MoveCandidates(
	ctx context.Context,
	partition core.Partition,
	applied []commandmodel.Candidate,
	denied []commandmodel.Candidate,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	appliedPath := s.candidatePath("applied", partition)
	if nonEmpty, err := fileNonEmpty(appliedPath); err != nil {
		return fmt.Errorf("candidate store: stat applied: %w, %w", err, errs.ErrStorageCommon)
	} else if nonEmpty {
		return fmt.Errorf("candidate store: applied candidates already exist for %s: %w", partition, errs.ErrConflict)
	}

	deniedPath := s.candidatePath("denied", partition)
	existingDenied, err := readCandidates(ctx, deniedPath)
	if err != nil {
		return fmt.Errorf("candidate store: read denied: %w", err)
	}
	denied = append(existingDenied, denied...)

	var tmps []string
	cleanupTmps := true
	defer func() {
		if cleanupTmps {
			for _, tmpPath := range tmps {
				_ = os.Remove(tmpPath)
			}
		}
	}()

	var appliedTmp string
	if len(applied) > 0 {
		appliedTmp, err = writeCandidatesTemp(ctx, appliedPath, applied)
		if err != nil {
			return fmt.Errorf("candidate store: write applied temp: %w", err)
		}
		tmps = append(tmps, appliedTmp)
	}
	var deniedTmp string
	if len(denied) > 0 {
		deniedTmp, err = writeCandidatesTemp(ctx, deniedPath, denied)
		if err != nil {
			return fmt.Errorf("candidate store: write denied temp: %w", err)
		}
		tmps = append(tmps, deniedTmp)
	}

	var renamed []string
	if appliedTmp != "" {
		if err := os.Rename(appliedTmp, appliedPath); err != nil {
			return fmt.Errorf("candidate store: rename applied: %w, %w", err, errs.ErrStorageCommon)
		}
		renamed = append(renamed, appliedPath)
	}
	if deniedTmp != "" {
		if err := os.Rename(deniedTmp, deniedPath); err != nil {
			for _, path := range renamed {
				_ = os.Remove(path)
			}
			return fmt.Errorf("candidate store: rename denied: %w, %w", err, errs.ErrStorageCommon)
		}
		renamed = append(renamed, deniedPath)
	}
	cleanupTmps = false

	pendingPath := s.candidatePath("pending", partition)
	if err := os.Remove(pendingPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("candidate store: remove pending: %w, %w", err, errs.ErrStorageCommon)
	}
	for _, path := range append(renamed, pendingPath) {
		if err := syncDir(filepath.Dir(path)); err != nil {
			return fmt.Errorf("candidate store: sync dir: %w, %w", err, errs.ErrStorageCommon)
		}
	}
	return nil
}

func (s *Store) DeleteApplied(ctx context.Context, partition core.Partition, candidateIDs []string) error {
	if len(candidateIDs) == 0 {
		return nil
	}

	appliedPath := s.candidatePath("applied", partition)
	applied, err := readCandidates(ctx, appliedPath)
	if err != nil {
		return err
	}
	if len(applied) == 0 {
		return nil
	}

	remove := make(map[string]struct{}, len(candidateIDs))
	for _, id := range candidateIDs {
		remove[id] = struct{}{}
	}
	kept := make([]commandmodel.Candidate, 0, len(applied))
	for _, c := range applied {
		if err := ctx.Err(); err != nil {
			return err
		}
		if _, ok := remove[c.ID]; !ok {
			kept = append(kept, c)
		}
	}
	if len(kept) == 0 {
		if err := os.Remove(appliedPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("candidate store: remove applied: %w, %w", err, errs.ErrStorageCommon)
		}
		return syncDir(filepath.Dir(appliedPath))
	}
	return replaceCandidates(ctx, appliedPath, kept)
}

func (s *Store) AuditAppend(ctx context.Context, event compaction.CandidateAuditEvent) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	path := s.candidateAuditPath(event.Partition)
	line, err := jsonx.Marshal(event)
	if err != nil {
		return fmt.Errorf("candidate store: marshal candidate audit: %w, %w", err, errs.ErrStorageCommon)
	}
	line = append(line, '\n')

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("candidate store: mkdir candidate audit: %w, %w", err, errs.ErrStorageCommon)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("candidate store: open candidate audit: %w, %w", err, errs.ErrStorageCommon)
	}
	defer func() { _ = f.Close() }()

	if _, err := f.Write(line); err != nil {
		return fmt.Errorf("candidate store: append candidate audit: %w, %w", err, errs.ErrStorageCommon)
	}
	if err := f.Sync(); err != nil {
		return fmt.Errorf("candidate store: sync candidate audit: %w, %w", err, errs.ErrStorageCommon)
	}
	return nil
}

func (s *Store) SharedLock(ctx context.Context, partition core.Partition) (func(), error) {
	guard, err := lockWithContext(ctx, s.partitionLock(partition).TrySharedLock)
	if err != nil {
		return nil, fmt.Errorf("candidate store: shared lock %s: %w", partition, err)
	}
	return func() { _ = guard.Release() }, nil
}

func (s *Store) ExclusiveLock(ctx context.Context, partition core.Partition) (func(), error) {
	guard, err := lockWithContext(ctx, s.partitionLock(partition).TryExclusiveLock)
	if err != nil {
		return nil, fmt.Errorf("candidate store: exclusive lock %s: %w", partition, err)
	}
	return func() { _ = guard.Release() }, nil
}

func (s *Store) partitionLock(partition core.Partition) *flock.RWLock {
	return flock.NewRWLock(filepath.Join(s.baseDir, "locks", partition.String()+".lock"))
}

func (s *Store) candidatePath(area string, partition core.Partition) string {
	return filepath.Join(
		s.baseDir,
		area,
		fmt.Sprintf("%d", partition.Year()),
		fmt.Sprintf("%s_candidates.jsonl", partition.String()),
	)
}

func (s *Store) candidateAuditPath(partition core.Partition) string {
	return filepath.Join(
		s.baseDir,
		"candidateaudit",
		fmt.Sprintf("%d", partition.Year()),
		fmt.Sprintf("%s_candidateaudit.jsonl", partition.String()),
	)
}

func readCandidates(ctx context.Context, path string) ([]commandmodel.Candidate, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("candidate store: open %s: %w, %w", path, err, errs.ErrStorageCommon)
	}
	defer func() { _ = f.Close() }()

	var candidates []commandmodel.Candidate
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, bufio.MaxScanTokenSize), maxJSONLRecordSizeBytes)
	for sc.Scan() {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var c commandmodel.Candidate
		if err := jsonx.Unmarshal(line, &c); err != nil {
			return nil, fmt.Errorf("candidate store: unmarshal %s: %w, %w", path, err, errs.ErrStorageCommon)
		}
		candidates = append(candidates, c)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("candidate store: scan %s: %w, %w", path, err, errs.ErrStorageCommon)
	}
	return candidates, nil
}

func replaceCandidates(ctx context.Context, path string, candidates []commandmodel.Candidate) error {
	tmpPath, err := writeCandidatesTemp(ctx, path, candidates)
	if err != nil {
		return err
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("candidate store: rename temp: %w, %w", err, errs.ErrStorageCommon)
	}
	cleanup = false
	return syncDir(filepath.Dir(path))
}

func writeCandidatesTemp(ctx context.Context, path string, candidates []commandmodel.Candidate) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("candidate store: mkdir: %w, %w", err, errs.ErrStorageCommon)
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return "", fmt.Errorf("candidate store: create temp: %w, %w", err, errs.ErrStorageCommon)
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()
	for _, c := range candidates {
		if err := ctx.Err(); err != nil {
			_ = tmp.Close()
			return "", err
		}
		line, err := jsonx.Marshal(c)
		if err != nil {
			_ = tmp.Close()
			return "", fmt.Errorf("candidate store: marshal: %w, %w", err, errs.ErrStorageCommon)
		}
		if _, err := tmp.Write(append(line, '\n')); err != nil {
			_ = tmp.Close()
			return "", fmt.Errorf("candidate store: write temp: %w, %w", err, errs.ErrStorageCommon)
		}
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("candidate store: sync temp: %w, %w", err, errs.ErrStorageCommon)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("candidate store: close temp: %w, %w", err, errs.ErrStorageCommon)
	}
	cleanup = false
	return tmpPath, nil
}

func fileNonEmpty(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return info.Size() > 0, nil
}

func syncDir(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer func() { _ = dir.Close() }()
	return dir.Sync()
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
