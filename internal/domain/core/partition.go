package core

import (
	"fmt"
	"time"
)

// Partition identifies a storage partition.
// The domain treats it as an opaque key; infrastructure translates it to file paths.
type Partition struct {
	date time.Time
}

// build partition from time.Time
func PartitionFromTime(t time.Time) Partition {
	return Partition{date: time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)}
}

// build partition from string-representation
func ParsePartition(s string) (Partition, error) {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return Partition{}, fmt.Errorf("invalid partition %q: %w", s, err)
	}
	return Partition{date: t}, nil
}

func (p Partition) String() string { return p.date.Format("2006-01-02") }
func (p Partition) Year() int      { return p.date.Year() }
func (p Partition) IsZero() bool   { return p.date.IsZero() }

func (p Partition) MarshalText() ([]byte, error) { return []byte(p.String()), nil }

func (p *Partition) UnmarshalText(b []byte) error {
	parsed, err := ParsePartition(string(b))
	if err != nil {
		return err
	}
	*p = parsed
	return nil
}
