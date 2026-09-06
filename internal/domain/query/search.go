package query

import (
	"context"
	"fmt"
	"log/slog"
	"sort"

	"slices"

	"github.com/digikeeper/digikeeper-journal/internal/domain/core"
	"github.com/digikeeper/digikeeper-journal/internal/domain/query/model"
)

// SearchRecords is the single Query operation.
// It uses the metadata index to locate relevant journal file, reads the actual
// records from those files, then applies record-level filtering and the caller's limit.
func (s *Service) SearchRecords(
	ctx context.Context,
	p model.SearchParams,
) ([]core.Record, error) {
	keys, err := s.metaStorage.Search(ctx, p)
	if err != nil {
		return nil, fmt.Errorf("query: meta search: %w", err)
	}

	records, err := s.storage.Read(ctx, keys)
	if err != nil {
		return nil, fmt.Errorf("query: read records: %w", err)
	}

	results := filterRecords(records, p)

	s.log.InfoContext(ctx, "search completed",
		slog.Any("tags", p.Tags),
		slog.Any("types", p.Types),
		slog.Int("files", len(keys)),
		slog.Int("raw_records", len(records)),
		slog.Int("count", len(results)),
		slog.Int("limit", p.Limit),
	)

	return results, nil
}

func filterRecords(records []core.Record, p model.SearchParams) []core.Record {
	limit := p.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}

	var result []core.Record
	for _, e := range records {
		if !p.From.IsZero() && e.Timestamp.Before(p.From) {
			continue
		}
		if !p.To.IsZero() && e.Timestamp.After(p.To) {
			continue
		}
		// OR: record must contain at least one of the requested tags
		if len(p.Tags) > 0 && !slices.ContainsFunc(p.Tags, func(t string) bool {
			return slices.Contains(e.Tags, t)
		}) {
			continue
		}
		// OR: record type must be one of the requested types
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
