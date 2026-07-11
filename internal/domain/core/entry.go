package core

import (
	"encoding/json"
	"time"
)

type EntryMeta struct {
	SchemaVersion int `json:"sv"`
	Revision      int `json:"r"`
	Src           int `json:"s"`
}

// UnmarshalJSON accepts the former "v" alias so existing JSONL entries remain
// readable. New writes use the unambiguous "sv" alias.
func (m *EntryMeta) UnmarshalJSON(data []byte) error {
	var raw struct {
		SchemaVersion       *int `json:"sv"`
		LegacySchemaVersion *int `json:"v"`
		Revision            int  `json:"r"`
		Src                 int  `json:"s"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	m.Revision = raw.Revision
	m.Src = raw.Src
	switch {
	case raw.SchemaVersion != nil:
		m.SchemaVersion = *raw.SchemaVersion
	case raw.LegacySchemaVersion != nil:
		m.SchemaVersion = *raw.LegacySchemaVersion
	default:
		m.SchemaVersion = 0
	}
	return nil
}

type Entry struct {
	ID        string         `json:"id"`
	RequestID string         `json:"request_id"`
	CreatedAt time.Time      `json:"created_at"`
	Meta      EntryMeta      `json:"m"`
	Timestamp time.Time      `json:"ts"`
	Type      string         `json:"type"`
	Tags      []string       `json:"tags"`
	Data      map[string]any `json:"d"`
}
