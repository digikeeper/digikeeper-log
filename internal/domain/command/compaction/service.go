package compaction

import (
	"context"
	"log/slog"

	"github.com/gitrus/digikeeper-log/internal/domain/model"
)

// PartitionLocker provides exclusive access to a partition during compaction
// and shared access during normal appends. Backed by flock on sidecar files.
type PartitionLocker interface {
	// ExclusiveLock blocks until exclusive access to the partition is acquired.
	// Returns a release function.
	ExclusiveLock(ctx context.Context, partition string) (release func(), err error)
}

// LogStorage reads and rewrites log partitions.
type LogStorage interface {
	// ReadPartition returns all entries in the given partition.
	ReadPartition(ctx context.Context, partition string) ([]model.Entry, error)

	// ReplacePartition atomically replaces the partition contents.
	// Must be called while holding the exclusive partition lock.
	ReplacePartition(ctx context.Context, partition string, entries []model.Entry) error
}

// CandidateStorage manages the pending/applied/dismissed candidate lifecycle.
type CandidateStorage interface {
	// ListPending returns all unresolved candidates for the given partition.
	ListPending(ctx context.Context, partition string) ([]model.Candidate, error)

	// MoveCandidates moves candidates from pending to applied or dismissed
	// based on the resolutions.
	MoveCandidates(ctx context.Context, resolutions []ResolvedCandidate) error
}

// JournalStorage records compaction events for audit.
type JournalStorage interface {
	AppendJournal(ctx context.Context, event JournalEvent) error
}

// IndexRebuilder updates the file-level index after a partition rewrite.
type IndexRebuilder interface {
	RebuildPartition(ctx context.Context, partition string, entries []model.Entry) error
}

// Service executes compaction: resolves candidates by rewriting log partitions.
//
// The compactor operates on log storage directly, not through the writer.
// Coordination with concurrent appenders is handled by PartitionLocker (flock).
type Service struct {
	locker     PartitionLocker
	logs       LogStorage
	candidates CandidateStorage
	journal    JournalStorage
	index      IndexRebuilder
	logger     *slog.Logger
}

func NewService(
	locker PartitionLocker,
	logs LogStorage,
	candidates CandidateStorage,
	journal JournalStorage,
	index IndexRebuilder,
	logger *slog.Logger,
) *Service {
	return &Service{
		locker:     locker,
		logs:       logs,
		candidates: candidates,
		journal:    journal,
		index:      index,
		logger:     logger,
	}
}
