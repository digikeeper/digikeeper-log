package compaction

import (
	"time"

	"github.com/digikeeper/digikeeper-journal/internal/domain/core"
)

// CompactRequest specifies which partition to compact.
// The compactor reads applied candidates from storage directly.
type CompactRequest struct {
	Partition core.Partition `json:"partition"`
}

// CandidateAuditEvent records a completed compaction for the audit trail.
type CandidateAuditEvent struct {
	Partition    core.Partition `json:"partition"`
	AppliedCount int            `json:"applied_count"`
	CompletedAt  time.Time      `json:"completed_at"`
}
