package query

import (
	"context"
	"fmt"
	"log/slog"
	"sort"

	"slices"

	"github.com/gitrus/digikeeper-log/internal/domain/core"
	"github.com/gitrus/digikeeper-log/internal/domain/query/model"
)

// SearchEntries is the single Query operation.
// It uses the metadata index to locate relevant log files, reads the actual
// entries from those files, then applies entry-level filtering and the caller's limit.
func (s *Service) SearchEntries(
	ctx context.Context,
	p model.SearchParams,
) ([]core.Entry, error) {
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
		slog.Any("tags", p.Tags),
		slog.Any("types", p.Types),
		slog.Int("files", len(keys)),
		slog.Int("raw_entries", len(entries)),
		slog.Int("count", len(results)),
		slog.Int("limit", p.Limit),
	)

	return results, nil
}

func filterEntries(entries []core.Entry, p model.SearchParams) []core.Entry {
	limit := p.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}

	var result []core.Entry
	for _, e := range entries {
		if !p.From.IsZero() && e.Timestamp.Before(p.From) {
			continue
		}
		if !p.To.IsZero() && e.Timestamp.After(p.To) {
			continue
		}
		// OR: entry must contain at least one of the requested tags
		if len(p.Tags) > 0 && !slices.ContainsFunc(p.Tags, func(t string) bool {
			return slices.Contains(e.Tags, t)
		}) {
			continue
		}
		// OR: entry type must be one of the requested types
		if len(p.Types) > 0 && !slices.Contains(p.Types, e.Type) {
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
