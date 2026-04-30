package candidatestore

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	commandmodel "github.com/gitrus/digikeeper-log/internal/domain/command/model"
	"github.com/gitrus/digikeeper-log/internal/domain/core"
	"github.com/gitrus/digikeeper-log/internal/domain/errs"
)

func TestStoreCandidateLifecycle(t *testing.T) {
	t.Parallel()

	store, err := New(t.TempDir())
	require.NoError(t, err)

	partition := testPartition(t)
	candidate := testCandidate("candidate-a", "entry-a")
	require.NoError(t, store.AppendCandidate(t.Context(), candidate))

	pending, err := store.ListPending(t.Context(), partition)
	require.NoError(t, err)
	require.Len(t, pending, 1)
	assert.Equal(t, candidate.ID, pending[0].ID)

	candidate.Action = core.Apply
	candidate.ResolvedBy = "tester"
	candidate.ResolvedAt = time.Now().UTC()
	require.NoError(t, store.MoveCandidates(t.Context(), partition, []commandmodel.Candidate{candidate}, nil))

	pending, err = store.ListPending(t.Context(), partition)
	require.NoError(t, err)
	assert.Empty(t, pending)

	applied, err := store.ListApplied(t.Context(), partition)
	require.NoError(t, err)
	require.Len(t, applied, 1)
	assert.Equal(t, candidate.ID, applied[0].ID)

	require.NoError(t, store.DeleteApplied(t.Context(), partition, []string{candidate.ID}))
	applied, err = store.ListApplied(t.Context(), partition)
	require.NoError(t, err)
	assert.Empty(t, applied)
}

func TestStoreMoveCandidatesAppliedConflict(t *testing.T) {
	t.Parallel()

	store, err := New(t.TempDir())
	require.NoError(t, err)

	partition := testPartition(t)
	first := testCandidate("candidate-a", "entry-a")
	first.Action = core.Apply
	require.NoError(t, store.MoveCandidates(t.Context(), partition, []commandmodel.Candidate{first}, nil))

	second := testCandidate("candidate-b", "entry-b")
	second.Action = core.Apply
	err = store.MoveCandidates(t.Context(), partition, []commandmodel.Candidate{second}, nil)
	require.True(t, errors.Is(err, errs.ErrConflict))
}

func TestStoreListMissingFilesReturnsEmpty(t *testing.T) {
	t.Parallel()

	store, err := New(t.TempDir())
	require.NoError(t, err)

	pending, err := store.ListPending(t.Context(), testPartition(t))
	require.NoError(t, err)
	assert.Empty(t, pending)

	applied, err := store.ListApplied(t.Context(), testPartition(t))
	require.NoError(t, err)
	assert.Empty(t, applied)
}

func TestStoreSharedLockRespectsContext(t *testing.T) {
	t.Parallel()

	store, err := New(t.TempDir())
	require.NoError(t, err)
	partition := testPartition(t)

	release, err := store.ExclusiveLock(t.Context(), partition)
	require.NoError(t, err)
	defer release()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	blockedRelease, err := store.SharedLock(ctx, partition)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Nil(t, blockedRelease)
}

func testPartition(t *testing.T) core.Partition {
	t.Helper()
	partition, err := core.ParsePartition("2026-03-08")
	require.NoError(t, err)
	return partition
}

func testCandidate(id, entryID string) commandmodel.Candidate {
	ts := time.Date(2026, 3, 8, 10, 0, 0, 0, time.UTC)
	return commandmodel.Candidate{
		ID:                id,
		EntryID:           entryID,
		OriginalTimestamp: ts,
		Entry: core.Entry{
			ID:        entryID,
			Timestamp: ts,
			Type:      "note",
			Tags:      []string{"work"},
			Data:      map[string]any{"note": id},
		},
		CreatedAt: ts,
	}
}
