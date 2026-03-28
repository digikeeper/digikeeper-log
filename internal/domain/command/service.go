package command

import (
	"context"
	"log/slog"

	"github.com/gitrus/digikeeper-log/internal/domain/model"
)

type Storage interface {
	Append(ctx context.Context, entry model.Entry) error
}

type Service struct {
	storage Storage
	logger  *slog.Logger
}

func NewService(s Storage, logger *slog.Logger) *Service {
	return &Service{storage: s, logger: logger}
}
