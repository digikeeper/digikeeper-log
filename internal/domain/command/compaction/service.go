package compaction

import (
	"context"
	"log/slog"

	"github.com/gitrus/digikeeper-log/internal/domain/command/model"
	"github.com/gitrus/digikeeper-log/internal/domain/core"
)

// LogStorage reads, rewrites, and locks log partitions.
type LogStorage interface {
	ExclusiveLock(ctx context.Context, partition core.Partition) (release func(), err error)
	ReadPartition(ctx context.Context, partition core.Partition) ([]core.Entry, error)
	ReplacePartition(ctx context.Context, partition core.Partition, entries []core.Entry) error
}

// CandidateStorage reads, cleans up, audits, and locks candidate partitions.
type CandidateStorage interface {
	ExclusiveLock(ctx context.Context, partition core.Partition) (release func(), err error)
	// ListApplied returns resolved-apply candidates awaiting compaction.
	ListApplied(ctx context.Context, partition core.Partition) ([]model.Candidate, error)
	// DeleteApplied removes candidates from applied/ after successful compaction.
	DeleteApplied(ctx context.Context, partition core.Partition, candidateIDs []string) error
	// AuditAppend records a completed compaction event for the audit trail.
	AuditAppend(ctx context.Context, event CandidateAuditEvent) error
}

// IndexRebuilder updates the file-level index after a partition rewrite.
type IndexRebuilder interface {
	RebuildPartition(ctx context.Context, partition core.Partition, entries []core.Entry) error
}

// Service drains applied candidates by rewriting log partitions.
type Service struct {
	logs       LogStorage
	candidates CandidateStorage
	index      IndexRebuilder
	logger     *slog.Logger
}

func NewService(
	logs LogStorage,
	candidates CandidateStorage,
	index IndexRebuilder,
	logger *slog.Logger,
) *Service {
	return &Service{
		logs:       logs,
		candidates: candidates,
		index:      index,
		logger:     logger,
	}
}
