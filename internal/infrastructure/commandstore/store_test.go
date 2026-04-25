package commandstore

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gitrus/digikeeper-log/internal/domain/core"
	"github.com/gitrus/digikeeper-log/internal/infrastructure/index"
)

func TestStoreExclusiveLockRespectsContext(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	idx, err := index.New(filepath.Join(dir, "index.db"))
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
