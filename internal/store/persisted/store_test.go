package persisted_test

import (
	"os"
	"testing"

	"distributed-kv/internal/store/persisted"

	"github.com/cockroachdb/pebble"
	"github.com/stretchr/testify/require"
)

func TestStore(t *testing.T) {
	t.Parallel()
	dir, err := os.MkdirTemp("", "persisted-test")
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = os.RemoveAll(dir)
	})
	s := persisted.New(dir)

	// Test Get
	t.Run("Get", func(t *testing.T) {
		_, err := s.Get("key")
		require.ErrorIs(t, err, pebble.ErrNotFound)
	})

	t.Run("Set", func(t *testing.T) {
		err := s.Set("key", "value")
		require.NoError(t, err)

		v, err := s.Get("key")
		require.NoError(t, err)
		require.Equal(t, "value", v)
	})

	t.Run("Delete", func(t *testing.T) {
		err := s.Set("key", "value")
		require.NoError(t, err)

		err = s.Delete("key")
		require.NoError(t, err)

		_, err = s.Get("key")
		require.ErrorIs(t, err, pebble.ErrNotFound)
	})

	t.Run("Dump", func(t *testing.T) {
		err := s.Set("key", "value")
		require.NoError(t, err)

		data := s.Dump()
		require.Equal(t, map[string]string{"key": "value"}, data)
	})

	t.Run("Clear", func(t *testing.T) {
		err := s.Set("key", "value")
		require.NoError(t, err)

		s.Clear()

		_, err = s.Get("key")
		require.ErrorIs(t, err, pebble.ErrNotFound)
	})
}
