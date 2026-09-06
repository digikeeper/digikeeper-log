package query

import (
	"time"

	"github.com/digikeeper/digikeeper-journal/internal/domain/core"
)

type RecordResource struct {
	id    string
	attrs RecordResourceAttrs
}

type RecordResourceMeta struct {
	SchemaVersion int    `json:"schema_version"`
	Revision      int    `json:"revision"`
	Source        string `json:"source"`
}

type RecordResourceAttrs struct {
	RequestID string    `json:"request_id"`
	CreatedAt time.Time `json:"created_at"`
	Timestamp time.Time `json:"timestamp"`

	Meta RecordResourceMeta `json:"meta"`

	Type string         `json:"type"`
	Tags []string       `json:"tags"`
	Data map[string]any `json:"data"`
}

func (r RecordResource) GetID() string      { return r.id }
func (r RecordResource) GetAttributes() any { return r.attrs }

func NewRecordResource(e core.Record, resolve func(int) string) RecordResource {
	return RecordResource{
		id: e.ID,
		attrs: RecordResourceAttrs{
			Type: e.Type,
			Meta: RecordResourceMeta{
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
