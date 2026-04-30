package candidate

import (
	"context"
	"fmt"

	"github.com/gitrus/digikeeper-log/internal/domain/command/model"
	"github.com/gitrus/digikeeper-log/internal/domain/core"
)

// ListPending returns the candidate batch currently awaiting resolution.
func (s *Service) ListPending(ctx context.Context, partition core.Partition) ([]model.Candidate, error) {
	release, err := s.storage.SharedLock(ctx, partition)
	if err != nil {
		return nil, fmt.Errorf("candidate: lock candidate partition %s: %w", partition, err)
	}
	defer release()

	pending, err := s.storage.ListPending(ctx, partition)
	if err != nil {
		return nil, fmt.Errorf("candidate: list pending: %w", err)
	}
	return pending, nil
}
