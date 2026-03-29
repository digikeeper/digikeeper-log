package candidate

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/gitrus/digikeeper-log/internal/domain/model"
)

// Resolve validates caller resolutions and enforces the all-at-once invariant:
// every pending candidate for a given entry must be resolved, with exactly one
// Apply and the rest Dismissed.
//
// Resolve does NOT execute the compaction — it returns the validated
// resolutions (with audit metadata) for the compaction service to execute.
func (s *Service) Resolve(
	ctx context.Context,
	partition string,
	req ResolveRequest,
) ([]model.Resolution, error) {
	// Load pending candidates for validation.
	pending, err := s.pendingReader.ListPending(ctx, partition)
	if err != nil {
		return nil, fmt.Errorf("candidate: list pending: %w", err)
	}

	pendingByID := make(map[string]model.Candidate, len(pending))
	for _, c := range pending {
		pendingByID[c.ID] = c
	}

	// Validate each resolution item.
	for _, item := range req.Resolutions {
		if !item.Action.IsValid() {
			return nil, fmt.Errorf("candidate %s: unknown action %q", item.CandidateID, item.Action)
		}
		if _, ok := pendingByID[item.CandidateID]; !ok {
			return nil, fmt.Errorf("candidate %s is not pending in partition %s", item.CandidateID, partition)
		}
	}

	// Build model.Resolution with audit metadata.
	now := time.Now().UTC()
	resolutions := make([]model.Resolution, len(req.Resolutions))
	for i, item := range req.Resolutions {
		resolutions[i] = model.Resolution{
			CandidateID: item.CandidateID,
			Action:      item.Action,
			ResolvedBy:  req.ResolvedBy,
			ResolvedAt:  now,
			Reason:      item.Reason,
		}
	}

	// Enforce all-at-once: every pending candidate resolved, max one Apply per entry.
	if err := validateAllAtOnce(pending, resolutions); err != nil {
		return nil, err
	}

	s.logger.InfoContext(ctx, "resolutions validated",
		slog.String("partition", partition),
		slog.String("resolved_by", req.ResolvedBy),
		slog.Int("count", len(resolutions)),
	)

	return resolutions, nil
}

// validateAllAtOnce checks that:
//  1. Every pending candidate has a resolution.
//  2. For each entry, at most one candidate is Applied (rest must be Dismissed).
func validateAllAtOnce(pending []model.Candidate, resolutions []model.Resolution) error {
	resolutionByID := make(map[string]model.Resolution, len(resolutions))
	for _, r := range resolutions {
		resolutionByID[r.CandidateID] = r
	}

	// Check all pending are covered.
	for _, c := range pending {
		if _, ok := resolutionByID[c.ID]; !ok {
			return fmt.Errorf(
				"candidate %s (entry %s) has no resolution — all pending candidates must be resolved at once",
				c.ID, c.EntryID,
			)
		}
	}

	// Group by entry, check at most one Apply per entry.
	applyCounts := make(map[string]int)
	for _, c := range pending {
		r := resolutionByID[c.ID]
		if r.Action == model.Apply {
			applyCounts[c.EntryID]++
		}
	}

	for entryID, count := range applyCounts {
		if count > 1 {
			return fmt.Errorf(
				"entry %s has %d candidates marked apply — at most one is allowed",
				entryID, count,
			)
		}
	}

	return nil
}
