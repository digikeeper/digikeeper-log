package core

import "time"

type EntryMeta struct {
	Version int `json:"v"`
	Src     int `json:"s"`
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
