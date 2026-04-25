package compaction

import (
	"context"
	"log/slog"

	"github.com/gitrus/digikeeper-log/internal/domain/command/model"
	"github.com/gitrus/digikeeper-log/internal/domain/core"
)

// PartitionLocker provides exclusive access to the entry (log) partition during compaction.
// Blocks concurrent appenders (which hold shared locks).
type PartitionLocker interface {
	ExclusiveLock(ctx context.Context, partition core.Partition) (release func(), err error)
}

// CandidateLocker provides exclusive access to the candidate partition during compaction.
// Blocks concurrent resolve (exclusive candidate lock) and submit (shared candidate lock).
type CandidateLocker interface {
	ExclusiveLock(ctx context.Context, partition core.Partition) (release func(), err error)
}

// LogStorage reads and rewrites log partitions.
type LogStorage interface {
	ReadPartition(ctx context.Context, partition core.Partition) ([]core.Entry, error)
	ReplacePartition(ctx context.Context, partition core.Partition, entries []core.Entry) error
}

// CandidateStorage reads and cleans up applied candidates.
type CandidateStorage interface {
	// ListApplied returns resolved-apply candidates awaiting compaction.
	ListApplied(ctx context.Context, partition core.Partition) ([]model.Candidate, error)

	// DeleteApplied removes candidates from applied/ after successful compaction.
	DeleteApplied(ctx context.Context, partition core.Partition, candidateIDs []string) error
}

// JournalStorage records compaction events for audit.
type JournalStorage interface {
	AppendJournal(ctx context.Context, event JournalEvent) error
}

// IndexRebuilder updates the file-level index after a partition rewrite.
type IndexRebuilder interface {
	RebuildPartition(ctx context.Context, partition core.Partition, entries []core.Entry) error
}

// Service drains applied candidates by rewriting log partitions.
type Service struct {
	locker          PartitionLocker
	candidateLocker CandidateLocker
	logs            LogStorage
	candidates      CandidateStorage
	journal         JournalStorage
	index           IndexRebuilder
	logger          *slog.Logger
}

func NewService(
	locker PartitionLocker,
	candidateLocker CandidateLocker,
	logs LogStorage,
	candidates CandidateStorage,
	journal JournalStorage,
	index IndexRebuilder,
	logger *slog.Logger,
) *Service {
	return &Service{
		locker:          locker,
		candidateLocker: candidateLocker,
		logs:            logs,
		candidates:      candidates,
		journal:         journal,
		index:           index,
		logger:          logger,
	}
}
