package jsonlstore

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gitrus/digikeeper-log/internal/domain/model"
)

var mar1_10 = time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
var mar1_14 = time.Date(2026, 3, 1, 14, 0, 0, 0, time.UTC)
var mar1_20 = time.Date(2026, 3, 1, 20, 0, 0, 0, time.UTC)

func testEntries() []model.Entry {
	return []model.Entry{
		{ID: "1", Timestamp: mar1_10, Tags: []string{"work"}, Data: map[string]any{"note": "morning standup"}},
		{ID: "2", Timestamp: mar1_14, Tags: []string{"work", "meeting"}, Data: map[string]any{"note": "sprint review"}},
		{ID: "3", Timestamp: mar1_20, Tags: []string{"personal"}, Data: map[string]any{"note": "gym"}},
	}
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

func TestRead_NoFilters(t *testing.T) {
	w, relPath := setupWriter(t)
	defer w.Close()

	entries, err := w.Read(relPath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("got %d entries, want 3", len(entries))
	}
}

func TestRead_WithTags_SingleMatch(t *testing.T) {
	w, relPath := setupWriter(t)
	defer w.Close()

	entries, err := w.Read(relPath, WithTags("meeting"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	if entries[0].ID != "2" {
		t.Fatalf("got ID %q, want \"2\"", entries[0].ID)
	}
}

func TestRead_WithTags_MultipleMatch(t *testing.T) {
	w, relPath := setupWriter(t)
	defer w.Close()

	entries, err := w.Read(relPath, WithTags("work"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}
}

func TestRead_WithTags_NoMatch(t *testing.T) {
	w, relPath := setupWriter(t)
	defer w.Close()

	entries, err := w.Read(relPath, WithTags("travel"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("got %d entries, want 0", len(entries))
	}
}

func TestRead_WithTimeRange(t *testing.T) {
	w, relPath := setupWriter(t)
	defer w.Close()

	from := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	to := time.Date(2026, 3, 1, 22, 0, 0, 0, time.UTC)
	entries, err := w.Read(relPath, WithTimeRange(from, to))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}
}

func TestRead_WithTimeRange_FromOnly(t *testing.T) {
	w, relPath := setupWriter(t)
	defer w.Close()

	from := time.Date(2026, 3, 1, 15, 0, 0, 0, time.UTC)
	entries, err := w.Read(relPath, WithTimeRange(from, time.Time{}))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	if entries[0].ID != "3" {
		t.Fatalf("got ID %q, want \"3\"", entries[0].ID)
	}
}

func TestRead_WithTimeRange_ToOnly(t *testing.T) {
	w, relPath := setupWriter(t)
	defer w.Close()

	to := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	entries, err := w.Read(relPath, WithTimeRange(time.Time{}, to))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	if entries[0].ID != "1" {
		t.Fatalf("got ID %q, want \"1\"", entries[0].ID)
	}
}

func TestRead_CombinedFilters(t *testing.T) {
	w, relPath := setupWriter(t)
	defer w.Close()

	from := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	to := time.Date(2026, 3, 1, 23, 0, 0, 0, time.UTC)
	entries, err := w.Read(relPath, WithTags("work"), WithTimeRange(from, to))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	if entries[0].ID != "2" {
		t.Fatalf("got ID %q, want \"2\"", entries[0].ID)
	}
}

func TestRead_FileNotFound(t *testing.T) {
	w := NewJSONLWriter(t.TempDir(), "logs")
	defer w.Close()

	entries, err := w.Read("nonexistent/path.jsonl")
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if entries != nil {
		t.Fatalf("expected nil entries, got %d", len(entries))
	}
}

func TestRead_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	relPath := "2026/2026-03-01_logs.jsonl"
	fpath := filepath.Join(dir, relPath)
	if err := os.MkdirAll(filepath.Dir(fpath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fpath, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	w := NewJSONLWriter(dir, "logs")
	defer w.Close()

	entries, err := w.Read(relPath)
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(entries))
	}
}

func TestMatchFilters_TagInDataNotMatched(t *testing.T) {
	line := []byte(`{"tags":["a"],"timestamp":"2026-03-01T10:00:00Z","data":{"note":"meeting"}}`)
	f := &ReadFilters{Tags: map[string]struct{}{"meeting": {}}}
	if matchFilters(line, f) {
		t.Fatal("expected false: 'meeting' is in data, not in tags")
	}
}

func TestMatchFilters_EmptyTagsArray(t *testing.T) {
	line := []byte(`{"tags":[],"timestamp":"2026-03-01T10:00:00Z"}`)
	f := &ReadFilters{Tags: map[string]struct{}{"work": {}}}
	if matchFilters(line, f) {
		t.Fatal("expected false: tags array is empty")
	}
}

func TestMatchFilters_NoTagsField(t *testing.T) {
	line := []byte(`{"timestamp":"2026-03-01T10:00:00Z","data":{}}`)
	f := &ReadFilters{Tags: map[string]struct{}{"work": {}}}
	if matchFilters(line, f) {
		t.Fatal("expected false: no tags field")
	}
}
