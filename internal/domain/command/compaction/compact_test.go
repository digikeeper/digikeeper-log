package compaction

import (
	"bytes"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gitrus/digikeeper-log/internal/domain/command/model"
	"github.com/gitrus/digikeeper-log/internal/domain/core"
)

func TestApplySubstitutions_ReplacesAndAppendsMissingCandidates(t *testing.T) {
	t.Parallel()

	entries := []core.Entry{
		{ID: "entry-a", Type: "note", Meta: core.EntryMeta{Revision: 1}},
		{ID: "entry-b", Type: "note", Meta: core.EntryMeta{Revision: 1}},
	}
	applied := []model.Candidate{
		{
			EntryID: "entry-b",
			Entry:   core.Entry{ID: "entry-b", Type: "updated", Meta: core.EntryMeta{Revision: 1}},
		},
		{
			EntryID: "entry-c",
			Entry:   core.Entry{ID: "entry-c", Type: "added", Meta: core.EntryMeta{Revision: 1}},
		},
	}

	rewritten, count := newTestService().applySubstitutions(entries, applied)

	require.Len(t, rewritten, 3)
	assert.Equal(t, 2, count)
	assert.Equal(t, core.Entry{ID: "entry-a", Type: "note", Meta: core.EntryMeta{Revision: 1}}, rewritten[0])
	assert.Equal(t, core.Entry{ID: "entry-b", Type: "updated", Meta: core.EntryMeta{Revision: 2}}, rewritten[1])
	assert.Equal(t, core.Entry{ID: "entry-c", Type: "added", Meta: core.EntryMeta{Revision: 2}}, rewritten[2])
}

func TestApplySubstitutions_ReapplyIsIdempotent(t *testing.T) {
	t.Parallel()

	entries := []core.Entry{
		{ID: "entry-a", Type: "note", Meta: core.EntryMeta{Revision: 1}},
		{ID: "entry-b", Type: "updated", Meta: core.EntryMeta{Revision: 2}},
		{ID: "entry-c", Type: "added", Meta: core.EntryMeta{Revision: 2}},
	}
	applied := []model.Candidate{
		{
			EntryID: "entry-b",
			Entry:   core.Entry{ID: "entry-b", Type: "updated", Meta: core.EntryMeta{Revision: 1}},
		},
		{
			EntryID: "entry-c",
			Entry:   core.Entry{ID: "entry-c", Type: "added", Meta: core.EntryMeta{Revision: 1}},
		},
	}

	var logs bytes.Buffer
	service := &Service{logger: slog.New(slog.NewJSONHandler(&logs, nil))}
	rewritten, count := service.applySubstitutions(entries, applied)

	require.Len(t, rewritten, 3)
	assert.Zero(t, count)
	assert.Equal(t, entries, rewritten)
	assert.Contains(t, logs.String(), `"msg":"skipping candidate with mismatched entry revision"`)
}

func newTestService() *Service {
	return &Service{logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
}
