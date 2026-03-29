package candidate

import (
	"context"
	"log/slog"

	"github.com/gitrus/digikeeper-log/internal/domain/command/model"
	"github.com/gitrus/digikeeper-log/internal/domain/core"
)

// CandidateLocker serializes submit and resolve on the candidate partition.
type CandidateLocker interface {
	SharedLock(ctx context.Context, partition core.Partition) (release func(), err error)
	ExclusiveLock(ctx context.Context, partition core.Partition) (release func(), err error)
}

// Storage manages candidate lifecycle: append, list pending, and move.
type Storage interface {
	AppendCandidate(ctx context.Context, c model.Candidate) error
	ListPending(ctx context.Context, partition core.Partition) ([]model.Candidate, error)
	MoveCandidates(ctx context.Context, partition core.Partition, applied, denied []model.Candidate) error
}

// LogStorage reads existing log entries to verify the original exists.
type LogStorage interface {
	ReadEntry(ctx context.Context, entryID string, partition core.Partition) (core.Entry, error)
}

// Service handles candidate commands: submit and resolve.
type Service struct {
	candidateLocker CandidateLocker
	storage         Storage
	logStorage      LogStorage
	logger          *slog.Logger
}

func NewService(candidateLocker CandidateLocker, s Storage, ls LogStorage, logger *slog.Logger) *Service {
	return &Service{candidateLocker: candidateLocker, storage: s, logStorage: ls, logger: logger}
}
