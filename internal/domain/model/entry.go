package model

import (
	_ "embed"
	"fmt"
	"time"

	"encoding/json/v2"
)

//go:embed sources.json
var sourcesRaw []byte

// clientSources maps client name → numeric source ID, loaded from sources.json.
var clientSources map[string]int

func init() {
	if err := json.Unmarshal(sourcesRaw, &clientSources); err != nil {
		panic(fmt.Sprintf("model: unmarshal sources.json: %v", err))
	}
}

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

// NewSourceResolver builds a SourceResolver from the embedded sources.
func NewSourceResolver() SourceResolver {
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

// ResolveSrc returns the numeric source ID for the given client name.
func ResolveSrc(clientID string) int {
	return clientSources[clientID]
}

type SearchParams struct {
	Tag   string
	From  time.Time
	To    time.Time
	Limit int
}
