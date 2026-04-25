package flock

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAcquire(t *testing.T) {
	t.Parallel()

	t.Run("creates lock file", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "test.lock")

		lock, err := Acquire(path)
		require.NoError(t, err)
		t.Cleanup(func() { _ = lock.Release() })

		assert.FileExists(t, path)
	})

	t.Run("release succeeds", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "test.lock")

		lock, err := Acquire(path)
		require.NoError(t, err)

		assert.NoError(t, lock.Release())
	})

	t.Run("double acquire fails", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "test.lock")

		lock1, err := Acquire(path)
		require.NoError(t, err)
		t.Cleanup(func() { _ = lock1.Release() })

		_, err = Acquire(path)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrLocked)
	})

	t.Run("reacquire after release", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "test.lock")

		lock1, err := Acquire(path)
		require.NoError(t, err)
		require.NoError(t, lock1.Release())

		lock2, err := Acquire(path)
		require.NoError(t, err)
		t.Cleanup(func() { _ = lock2.Release() })
	})
}

func TestAcquire_Errors(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		path    string
		wantMsg string
	}{
		"nonexistent directory": {
			path:    "/nonexistent/dir/test.lock",
			wantMsg: "flock: open",
		},
		"empty path": {
			path:    "",
			wantMsg: "flock: open",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := Acquire(tt.path)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantMsg)
		})
	}
}
