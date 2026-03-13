package flock

import (
	"fmt"
	"os"
	"syscall"
)

// Lock holds an exclusive flock on a file.
// The lock is automatically released by the OS when the process exits,
// including crashes and SIGKILL — no stale lock cleanup required.
type Lock struct {
	fd *os.File
}

// Acquire opens (or creates) the file at path and takes an exclusive
// non-blocking flock. Returns an error if another process already holds
// the lock.
func Acquire(path string) (*Lock, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, fmt.Errorf("flock: open %s: %w", path, err)
	}

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("flock: another server is already running (lock: %s)", path)
	}

	return &Lock{fd: f}, nil
}

// Release unlocks and closes the lock file.
func (l *Lock) Release() error {
	// Unlock explicitly, then close. Close alone releases the flock,
	// but being explicit costs nothing and is clearer.
	_ = syscall.Flock(int(l.fd.Fd()), syscall.LOCK_UN)
	return l.fd.Close()
}
