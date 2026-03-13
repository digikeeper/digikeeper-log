package query

import (
	"context"
	"fmt"
	"log/slog"
	"sort"

	"slices"

	"github.com/gitrus/digikeeper-log/internal/domain/model"
)

// SearchEntries is the single Query operation.
// It uses the metadata index to locate relevant log files, reads the actual
// entries from those files, then applies entry-level filtering and the caller's limit.
func (s *Service) SearchEntries(
	ctx context.Context,
	p model.SearchParams,
) ([]model.Entry, error) {
	keys, err := s.metaStorage.Search(ctx, p)
	if err != nil {
		return nil, fmt.Errorf("query: meta search: %w", err)
	}

	entries, err := s.storage.Read(ctx, keys)
	if err != nil {
		return nil, fmt.Errorf("query: read entries: %w", err)
	}

	results := filterEntries(entries, p)

	s.log.InfoContext(ctx, "search completed",
		slog.String("tag", p.Tag),
		slog.Int("files", len(keys)),
		slog.Int("raw_entries", len(entries)),
		slog.Int("count", len(results)),
		slog.Int("limit", p.Limit),
	)

	return results, nil
}

func filterEntries(entries []model.Entry, p model.SearchParams) []model.Entry {
	limit := p.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}

	var result []model.Entry
	for _, e := range entries {
		if !p.From.IsZero() && e.Timestamp.Before(p.From) {
			continue
		}
		if !p.To.IsZero() && e.Timestamp.After(p.To) {
			continue
		}
		if p.Tag != "" && !slices.Contains(e.Tags, p.Tag) {
			continue
		}
		result = append(result, e)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp.After(result[j].Timestamp)
	})
	if len(result) > limit {
		result = result[:limit]
	}
	return result
}
