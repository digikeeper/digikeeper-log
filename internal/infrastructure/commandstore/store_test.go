package commandstore

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/digikeeper/digikeeper-journal/internal/domain/core"
	"github.com/digikeeper/digikeeper-journal/internal/domain/errs"
	"github.com/digikeeper/digikeeper-journal/internal/infrastructure/index"
)

func TestStoreExclusiveLockRespectsContext(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	idx, err := index.New(filepath.Join(dir, "index.db"), index.Config{})
	require.NoError(t, err)
	t.Cleanup(func() { _ = idx.Close() })

	store, err := NewStore(dir, idx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	partition := core.PartitionFromTime(time.Date(2026, 3, 8, 10, 0, 0, 0, time.UTC))
	relPath := store.rawStore.BuildRelPath(partition)

	guard, err := store.partitionLock(relPath).SharedLock()
	require.NoError(t, err)
	t.Cleanup(func() { _ = guard.Release() })

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	release, err := store.ExclusiveLock(ctx, partition)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Nil(t, release)
}

func TestStoreReadRecord(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	idx, err := index.New(filepath.Join(dir, "index.db"), index.Config{})
	require.NoError(t, err)
	t.Cleanup(func() { _ = idx.Close() })

	store, err := NewStore(dir, idx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	partition := core.PartitionFromTime(time.Date(2026, 3, 8, 10, 0, 0, 0, time.UTC))
	want := core.Record{
		ID:        "record-a",
		Timestamp: time.Date(2026, 3, 8, 10, 0, 0, 0, time.UTC),
		Type:      "note",
		Tags:      []string{"work"},
		Data:      map[string]any{"note": "test"},
	}
	require.NoError(t, store.Append(t.Context(), want))

	got, err := store.ReadRecord(t.Context(), want.ID, partition)
	require.NoError(t, err)
	assert.Equal(t, want.ID, got.ID)
	assert.Equal(t, want.Type, got.Type)

	_, err = store.ReadRecord(t.Context(), "missing", partition)
	require.True(t, errors.Is(err, errs.ErrRecordNotFound))
}
