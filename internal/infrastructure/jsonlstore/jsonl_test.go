package jsonlstore

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gitrus/digikeeper-log/internal/domain/core"
)

func testEntries() []core.Entry {
	return []core.Entry{
		{ID: "1", Timestamp: march1At(10), Tags: []string{"work"}, Data: map[string]any{"note": "morning standup"}},
		{ID: "2", Timestamp: march1At(14), Tags: []string{"work", "meeting"}, Data: map[string]any{"note": "sprint review"}},
		{ID: "3", Timestamp: march1At(20), Tags: []string{"personal"}, Data: map[string]any{"note": "gym"}},
	}
}

func march1At(hour int) time.Time {
	return time.Date(2026, 3, 1, hour, 0, 0, 0, time.UTC)
}

func setupWriter(t *testing.T) (*JSONLWriter, string) {
	t.Helper()
	dir := t.TempDir()
	w := NewJSONLWriter(dir, "logs")
	var relPath string
	for _, e := range testEntries() {
		rp, err := w.Append(e)
		if err != nil {
			t.Fatalf("append: %v", err)
		}
		relPath = rp
	}
	return w, relPath
}

func closeWriter(t *testing.T, w *JSONLWriter) {
	t.Helper()
	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
}

func TestRead_Filters(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		opts    []ReadOption
		wantIDs []string
	}{
		{
			name:    "no filters",
			wantIDs: []string{"1", "2", "3"},
		},
		{
			name:    "tag matches multiple entries",
			opts:    []ReadOption{WithTags("work")},
			wantIDs: []string{"1", "2"},
		},
		{
			name:    "tag has no match",
			opts:    []ReadOption{WithTags("travel")},
			wantIDs: []string{},
		},
		{
			name:    "bounded time range",
			opts:    []ReadOption{WithTimeRange(march1At(12), march1At(22))},
			wantIDs: []string{"2", "3"},
		},
		{
			name:    "from only",
			opts:    []ReadOption{WithTimeRange(march1At(15), time.Time{})},
			wantIDs: []string{"3"},
		},
		{
			name:    "to only",
			opts:    []ReadOption{WithTimeRange(time.Time{}, march1At(12))},
			wantIDs: []string{"1"},
		},
		{
			name:    "combined filters",
			opts:    []ReadOption{WithTags("work"), WithTimeRange(march1At(12), march1At(23))},
			wantIDs: []string{"2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			w, relPath := setupWriter(t)
			t.Cleanup(func() { closeWriter(t, w) })

			entries, err := w.Read(relPath, tt.opts...)
			require.NoError(t, err)
			assert.Equal(t, tt.wantIDs, entryIDs(entries))
		})
	}
}

func TestRead_FileNotFound(t *testing.T) {
	t.Parallel()

	w := NewJSONLWriter(t.TempDir(), "logs")
	t.Cleanup(func() { closeWriter(t, w) })

	entries, err := w.Read("nonexistent/path.jsonl")
	require.NoError(t, err)
	assert.Nil(t, entries)
}

func TestRead_EmptyFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	relPath := "2026/2026-03-01_logs.jsonl"
	fpath := filepath.Join(dir, relPath)
	require.NoError(t, os.MkdirAll(filepath.Dir(fpath), 0o755))
	require.NoError(t, os.WriteFile(fpath, nil, 0o644))

	w := NewJSONLWriter(dir, "logs")
	t.Cleanup(func() { closeWriter(t, w) })

	entries, err := w.Read(relPath)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestMatchFilters_TagEdges(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		line []byte
		tags map[string]struct{}
	}{
		{
			name: "tag value inside data is ignored",
			line: []byte(`{"tags":["a"],"ts":"2026-03-01T10:00:00Z","d":{"note":"meeting"}}`),
			tags: map[string]struct{}{"meeting": {}},
		},
		{
			name: "empty tags array does not match",
			line: []byte(`{"tags":[],"ts":"2026-03-01T10:00:00Z"}`),
			tags: map[string]struct{}{"work": {}},
		},
		{
			name: "missing tags field does not match",
			line: []byte(`{"ts":"2026-03-01T10:00:00Z","d":{}}`),
			tags: map[string]struct{}{"work": {}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			f := &ReadFilters{Tags: tt.tags}
			assert.False(t, matchFilters(tt.line, f))
		})
	}
}

func entryIDs(entries []core.Entry) []string {
	ids := make([]string, 0, len(entries))
	for _, entry := range entries {
		ids = append(ids, entry.ID)
	}
	return ids
}
