package compaction

import (
	"time"

	"github.com/gitrus/digikeeper-log/internal/domain/model"
)

// CompactRequest specifies which partition to compact and the pre-validated
// resolutions. Resolutions must be validated by the candidate service before
// being passed here.
type CompactRequest struct {
	Partition   string             `json:"partition"`
	Resolutions []model.Resolution `json:"resolutions"`
}

// ResolvedCandidate pairs a candidate with its resolution action.
type ResolvedCandidate struct {
	Candidate model.Candidate
	Action    model.ResolutionAction
}

// JournalEvent records a completed compaction for the audit trail.
type JournalEvent struct {
	Partition      string    `json:"partition"`
	ResolvedCount  int       `json:"resolved_count"`
	AppliedCount   int       `json:"applied_count"`
	DismissedCount int       `json:"dismissed_count"`
	CompletedAt    time.Time `json:"completed_at"`
}
