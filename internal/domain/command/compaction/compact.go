package compaction

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/gitrus/digikeeper-log/internal/domain/command/model"
	"github.com/gitrus/digikeeper-log/internal/domain/core"
)

// Compact drains applied candidates for a partition by rewriting the log.
//
//  1. Read applied/{partition} — if empty, nothing to do.
//  2. Acquire exclusive entry and candidate partition locks.
//  3. Read all entries from logs/{partition}.
//  4. Substitute: replace matching entries; append unmatched applied candidates.
//  5. Atomic rewrite (write-temp → fsync → rename via renameio).
//  6. Delete compacted candidates from applied/.
//  7. Rebuild index for the rewritten partition.
//  8. Append event to candidate audit.
//
// Steps 6–8 are best-effort. If they fail, next compaction sees the same
// applied candidates and re-applies them (idempotent: same full-copy by ID).
func (s *Service) Compact(ctx context.Context, req CompactRequest) error {
	// 1. Read applied candidates.
	applied, err := s.candidates.ListApplied(ctx, req.Partition)
	if err != nil {
		return fmt.Errorf("compact: list applied: %w", err)
	}
	if len(applied) == 0 {
		s.logger.InfoContext(ctx, "no applied candidates, nothing to compact",
			slog.String("partition", req.Partition.String()))
		return nil
	}

	// 2. Acquire exclusive locks on entry and candidate partitions.
	releaseEntry, err := s.logs.ExclusiveLock(ctx, req.Partition)
	if err != nil {
		return fmt.Errorf("compact: lock entry partition %s: %w", req.Partition, err)
	}
	defer releaseEntry()

	releaseCandidate, err := s.candidates.ExclusiveLock(ctx, req.Partition)
	if err != nil {
		return fmt.Errorf("compact: lock candidate partition %s: %w", req.Partition, err)
	}
	defer releaseCandidate()

	// 3. Read all entries.
	entries, err := s.logs.ReadPartition(ctx, req.Partition)
	if err != nil {
		return fmt.Errorf("compact: read partition %s: %w", req.Partition, err)
	}

	// 4. Substitute.
	rewritten, appliedCount := applySubstitutions(entries, applied)

	// 5. Atomic rewrite.
	if err := s.logs.ReplacePartition(ctx, req.Partition, rewritten); err != nil {
		return fmt.Errorf("compact: rewrite partition %s: %w", req.Partition, err)
	}

	// 6. Delete compacted candidates (best-effort).
	ids := make([]string, len(applied))
	for i, c := range applied {
		ids[i] = c.ID
	}
	if err := s.candidates.DeleteApplied(ctx, req.Partition, ids); err != nil {
		s.logger.ErrorContext(ctx, "delete applied failed, will re-apply on next run",
			slog.String("partition", req.Partition.String()),
			slog.Any("error", err))
	}

	// 7. Rebuild index (best-effort).
	if err := s.index.RebuildPartition(ctx, req.Partition, rewritten); err != nil {
		s.logger.ErrorContext(ctx, "index rebuild failed, partition is consistent but index may be stale",
			slog.String("partition", req.Partition.String()),
			slog.Any("error", err))
	}

	// 8. Candidate audit (best-effort).
	event := CandidateAuditEvent{
		Partition:    req.Partition,
		AppliedCount: appliedCount,
		CompletedAt:  time.Now().UTC(),
	}
	if err := s.candidates.AuditAppend(ctx, event); err != nil {
		s.logger.ErrorContext(ctx, "candidate audit append failed",
			slog.String("partition", req.Partition.String()),
			slog.Any("error", err))
	}

	s.logger.InfoContext(ctx, "compaction completed",
		slog.String("partition", req.Partition.String()),
		slog.Int("applied", appliedCount),
		slog.Int("total_entries", len(rewritten)),
	)

	return nil
}

// applySubstitutions replaces entries whose ID matches an applied candidate and
// appends applied candidates with no matching entry to the end of the partition.
func applySubstitutions(
	entries []core.Entry,
	applied []model.Candidate,
) ([]core.Entry, int) {
	replacements := make(map[string]core.Entry, len(applied))
	for _, c := range applied {
		replacements[c.EntryID] = c.Entry
	}

	count := 0
	seen := make(map[string]struct{}, len(applied))
	result := make([]core.Entry, 0, len(entries)+len(applied))
	for _, e := range entries {
		if replacement, ok := replacements[e.ID]; ok {
			result = append(result, replacement)
			seen[e.ID] = struct{}{}
			count++
		} else {
			result = append(result, e)
		}
	}

	for _, c := range applied {
		if _, ok := seen[c.EntryID]; ok {
			continue
		}
		result = append(result, c.Entry)
		seen[c.EntryID] = struct{}{}
		count++
	}

	return result, count
}
