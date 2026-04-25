package model

import (
	"time"

	"github.com/gitrus/digikeeper-log/internal/domain/core"
)

type Candidate struct {
	ID                string                   `json:"id"`
	EntryID           string                   `json:"entry_id"`
	OriginalTimestamp time.Time                `json:"original_ts"`
	Entry             core.Entry               `json:"entry"`
	CreatedAt         time.Time                `json:"created_at"`
	Action            core.CandidateResolution `json:"action,omitempty"`
	ResolvedBy        string                   `json:"resolved_by,omitempty"`
	ResolvedAt        time.Time                `json:"resolved_at,omitzero"`
	Reason            string                   `json:"reason,omitempty"`
	ClientID          string                   `json:"client_id,omitempty"`
}

func (c *Candidate) IsResolved() bool {
	return c.Action.IsValid()
}
