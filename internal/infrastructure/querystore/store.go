package querystore

import (
	"context"
	"fmt"

	"github.com/gitrus/digikeeper-log/internal/domain/core"
	"github.com/gitrus/digikeeper-log/internal/domain/query/model"
	"github.com/gitrus/digikeeper-log/internal/infrastructure/index"
	"github.com/gitrus/digikeeper-log/internal/infrastructure/jsonlstore"
)

type Store struct {
	rawStore *jsonlstore.JSONLWriter
	idx      *index.Store
}

func NewStore(jsonLogsDir string, idx *index.Store) *Store {
	return &Store{
		rawStore: jsonlstore.NewJSONLWriter(jsonLogsDir, "logs"),
		idx:      idx,
	}
}

// Search satisfies query.MetaStorage.
func (s *Store) Search(ctx context.Context, p model.SearchParams) ([]string, error) {
	results, err := s.idx.Search(ctx, index.SearchParams{
		Tags:  p.Tags,
		Types: p.Types,
		From:  p.From,
		To:    p.To,
	})
	if err != nil {
		return nil, err
	}
	keys := make([]string, len(results))
	for i, r := range results {
		keys[i] = r.File
	}
	return keys, nil
}

// Read satisfies query.Storage.
func (s *Store) Read(ctx context.Context, keys []string) ([]core.Entry, error) {
	var entries []core.Entry
	for _, key := range keys {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("store: read cancelled: %w", err)
		}
		fileEntries, err := s.rawStore.Read(key)
		if err != nil {
			return nil, fmt.Errorf("store: read %s: %w", key, err)
		}
		entries = append(entries, fileEntries...)
	}
	return entries, nil
}
