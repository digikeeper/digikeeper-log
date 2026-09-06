package candidate

import (
	"context"
	"log/slog"

	"github.com/digikeeper/digikeeper-journal/internal/domain/core"
	"github.com/digikeeper/digikeeper-journal/internal/domain/query/model"
)

// Storage reads candidates from the pending area.
type Storage interface {
	// ListPending returns all unresolved candidates for a partition.
	ListPending(ctx context.Context, partition core.Partition) ([]model.Candidate, error)
}

// Service handles candidate queries.
type Service struct {
	storage Storage
	logger  *slog.Logger
}

func NewService(s Storage, logger *slog.Logger) *Service {
	return &Service{storage: s, logger: logger}
}
