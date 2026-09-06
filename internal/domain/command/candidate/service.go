package candidate

import (
	"context"
	"log/slog"

	"github.com/digikeeper/digikeeper-journal/internal/domain/command/model"
	"github.com/digikeeper/digikeeper-journal/internal/domain/core"
)

// Storage manages candidate lifecycle: append, list pending, move, and partition locking.
type Storage interface {
	SharedLock(ctx context.Context, partition core.Partition) (release func(), err error)
	ExclusiveLock(ctx context.Context, partition core.Partition) (release func(), err error)
	AppendCandidate(ctx context.Context, c model.Candidate) error
	ListPending(ctx context.Context, partition core.Partition) ([]model.Candidate, error)
	MoveCandidates(ctx context.Context, partition core.Partition, applied, denied []model.Candidate) error
}

// JournalStorage reads existing journal records to verify the original exists.
type JournalStorage interface {
	ReadRecord(ctx context.Context, recordID string, partition core.Partition) (core.Record, error)
}

// Service handles candidate commands: submit and resolve.
type Service struct {
	storage        Storage
	journalStorage JournalStorage
	logger         *slog.Logger
}

func NewService(s Storage, ls JournalStorage, logger *slog.Logger) *Service {
	return &Service{storage: s, journalStorage: ls, logger: logger}
}
