package candidate

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/gitrus/digikeeper-log/internal/domain/core"
	"github.com/gitrus/digikeeper-log/internal/domain/query/model"
)

// Collision represents a log entry that has unresolved candidates.
type Collision struct {
	EntryID    string            `json:"entry_id"`
	Partition  core.Partition    `json:"partition"`
	Candidates []model.Candidate `json:"candidates"`
}

// ListCollisions returns log entries that have unresolved candidates in the
// given partition. Each collision includes the entry ID and its pending
// candidates.
func (s *Service) ListCollisions(
	ctx context.Context,
	partition core.Partition,
) ([]Collision, error) {
	pending, err := s.storage.ListPending(ctx, partition)
	if err != nil {
		return nil, fmt.Errorf("candidate query: list pending for %s: %w", partition, err)
	}

	if len(pending) == 0 {
		return nil, nil
	}

	// Group candidates by the entry they target.
	byEntry := make(map[string][]model.Candidate)
	for _, c := range pending {
		byEntry[c.EntryID] = append(byEntry[c.EntryID], c)
	}

	collisions := make([]Collision, 0, len(byEntry))
	for entryID, candidates := range byEntry {
		collisions = append(collisions, Collision{
			EntryID:    entryID,
			Partition:  partition,
			Candidates: candidates,
		})
	}

	s.logger.InfoContext(ctx, "listed collisions",
		slog.String("partition", partition.String()),
		slog.Int("entries_with_candidates", len(collisions)),
		slog.Int("total_candidates", len(pending)),
	)

	return collisions, nil
}
