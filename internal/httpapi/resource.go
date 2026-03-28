package httpapi

import (
	"time"

	"github.com/gitrus/digikeeper-log/internal/domain/model"
)

type EntryResource struct {
	id    string
	attrs EntryResourceAttrs
}

type EntryResourceMeta struct {
	Version int    `json:"version"`
	Source  string `json:"source"`
}

type EntryResourceAttrs struct {
	Type      string            `json:"type"`
	Meta      EntryResourceMeta `json:"meta"`
	RequestID string            `json:"request_id"`
	CreatedAt time.Time         `json:"created_at"`
	Timestamp time.Time         `json:"timestamp"`
	Tags      []string          `json:"tags"`
	Data      map[string]any    `json:"data"`
}

func (r EntryResource) GetID() string      { return r.id }
func (r EntryResource) GetAttributes() any { return r.attrs }

// SourceResolver maps numeric source IDs back to their string names.
type SourceResolver func(int) string

// NewSourceResolver builds a SourceResolver from the forward clientSources map.
func NewSourceResolver(clientSources map[string]int) SourceResolver {
	reverse := make(map[int]string, len(clientSources))
	for name, id := range clientSources {
		reverse[id] = name
	}
	return func(id int) string {
		if name, ok := reverse[id]; ok {
			return name
		}
		return ""
	}
}

func NewEntryResource(e model.Entry, resolve SourceResolver) EntryResource {
	return EntryResource{
		id: e.ID,
		attrs: EntryResourceAttrs{
			Type: e.Type,
			Meta: EntryResourceMeta{
				Version: e.Meta.Version,
				Source:  resolve(e.Meta.Src),
			},
			RequestID: e.RequestID,
			CreatedAt: e.CreatedAt,
			Timestamp: e.Timestamp,
			Tags:      e.Tags,
			Data:      e.Data,
		},
	}
}
