package flock

import (
	"fmt"
	"os"
	"syscall"
)

// PartitionLock provides shared/exclusive advisory file locking on a sidecar
// .lock file. Each lock acquisition opens a fresh file descriptor, which gives
// proper conflict semantics between goroutines (and processes) via flock(2).
//
// This works because flock locks are associated with the kernel's open file
// description, not the process. Two separate open() calls create independent
// descriptions with independent lock state. This behavior is consistent across
// Linux, macOS, FreeBSD, OpenBSD, NetBSD, and illumos.
//
// Use SharedLock for concurrent readers/appenders and ExclusiveLock for
// exclusive operations like compaction. The OS automatically releases all
// locks when the process exits (including crashes and SIGKILL).
type PartitionLock struct {
	path string
}

// Guard represents a held flock. Call Release to unlock and close the fd.
type Guard struct {
	fd *os.File
}

// NewPartitionLock returns a lock bound to the given sidecar file path.
// The file is created on first lock acquisition, not here.
func NewPartitionLock(path string) *PartitionLock {
	return &PartitionLock{path: path}
}

// SharedLock acquires a shared (LOCK_SH) flock, blocking until available.
// Multiple shared locks can coexist; an exclusive lock blocks until all
// shared holders release.
func (pl *PartitionLock) SharedLock() (*Guard, error) {
	return pl.acquire(syscall.LOCK_SH)
}

// ExclusiveLock acquires an exclusive (LOCK_EX) flock, blocking until
// all existing shared and exclusive holders release.
func (pl *PartitionLock) ExclusiveLock() (*Guard, error) {
	return pl.acquire(syscall.LOCK_EX)
}

// TryExclusiveLock attempts a non-blocking exclusive lock.
// Returns (nil, ErrLocked) if the lock is already held.
func (pl *PartitionLock) TryExclusiveLock() (*Guard, error) {
	return pl.acquire(syscall.LOCK_EX | syscall.LOCK_NB)
}

// ErrLocked is returned by TryExclusiveLock when the lock is already held.
var ErrLocked = fmt.Errorf("flock: lock is held by another fd")

func (pl *PartitionLock) acquire(how int) (*Guard, error) {
	f, err := os.OpenFile(pl.path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, fmt.Errorf("flock: open %s: %w", pl.path, err)
	}

	if err := syscall.Flock(int(f.Fd()), how); err != nil {
		_ = f.Close()
		if how&syscall.LOCK_NB != 0 {
			return nil, ErrLocked
		}
		return nil, fmt.Errorf("flock: lock %s: %w", pl.path, err)
	}

	return &Guard{fd: f}, nil
}

// Release unlocks and closes the underlying file descriptor.
// Safe to call multiple times.
func (g *Guard) Release() error {
	if g == nil || g.fd == nil {
		return nil
	}
	_ = syscall.Flock(int(g.fd.Fd()), syscall.LOCK_UN)
	err := g.fd.Close()
	g.fd = nil
	return err
}
