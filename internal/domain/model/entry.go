package model

import (
	"time"
)

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

type SearchParams struct {
	Tag   string
	From  time.Time
	To    time.Time
	Limit int
}
