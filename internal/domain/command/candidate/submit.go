package candidate

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/digikeeper/digikeeper-journal/internal/domain/command/model"
	"github.com/digikeeper/digikeeper-journal/internal/domain/core"
)

// Submit creates a candidate replacement for an existing journal record.
func (s *Service) Submit(
	ctx context.Context,
	req SubmitRequest,
	requestID string,
) (model.Candidate, error) {
	partition := core.PartitionFromTime(req.OriginalTimestamp)

	releaseCandidate, err := s.storage.SharedLock(ctx, partition)
	if err != nil {
		return model.Candidate{}, fmt.Errorf("candidate: lock candidate partition %s: %w", partition, err)
	}
	defer releaseCandidate()

	// Verify the original record exists in the expected partition.
	original, err := s.journalStorage.ReadRecord(ctx, req.RecordID, partition)
	if err != nil {
		return model.Candidate{}, fmt.Errorf("candidate: lookup original %s: %w", req.RecordID, err)
	}

	// Build the replacement record: same ID, updated fields.
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
		RecordID:          req.RecordID,
		Record:            replacement,
		OriginalTimestamp: req.OriginalTimestamp,
		CreatedAt:         time.Now().UTC(),
	}

	if err := s.storage.AppendCandidate(ctx, c); err != nil {
		return model.Candidate{}, fmt.Errorf("candidate: append: %w", err)
	}

	s.logger.InfoContext(ctx, "candidate submitted",
		slog.String("candidate_id", c.ID),
		slog.String("record_id", c.RecordID),
		slog.String("request_id", requestID),
	)

	return c, nil
}
