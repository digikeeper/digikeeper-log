package compaction

import (
	"context"
	"log/slog"

	"github.com/digikeeper/digikeeper-journal/internal/domain/command/model"
	"github.com/digikeeper/digikeeper-journal/internal/domain/core"
)

// JournalStorage reads, rewrites, and locks journal partitions.
type JournalStorage interface {
	ExclusiveLock(ctx context.Context, partition core.Partition) (release func(), err error)
	ReadPartition(ctx context.Context, partition core.Partition) ([]core.Record, error)
	ReplacePartition(ctx context.Context, partition core.Partition, records []core.Record) error
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
	RebuildPartition(ctx context.Context, partition core.Partition, records []core.Record) error
}

// Service drains applied candidates by rewriting journal partitions.
type Service struct {
	journalStorage JournalStorage
	candidates     CandidateStorage
	index          IndexRebuilder
	logger         *slog.Logger
}

func NewService(
	journalStorage JournalStorage,
	candidates CandidateStorage,
	index IndexRebuilder,
	logger *slog.Logger,
) *Service {
	return &Service{
		journalStorage: journalStorage,
		candidates:     candidates,
		index:          index,
		logger:         logger,
	}
}
