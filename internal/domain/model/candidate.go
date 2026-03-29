package model

import "time"

// Candidate is a proposed replacement for an existing log entry.
// It carries the full replacement payload and enough metadata to locate
// the original entry's partition.
type Candidate struct {
	ID string `json:"id"`

	// EntryID is the original entry being replaced (matched by ID).
	EntryID string `json:"entry_id"`
	// OriginalTimestamp is the original entry's Timestamp, to derive the partition.
	OriginalTimestamp time.Time `json:"original_ts"`
	// Entry is the full replacement
	Entry Entry `json:"entry"`

	CreatedAt time.Time `json:"created_at"`
}

type Resolution struct {
	CandidateID string           `json:"candidate_id"`
	Action      ResolutionAction `json:"action"`

	// Audit metadata — populated at resolve time.
	ResolvedBy string    `json:"resolved_by"`
	ResolvedAt time.Time `json:"resolved_at"`
	Reason     string    `json:"reason,omitempty"`

	ClientID string `json:"client_id"`
}
