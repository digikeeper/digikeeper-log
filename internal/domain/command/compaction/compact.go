package compaction

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/gitrus/digikeeper-log/internal/domain/model"
)

// Compact executes a compaction for a partition using pre-validated resolutions.
//
// The resolutions must be validated by the candidate service before calling
// this method. Compact handles the physical rewrite:
//
//  1. Fetch pending candidates and pair with resolutions.
//  2. Acquire exclusive partition lock (blocks appenders via flock).
//  3. Read all entries from the partition.
//  4. Apply substitutions (replace entries whose ID matches an applied candidate).
//  5. Atomically rewrite the partition (write-temp → fsync → rename via renameio).
//  6. Rebuild the index for the rewritten partition.
//  7. Move candidates: pending → applied/dismissed.
//  8. Record the event in the journal.
func (s *Service) Compact(ctx context.Context, req CompactRequest) error {
	// 1. Fetch pending candidates and pair with resolutions.
	pending, err := s.candidates.ListPending(ctx, req.Partition)
	if err != nil {
		return fmt.Errorf("compact: list pending: %w", err)
	}
	if len(pending) == 0 {
		s.logger.InfoContext(ctx, "no pending candidates, nothing to compact",
			slog.String("partition", req.Partition))
		return nil
	}

	resolved, err := pairResolutions(pending, req.Resolutions)
	if err != nil {
		return fmt.Errorf("compact: %w", err)
	}

	// 2. Acquire exclusive partition lock.
	release, err := s.locker.ExclusiveLock(ctx, req.Partition)
	if err != nil {
		return fmt.Errorf("compact: lock partition %s: %w", req.Partition, err)
	}
	defer release()

	// 3. Read all entries.
	entries, err := s.logs.ReadPartition(ctx, req.Partition)
	if err != nil {
		return fmt.Errorf("compact: read partition %s: %w", req.Partition, err)
	}

	// 4. Apply substitutions.
	rewritten, appliedCount := applySubstitutions(entries, resolved)

	// 5. Atomically rewrite the partition.
	if err := s.logs.ReplacePartition(ctx, req.Partition, rewritten); err != nil {
		return fmt.Errorf("compact: rewrite partition %s: %w", req.Partition, err)
	}

	// 6. Rebuild index (best-effort — partition is already consistent).
	if err := s.index.RebuildPartition(ctx, req.Partition, rewritten); err != nil {
		s.logger.ErrorContext(ctx, "index rebuild failed, partition is consistent but index may be stale",
			slog.String("partition", req.Partition),
			slog.Any("error", err))
	}

	// 7. Move candidates (best-effort — partition is already consistent).
	if err := s.candidates.MoveCandidates(ctx, resolved); err != nil {
		s.logger.ErrorContext(ctx, "candidate move failed, partition is consistent but candidates may remain in pending",
			slog.String("partition", req.Partition),
			slog.Any("error", err))
	}

	// 8. Journal.
	dismissedCount := len(resolved) - appliedCount
	event := JournalEvent{
		Partition:      req.Partition,
		ResolvedCount:  len(resolved),
		AppliedCount:   appliedCount,
		DismissedCount: dismissedCount,
		CompletedAt:    time.Now().UTC(),
	}
	if err := s.journal.AppendJournal(ctx, event); err != nil {
		s.logger.ErrorContext(ctx, "journal append failed",
			slog.String("partition", req.Partition),
			slog.Any("error", err))
	}

	s.logger.InfoContext(ctx, "compaction completed",
		slog.String("partition", req.Partition),
		slog.Int("applied", appliedCount),
		slog.Int("dismissed", dismissedCount),
		slog.Int("total_entries", len(rewritten)),
	)

	return nil
}

// pairResolutions matches each resolution to its pending candidate.
func pairResolutions(
	pending []model.Candidate,
	resolutions []model.Resolution,
) ([]ResolvedCandidate, error) {
	byID := make(map[string]model.Resolution, len(resolutions))
	for _, r := range resolutions {
		byID[r.CandidateID] = r
	}

	resolved := make([]ResolvedCandidate, 0, len(pending))
	for _, c := range pending {
		r, ok := byID[c.ID]
		if !ok {
			return nil, fmt.Errorf(
				"candidate %s (entry %s) has no resolution",
				c.ID, c.EntryID,
			)
		}
		resolved = append(resolved, ResolvedCandidate{
			Candidate: c,
			Action:    r.Action,
		})
	}

	return resolved, nil
}

// applySubstitutions replaces entries whose ID matches an applied candidate.
func applySubstitutions(
	entries []model.Entry,
	resolved []ResolvedCandidate,
) ([]model.Entry, int) {
	replacements := make(map[string]model.Entry)
	for _, rc := range resolved {
		if rc.Action == model.Apply {
			replacements[rc.Candidate.EntryID] = rc.Candidate.Entry
		}
	}

	if len(replacements) == 0 {
		return entries, 0
	}

	applied := 0
	result := make([]model.Entry, len(entries))
	for i, e := range entries {
		if replacement, ok := replacements[e.ID]; ok {
			result[i] = replacement
			applied++
		} else {
			result[i] = e
		}
	}

	return result, applied
}
