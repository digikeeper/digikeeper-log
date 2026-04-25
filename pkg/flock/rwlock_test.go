package flock

import (
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRWLock_SharedCoexist(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "test.lock")
	l := NewRWLock(path)

	g1, err := l.SharedLock()
	require.NoError(t, err)

	g2, err := l.SharedLock()
	require.NoError(t, err)

	assert.NoError(t, g1.Release())
	assert.NoError(t, g2.Release())
}

func TestRWLock_ExclusiveBlocksShared(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "test.lock")
	l := NewRWLock(path)

	exGuard, err := l.ExclusiveLock()
	require.NoError(t, err)

	done := make(chan struct{})
	go func() {
		g, err := l.SharedLock()
		assert.NoError(t, err)
		_ = g.Release()
		close(done)
	}()

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

func TestRWLock_SharedBlocksExclusive(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "test.lock")
	l := NewRWLock(path)

	shGuard, err := l.SharedLock()
	require.NoError(t, err)

	done := make(chan struct{})
	go func() {
		g, err := l.ExclusiveLock()
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

func TestRWLock_TryExclusiveLock_Fails(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "test.lock")
	l := NewRWLock(path)

	g, err := l.SharedLock()
	require.NoError(t, err)
	defer func() { _ = g.Release() }()

	_, err = l.TryExclusiveLock()
	assert.ErrorIs(t, err, ErrLocked)
}

func TestRWLock_TryExclusiveLock_Succeeds(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "test.lock")
	l := NewRWLock(path)

	g, err := l.TryExclusiveLock()
	require.NoError(t, err)
	assert.NoError(t, g.Release())
}

func TestRWLock_ConcurrentAppendSimulation(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "test.lock")
	l := NewRWLock(path)

	const n = 20
	var wg sync.WaitGroup
	wg.Add(n)
	for range n {
		go func() {
			defer wg.Done()
			g, err := l.SharedLock()
			assert.NoError(t, err)
			time.Sleep(10 * time.Millisecond)
			assert.NoError(t, g.Release())
		}()
	}
	wg.Wait()
}

func TestRWLock_ExclusiveBlocksMultipleShared(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "test.lock")
	l := NewRWLock(path)

	exGuard, err := l.ExclusiveLock()
	require.NoError(t, err)

	const n = 5
	blocked := make(chan int, n)
	for i := range n {
		go func(id int) {
			g, _ := l.SharedLock()
			blocked <- id
			_ = g.Release()
		}(i)
	}

	time.Sleep(50 * time.Millisecond)
	assert.Empty(t, blocked, "no shared locks should have succeeded")

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
	l := NewRWLock(path)

	g, err := l.ExclusiveLock()
	require.NoError(t, err)

	assert.NoError(t, g.Release())
	assert.NoError(t, g.Release())
}

func TestGuard_NilRelease(t *testing.T) {
	t.Parallel()
	var g *Guard
	assert.NoError(t, g.Release())
}
