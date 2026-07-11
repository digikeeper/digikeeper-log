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
// applied candidates; the revision CAS in applyCandidate detects them as
// already applied, skips them (not counted in appliedCount/audit), and
// deletes them. The end state is idempotent.
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
	rewritten, appliedCount := s.applySubstitutions(entries, applied)

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
func (s *Service) applySubstitutions(
	entries []core.Entry,
	applied []model.Candidate,
) ([]core.Entry, int) {
	candidatesByEntryID := make(map[string]model.Candidate, len(applied))
	for _, candidate := range applied {
		candidatesByEntryID[candidate.EntryID] = candidate
	}

	appliedCount := 0
	seen := make(map[string]struct{}, len(applied))
	result := make([]core.Entry, 0, len(entries)+len(applied))
	for _, entry := range entries {
		candidate, ok := candidatesByEntryID[entry.ID]
		if !ok {
			result = append(result, entry)
			continue
		}

		replacement, applied := s.applyCandidate(entry, candidate)
		result = append(result, replacement)
		seen[entry.ID] = struct{}{}
		if applied {
			appliedCount++
		}
	}

	for _, candidate := range applied {
		if _, ok := seen[candidate.EntryID]; ok {
			continue
		}
		replacement := candidate.Entry
		replacement.Meta.Revision = nextRevision(replacement.Meta.Revision)
		result = append(result, replacement)
		seen[candidate.EntryID] = struct{}{}
		appliedCount++
	}

	return result, appliedCount
}

func (s *Service) applyCandidate(current core.Entry, candidate model.Candidate) (core.Entry, bool) {
	replacement := candidate.Entry
	currentRevision := normalizedRevision(current.Meta.Revision)
	candidateRevision := normalizedRevision(replacement.Meta.Revision)

	// A candidate is a compare-and-swap operation over the revision it copied
	// at submission. It is stale after a successful rewrite whose applied-file
	// cleanup failed, and it is invalid if it was somehow created from a future
	// revision. In both cases, preserve the current entry.
	if candidateRevision != currentRevision {
		s.logger.Warn("skipping candidate with mismatched entry revision",
			slog.String("candidate_id", candidate.ID),
			slog.String("entry_id", current.ID),
			slog.Int("candidate_revision", candidateRevision),
			slog.Int("current_revision", currentRevision),
		)
		return current, false
	}

	replacement.Meta.Revision = nextRevision(current.Meta.Revision)
	return replacement, true
}

func nextRevision(revision int) int {
	return normalizedRevision(revision) + 1
}

func normalizedRevision(revision int) int {
	// Entries written before revisions existed omit "r". Treat them as the
	// initial logical revision so their first replacement becomes revision 2.
	if revision < 1 {
		return 1
	}
	return revision
}
