package model

import (
	"time"

	"github.com/gitrus/digikeeper-log/internal/domain/core"
)

type Candidate struct {
	ID                string     `json:"id"`
	EntryID           string     `json:"entry_id"`
	OriginalTimestamp time.Time  `json:"original_ts"`
	Entry             core.Entry `json:"entry"`
	CreatedAt         time.Time  `json:"created_at"`
}
