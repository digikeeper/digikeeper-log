package flock

import (
	"path/filepath"
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

	started, done := startLockAttempt(l.SharedLock)
	requireLockAttemptStarted(t, started)
	assertLockStillWaiting(t, done)

	require.NoError(t, exGuard.Release())

	g := requireLockDone(t, done)
	assert.NoError(t, g.Release())
}

func TestRWLock_SharedBlocksExclusive(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "test.lock")
	l := NewRWLock(path)

	shGuard, err := l.SharedLock()
	require.NoError(t, err)

	started, done := startLockAttempt(l.ExclusiveLock)
	requireLockAttemptStarted(t, started)
	assertLockStillWaiting(t, done)

	require.NoError(t, shGuard.Release())

	g := requireLockDone(t, done)
	assert.NoError(t, g.Release())
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
	errs := make(chan error, n)
	for range n {
		go func() {
			g, err := l.SharedLock()
			if err != nil {
				errs <- err
				return
			}
			time.Sleep(10 * time.Millisecond)
			errs <- g.Release()
		}()
	}

	for range n {
		require.NoError(t, <-errs)
	}
}

func TestRWLock_ExclusiveBlocksMultipleShared(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "test.lock")
	l := NewRWLock(path)

	exGuard, err := l.ExclusiveLock()
	require.NoError(t, err)

	const n = 5
	type lockAttempt struct {
		started <-chan struct{}
		done    <-chan error
	}
	attempts := make([]lockAttempt, 0, n)
	for range n {
		started, done := startSharedLockAndRelease(l)
		attempts = append(attempts, lockAttempt{started: started, done: done})
	}

	for _, attempt := range attempts {
		requireLockAttemptStarted(t, attempt.started)
		assertLockReleaseStillWaiting(t, attempt.done)
	}

	require.NoError(t, exGuard.Release())

	for _, attempt := range attempts {
		requireLockReleaseDone(t, attempt.done)
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

type lockAttemptResult struct {
	guard *Guard
	err   error
}

func startLockAttempt(acquire func() (*Guard, error)) (<-chan struct{}, <-chan lockAttemptResult) {
	started := make(chan struct{})
	done := make(chan lockAttemptResult, 1)

	go func() {
		close(started)
		g, err := acquire()
		done <- lockAttemptResult{guard: g, err: err}
	}()

	return started, done
}

func startSharedLockAndRelease(l *RWLock) (<-chan struct{}, <-chan error) {
	started := make(chan struct{})
	done := make(chan error, 1)

	go func() {
		close(started)
		g, err := l.SharedLock()
		if err != nil {
			done <- err
			return
		}
		done <- g.Release()
	}()

	return started, done
}

func requireLockAttemptStarted(t *testing.T, started <-chan struct{}) {
	t.Helper()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("lock attempt did not start")
	}
}

func assertLockStillWaiting(t *testing.T, done <-chan lockAttemptResult) {
	t.Helper()

	select {
	case result := <-done:
		if result.guard != nil {
			_ = result.guard.Release()
		}
		t.Fatalf("lock attempt completed before blocker was released: %v", result.err)
	case <-time.After(50 * time.Millisecond):
	}
}

func assertLockReleaseStillWaiting(t *testing.T, done <-chan error) {
	t.Helper()

	select {
	case err := <-done:
		t.Fatalf("lock attempt completed before blocker was released: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
}

func requireLockReleaseDone(t *testing.T, done <-chan error) {
	t.Helper()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("lock attempt did not complete")
	}
}

func requireLockDone(t *testing.T, done <-chan lockAttemptResult) *Guard {
	t.Helper()

	select {
	case result := <-done:
		require.NoError(t, result.err)
		require.NotNil(t, result.guard)
		return result.guard
	case <-time.After(2 * time.Second):
		t.Fatal("lock attempt did not complete")
		return nil
	}
}
