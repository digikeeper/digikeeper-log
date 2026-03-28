package model

import "time"

type EntryMeta struct {
	Version int `json:"v"`
	Src     int `json:"s"`
}

type Entry struct {
	ID        string         `json:"id"`
	Type      string         `json:"type"`
	Meta      EntryMeta      `json:"m"`
	RequestID string         `json:"request_id"`
	CreatedAt time.Time      `json:"created_at"`
	Timestamp time.Time      `json:"ts"`
	Tags      []string       `json:"tags"`
	Data      map[string]any `json:"d"`
}

// SourceResolver maps numeric source IDs back to their string names.
type SourceResolver func(int) string

// NewSourceResolver builds a SourceResolver from a forward clientSources map.
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

type SearchParams struct {
	Tag   string
	From  time.Time
	To    time.Time
	Limit int
}
