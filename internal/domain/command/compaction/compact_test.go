package compaction

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gitrus/digikeeper-log/internal/domain/command/model"
	"github.com/gitrus/digikeeper-log/internal/domain/core"
)

func TestApplySubstitutions_ReplacesAndAppendsMissingCandidates(t *testing.T) {
	t.Parallel()

	entries := []core.Entry{
		{ID: "entry-a", Type: "note"},
		{ID: "entry-b", Type: "note"},
	}
	applied := []model.Candidate{
		{
			EntryID: "entry-b",
			Entry:   core.Entry{ID: "entry-b", Type: "updated"},
		},
		{
			EntryID: "entry-c",
			Entry:   core.Entry{ID: "entry-c", Type: "added"},
		},
	}

	rewritten, count := applySubstitutions(entries, applied)

	require.Len(t, rewritten, 3)
	assert.Equal(t, 2, count)
	assert.Equal(t, core.Entry{ID: "entry-a", Type: "note"}, rewritten[0])
	assert.Equal(t, core.Entry{ID: "entry-b", Type: "updated"}, rewritten[1])
	assert.Equal(t, core.Entry{ID: "entry-c", Type: "added"}, rewritten[2])
}

func TestApplySubstitutions_ReapplyIsIdempotent(t *testing.T) {
	t.Parallel()

	entries := []core.Entry{
		{ID: "entry-a", Type: "note"},
		{ID: "entry-b", Type: "updated"},
		{ID: "entry-c", Type: "added"},
	}
	applied := []model.Candidate{
		{
			EntryID: "entry-b",
			Entry:   core.Entry{ID: "entry-b", Type: "updated"},
		},
		{
			EntryID: "entry-c",
			Entry:   core.Entry{ID: "entry-c", Type: "added"},
		},
	}

	rewritten, count := applySubstitutions(entries, applied)

	require.Len(t, rewritten, 3)
	assert.Equal(t, 2, count)
	assert.Equal(t, entries, rewritten)
}
