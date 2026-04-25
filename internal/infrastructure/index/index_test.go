package index

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStoreSearch(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	for _, row := range []Row{
		{
			File:      "dk_logs/2026/2026-03-08_logs.jsonl",
			Tags:      []string{"work", "focus"},
			Types:     []string{"note"},
			Timestamp: mustParseTime(t, "2026-03-08T10:00:00Z"),
		},
		{
			File:      "dk_logs/2026/2026-03-09_logs.jsonl",
			Tags:      []string{"fitness", "health"},
			Types:     []string{"exercise"},
			Timestamp: mustParseTime(t, "2026-03-09T10:00:00Z"),
		},
		{
			File:      "dk_logs/2026/2026-03-10_logs.jsonl",
			Tags:      []string{"health", "nutrition"},
			Types:     []string{"meal"},
			Timestamp: mustParseTime(t, "2026-03-10T10:00:00Z"),
		},
	} {
		mustInsertIndexRow(t, store, row)
	}

	tests := []struct {
		name      string
		params    SearchParams
		wantFiles []string
	}{
		{
			name:   "repeated tags use OR semantics",
			params: SearchParams{Tags: []string{"fitness", "nutrition"}},
			wantFiles: []string{
				"dk_logs/2026/2026-03-10_logs.jsonl",
				"dk_logs/2026/2026-03-09_logs.jsonl",
			},
		},
		{
			name:   "repeated types use OR semantics",
			params: SearchParams{Types: []string{"exercise", "meal"}},
			wantFiles: []string{
				"dk_logs/2026/2026-03-10_logs.jsonl",
				"dk_logs/2026/2026-03-09_logs.jsonl",
			},
		},
		{
			name:      "tag and type filters are combined",
			params:    SearchParams{Tags: []string{"health"}, Types: []string{"meal"}},
			wantFiles: []string{"dk_logs/2026/2026-03-10_logs.jsonl"},
		},
		{
			name: "time filters match overlapping files",
			params: SearchParams{
				From: mustParseTime(t, "2026-03-09T00:00:00Z"),
				To:   mustParseTime(t, "2026-03-09T23:59:59Z"),
			},
			wantFiles: []string{"dk_logs/2026/2026-03-09_logs.jsonl"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := store.Search(t.Context(), tt.params)
			require.NoError(t, err)
			assert.Equal(t, tt.wantFiles, resultFiles(results))
		})
	}
}

func TestStoreInsertMergesFileMetadata(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		rows      []Row
		params    SearchParams
		want      Result
		wantTags  []string
		wantTypes []string
	}{
		{
			name: "same file merges tags types and time range",
			rows: []Row{
				{
					File:      "dk_logs/2026/2026-03-08_logs.jsonl",
					Tags:      []string{"work"},
					Types:     []string{"note"},
					Timestamp: mustParseTime(t, "2026-03-08T10:00:00Z"),
				},
				{
					File:      "dk_logs/2026/2026-03-08_logs.jsonl",
					Tags:      []string{"fitness"},
					Types:     []string{"exercise"},
					Timestamp: mustParseTime(t, "2026-03-08T14:00:00Z"),
				},
			},
			params: SearchParams{
				Tags:  []string{"fitness"},
				Types: []string{"exercise"},
			},
			want: Result{
				File:  "dk_logs/2026/2026-03-08_logs.jsonl",
				MinTS: mustParseTime(t, "2026-03-08T10:00:00Z"),
				MaxTS: mustParseTime(t, "2026-03-08T14:00:00Z"),
			},
			wantTags:  []string{"work", "fitness"},
			wantTypes: []string{"note", "exercise"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			store := newTestStore(t)
			for _, row := range tt.rows {
				mustInsertIndexRow(t, store, row)
			}

			results, err := store.Search(t.Context(), tt.params)
			require.NoError(t, err)
			require.Len(t, results, 1)
			assert.Equal(t, tt.want.File, results[0].File)
			assert.Equal(t, tt.want.MinTS, results[0].MinTS)
			assert.Equal(t, tt.want.MaxTS, results[0].MaxTS)
			assert.ElementsMatch(t, tt.wantTags, results[0].Tags)
			assert.ElementsMatch(t, tt.wantTypes, results[0].Types)
		})
	}
}

func newTestStore(t *testing.T) *Store {
	t.Helper()

	store, err := New(filepath.Join(t.TempDir(), "index.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func mustInsertIndexRow(t *testing.T, store *Store, row Row) {
	t.Helper()

	require.NoError(t, store.Insert(t.Context(), row))
}

func mustParseTime(t *testing.T, value string) time.Time {
	t.Helper()

	ts, err := time.Parse(time.RFC3339, value)
	require.NoError(t, err)
	return ts
}

func resultFiles(results []Result) []string {
	files := make([]string, 0, len(results))
	for _, result := range results {
		files = append(files, result.File)
	}
	return files
}
