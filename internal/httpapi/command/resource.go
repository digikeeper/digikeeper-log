package command

import (
	"time"

	commandmodel "github.com/gitrus/digikeeper-log/internal/domain/command/model"
	"github.com/gitrus/digikeeper-log/internal/domain/core"
)

type EntryResource struct {
	id    string
	attrs EntryResourceAttrs
}

type EntryResourceMeta struct {
	SchemaVersion int    `json:"schema_version"`
	Revision      int    `json:"revision"`
	Source        string `json:"source"`
}

type EntryResourceAttrs struct {
	RequestID string    `json:"request_id"`
	CreatedAt time.Time `json:"created_at"`
	Timestamp time.Time `json:"timestamp"`

	Meta EntryResourceMeta `json:"meta"`

	Type string         `json:"type"`
	Tags []string       `json:"tags"`
	Data map[string]any `json:"data"`
}

func (r EntryResource) GetID() string      { return r.id }
func (r EntryResource) GetAttributes() any { return r.attrs }

func NewEntryResource(e core.Entry, resolve func(int) string) EntryResource {
	return EntryResource{
		id: e.ID,
		attrs: EntryResourceAttrs{
			Type: e.Type,
			Meta: EntryResourceMeta{
				SchemaVersion: e.Meta.SchemaVersion,
				Revision:      e.Meta.Revision,
				Source:        resolve(e.Meta.Src),
			},
			RequestID: e.RequestID,
			CreatedAt: e.CreatedAt,
			Timestamp: e.Timestamp,
			Tags:      e.Tags,
			Data:      e.Data,
		},
	}
}

type CandidateResource struct {
	id    string
	attrs CandidateResourceAttrs
}

type CandidateResourceAttrs struct {
	EntryID           string                   `json:"entry_id"`
	OriginalTimestamp time.Time                `json:"original_timestamp"`
	Entry             core.Entry               `json:"entry"`
	CreatedAt         time.Time                `json:"created_at"`
	Action            core.CandidateResolution `json:"action,omitempty"`
	Reason            string                   `json:"reason,omitempty"`
	ClientID          string                   `json:"client_id,omitempty"`

	ResolvedBy string    `json:"resolved_by,omitempty"`
	ResolvedAt time.Time `json:"resolved_at,omitzero"`
}

func (r CandidateResource) GetID() string      { return r.id }
func (r CandidateResource) GetAttributes() any { return r.attrs }

func NewCandidateResource(c commandmodel.Candidate) CandidateResource {
	return CandidateResource{
		id: c.ID,
		attrs: CandidateResourceAttrs{
			EntryID:           c.EntryID,
			OriginalTimestamp: c.OriginalTimestamp,
			Entry:             c.Entry,
			CreatedAt:         c.CreatedAt,
			Action:            c.Action,
			Reason:            c.Reason,
			ClientID:          c.ClientID,

			ResolvedBy: c.ResolvedBy,
			ResolvedAt: c.ResolvedAt,
		},
	}
}
