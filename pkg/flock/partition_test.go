package flock

import (
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPartitionLock_SharedCoexist(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "test.lock")
	pl := NewPartitionLock(path)

	g1, err := pl.SharedLock()
	require.NoError(t, err)

	g2, err := pl.SharedLock()
	require.NoError(t, err)

	assert.NoError(t, g1.Release())
	assert.NoError(t, g2.Release())
}

func TestPartitionLock_ExclusiveBlocksShared(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "test.lock")
	pl := NewPartitionLock(path)

	// Take exclusive lock
	exGuard, err := pl.ExclusiveLock()
	require.NoError(t, err)

	// Shared lock should block, then succeed after release
	done := make(chan struct{})
	go func() {
		g, err := pl.SharedLock()
		assert.NoError(t, err)
		_ = g.Release()
		close(done)
	}()

	// Give goroutine time to block
	time.Sleep(50 * time.Millisecond)
	select {
	case <-done:
		t.Fatal("shared lock should have blocked while exclusive is held")
	default:
	}

	require.NoError(t, exGuard.Release())

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("shared lock did not unblock after exclusive release")
	}
}

func TestPartitionLock_SharedBlocksExclusive(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "test.lock")
	pl := NewPartitionLock(path)

	shGuard, err := pl.SharedLock()
	require.NoError(t, err)

	done := make(chan struct{})
	go func() {
		g, err := pl.ExclusiveLock()
		assert.NoError(t, err)
		_ = g.Release()
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)
	select {
	case <-done:
		t.Fatal("exclusive lock should have blocked while shared is held")
	default:
	}

	require.NoError(t, shGuard.Release())

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("exclusive lock did not unblock after shared release")
	}
}

func TestPartitionLock_TryExclusiveLock_Fails(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "test.lock")
	pl := NewPartitionLock(path)

	g, err := pl.SharedLock()
	require.NoError(t, err)
	defer func() { _ = g.Release() }()

	_, err = pl.TryExclusiveLock()
	assert.ErrorIs(t, err, ErrLocked)
}

func TestPartitionLock_TryExclusiveLock_Succeeds(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "test.lock")
	pl := NewPartitionLock(path)

	g, err := pl.TryExclusiveLock()
	require.NoError(t, err)
	assert.NoError(t, g.Release())
}

func TestPartitionLock_ConcurrentAppendSimulation(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "test.lock")
	pl := NewPartitionLock(path)

	// Simulate N concurrent appenders holding shared locks
	const n = 20
	var wg sync.WaitGroup
	wg.Add(n)
	for range n {
		go func() {
			defer wg.Done()
			g, err := pl.SharedLock()
			assert.NoError(t, err)
			time.Sleep(10 * time.Millisecond) // simulate write
			assert.NoError(t, g.Release())
		}()
	}
	wg.Wait()
}

func TestPartitionLock_CompactionBlocksAppends(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "test.lock")
	pl := NewPartitionLock(path)

	// Take exclusive lock (compactor)
	exGuard, err := pl.ExclusiveLock()
	require.NoError(t, err)

	// Try multiple shared locks — all should block
	const n = 5
	blocked := make(chan int, n)
	for i := range n {
		go func(id int) {
			g, _ := pl.SharedLock()
			blocked <- id
			_ = g.Release()
		}(i)
	}

	time.Sleep(50 * time.Millisecond)
	assert.Empty(t, blocked, "no shared locks should have succeeded")

	// Release exclusive — all should unblock
	require.NoError(t, exGuard.Release())

	for range n {
		select {
		case <-blocked:
		case <-time.After(2 * time.Second):
			t.Fatal("not all shared locks unblocked after exclusive release")
		}
	}
}

func TestGuard_ReleaseIdempotent(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "test.lock")
	pl := NewPartitionLock(path)

	g, err := pl.ExclusiveLock()
	require.NoError(t, err)

	assert.NoError(t, g.Release())
	assert.NoError(t, g.Release()) // second release should be safe
}

func TestGuard_NilRelease(t *testing.T) {
	t.Parallel()
	var g *Guard
	assert.NoError(t, g.Release())
}
