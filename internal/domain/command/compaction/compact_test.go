package compaction

import (
	"bytes"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/digikeeper/digikeeper-journal/internal/domain/command/model"
	"github.com/digikeeper/digikeeper-journal/internal/domain/core"
)

func TestApplySubstitutions_ReplacesAndAppendsMissingCandidates(t *testing.T) {
	t.Parallel()

	records := []core.Record{
		{ID: "record-a", Type: "note", Meta: core.RecordMeta{Revision: 1}},
		{ID: "record-b", Type: "note", Meta: core.RecordMeta{Revision: 1}},
	}
	applied := []model.Candidate{
		{
			RecordID: "record-b",
			Record:   core.Record{ID: "record-b", Type: "updated", Meta: core.RecordMeta{Revision: 1}},
		},
		{
			RecordID: "record-c",
			Record:   core.Record{ID: "record-c", Type: "added", Meta: core.RecordMeta{Revision: 1}},
		},
	}

	rewritten, count := newTestService().applySubstitutions(records, applied)

	require.Len(t, rewritten, 3)
	assert.Equal(t, 2, count)
	assert.Equal(t, core.Record{ID: "record-a", Type: "note", Meta: core.RecordMeta{Revision: 1}}, rewritten[0])
	assert.Equal(t, core.Record{ID: "record-b", Type: "updated", Meta: core.RecordMeta{Revision: 2}}, rewritten[1])
	assert.Equal(t, core.Record{ID: "record-c", Type: "added", Meta: core.RecordMeta{Revision: 2}}, rewritten[2])
}

func TestApplySubstitutions_ReapplyIsIdempotent(t *testing.T) {
	t.Parallel()

	records := []core.Record{
		{ID: "record-a", Type: "note", Meta: core.RecordMeta{Revision: 1}},
		{ID: "record-b", Type: "updated", Meta: core.RecordMeta{Revision: 2}},
		{ID: "record-c", Type: "added", Meta: core.RecordMeta{Revision: 2}},
	}
	applied := []model.Candidate{
		{
			RecordID: "record-b",
			Record:   core.Record{ID: "record-b", Type: "updated", Meta: core.RecordMeta{Revision: 1}},
		},
		{
			RecordID: "record-c",
			Record:   core.Record{ID: "record-c", Type: "added", Meta: core.RecordMeta{Revision: 1}},
		},
	}

	var logs bytes.Buffer
	service := &Service{logger: slog.New(slog.NewJSONHandler(&logs, nil))}
	rewritten, count := service.applySubstitutions(records, applied)

	require.Len(t, rewritten, 3)
	assert.Zero(t, count)
	assert.Equal(t, records, rewritten)
	assert.Contains(t, logs.String(), `"msg":"skipping candidate with mismatched record revision"`)
}

func newTestService() *Service {
	return &Service{logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
}
