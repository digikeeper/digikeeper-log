package candidate

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/gitrus/digikeeper-log/internal/domain/command/model"
	"github.com/gitrus/digikeeper-log/internal/domain/core"
)

// Submit creates a candidate replacement for an existing log entry.
func (s *Service) Submit(
	ctx context.Context,
	req SubmitRequest,
	requestID string,
) (model.Candidate, error) {
	partition := core.PartitionFromTime(req.OriginalTimestamp)

	releaseCandidate, err := s.candidateLocker.SharedLock(ctx, partition)
	if err != nil {
		return model.Candidate{}, fmt.Errorf("candidate: lock candidate partition %s: %w", partition, err)
	}
	defer releaseCandidate()

	// Verify the original entry exists in the expected partition.
	original, err := s.logStorage.ReadEntry(ctx, req.EntryID, partition)
	if err != nil {
		return model.Candidate{}, fmt.Errorf("candidate: lookup original %s: %w", req.EntryID, err)
	}

	// Build the replacement entry: same ID, updated fields.
	replacement := original
	replacement.Type = req.Type
	replacement.Tags = req.Tags
	replacement.Data = req.Data
	if replacement.Tags == nil {
		replacement.Tags = []string{}
	}
	if replacement.Data == nil {
		replacement.Data = map[string]any{}
	}

	c := model.Candidate{
		ID:                uuid.NewString(),
		EntryID:           req.EntryID,
		Entry:             replacement,
		OriginalTimestamp: req.OriginalTimestamp,
		CreatedAt:         time.Now().UTC(),
	}

	if err := s.storage.AppendCandidate(ctx, c); err != nil {
		return model.Candidate{}, fmt.Errorf("candidate: append: %w", err)
	}

	s.logger.InfoContext(ctx, "candidate submitted",
		slog.String("candidate_id", c.ID),
		slog.String("entry_id", c.EntryID),
		slog.String("request_id", requestID),
	)

	return c, nil
}
