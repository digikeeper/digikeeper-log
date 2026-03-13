package command

import "time"

type AppendRequest struct {
	Timestamp time.Time      `json:"timestamp"`
	Tags      []string       `json:"tags"`
	Data      map[string]any `json:"data"`
	ClientID  string         `json:"-"`
}
