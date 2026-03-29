package candidate

import (
	"context"
	"log/slog"

	"github.com/gitrus/digikeeper-log/internal/domain/model"
)

// Storage writes candidates to the pending area.
type Storage interface {
	AppendCandidate(ctx context.Context, c model.Candidate) error
}

// PendingReader reads pending candidates for validation during resolve.
type PendingReader interface {
	ListPending(ctx context.Context, partition string) ([]model.Candidate, error)
}

// LogStorage reads existing log entries to verify the original exists.
type LogStorage interface {
	ReadEntry(ctx context.Context, entryID string, partitionHint string) (model.Entry, error)
}

// Service handles candidate commands: submit and resolve.
type Service struct {
	storage       Storage
	pendingReader PendingReader
	logStorage    LogStorage
	logger        *slog.Logger
}

func NewService(s Storage, pr PendingReader, ls LogStorage, logger *slog.Logger) *Service {
	return &Service{storage: s, pendingReader: pr, logStorage: ls, logger: logger}
}
