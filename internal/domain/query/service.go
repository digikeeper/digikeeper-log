package query

import (
	"context"
	"log/slog"

	"github.com/gitrus/digikeeper-log/internal/domain/model"
)

type MetaStorage interface {
	Search(ctx context.Context, p model.SearchParams) ([]string, error)
}

type Storage interface {
	Read(ctx context.Context, keys []string) ([]model.Entry, error)
}

type Service struct {
	storage     Storage
	metaStorage MetaStorage
	log         *slog.Logger
}

func NewService(s Storage, ms MetaStorage, log *slog.Logger) *Service {
	return &Service{storage: s, metaStorage: ms, log: log}
}
