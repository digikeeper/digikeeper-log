package candidate

import (
	"time"

	"github.com/gitrus/digikeeper-log/internal/domain/model"
)

// SubmitRequest is the input for submitting a candidate replacement.
type SubmitRequest struct {
	//  ID of the original entry.
	EntryID string `json:"entry_id"`
	// OriginalTimestamp is the original entry's Timestamp, used to locate the partition.
	OriginalTimestamp time.Time `json:"original_timestamp"`

	// Type, Tags, and Data are the replacement values.
	Type string         `json:"type"`
	Tags []string       `json:"tags"`
	Data map[string]any `json:"data"`

	ClientID string `json:"-"`
}

// ResolveRequest is the input for resolving one or more candidates.
type ResolveRequest struct {
	Resolutions []ResolveItem `json:"resolutions"`

	// ResolvedBy identifies who is performing the resolution (user/system ID).
	ResolvedBy string `json:"-"`
}

// ResolveItem is a single resolution decision from the caller.
type ResolveItem struct {
	CandidateID string                `json:"candidate_id"`
	Action      model.ResolutionAction `json:"action"`
	Reason      string                `json:"reason,omitempty"`
}

