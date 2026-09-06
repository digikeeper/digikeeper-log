package command

import (
	"context"
	"log/slog"

	"github.com/digikeeper/digikeeper-journal/internal/domain/core"
)

type Storage interface {
	Append(ctx context.Context, record core.Record) error
}

type SourceRepo interface {
	ResolveID(clientName string) int
}

type Service struct {
	storage    Storage
	sourceRepo SourceRepo
	logger     *slog.Logger
}

func NewService(s Storage, sr SourceRepo, logger *slog.Logger) *Service {
	return &Service{storage: s, sourceRepo: sr, logger: logger}
}
