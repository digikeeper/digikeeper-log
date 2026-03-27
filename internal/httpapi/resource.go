package httpapi

import (
	"time"

	"github.com/gitrus/digikeeper-log/internal/domain/model"
)

type EntryResource struct {
	id    string
	attrs EntryResourceAttrs
}

type EntryResourceAttrs struct {
	Type      string          `json:"type"`
	Meta      model.EntryMeta `json:"meta"`
	RequestID string          `json:"request_id"`
	CreatedAt time.Time       `json:"created_at"`
	Timestamp time.Time       `json:"timestamp"`
	Tags      []string        `json:"tags"`
	Data      map[string]any  `json:"data"`
}

func (r EntryResource) GetID() string      { return r.id }
func (r EntryResource) GetAttributes() any { return r.attrs }

func NewEntryResource(e model.Entry) EntryResource {
	return EntryResource{
		id: e.ID,
		attrs: EntryResourceAttrs{
			Type:      e.Type,
			Meta:      e.Meta,
			RequestID: e.RequestID,
			CreatedAt: e.CreatedAt,
			Timestamp: e.Timestamp,
			Tags:      e.Tags,
			Data:      e.Data,
		},
	}
}
