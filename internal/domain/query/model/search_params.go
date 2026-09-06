package model

import "time"

type SearchParams struct {
	Tags  []string
	Types []string
	From  time.Time
	To    time.Time
	Limit int
}
