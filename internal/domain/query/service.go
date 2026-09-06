package query

import (
	"context"
	"log/slog"

	"github.com/digikeeper/digikeeper-journal/internal/domain/core"
	"github.com/digikeeper/digikeeper-journal/internal/domain/query/model"
)

type MetaStorage interface {
	Search(ctx context.Context, p model.SearchParams) ([]string, error)
}

type Storage interface {
	Read(ctx context.Context, keys []string) ([]core.Record, error)
}

type Service struct {
	storage     Storage
	metaStorage MetaStorage
	log         *slog.Logger
}

func NewService(s Storage, ms MetaStorage, log *slog.Logger) *Service {
	return &Service{storage: s, metaStorage: ms, log: log}
}
