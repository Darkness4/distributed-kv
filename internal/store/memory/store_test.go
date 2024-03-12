package memory_test

import (
	"testing"

	"distributed-kv/internal/store/memory"

	"github.com/stretchr/testify/require"
)

func TestStore(t *testing.T) {
	t.Parallel()
	s := memory.New()

	// Test Get
	t.Run("Get", func(t *testing.T) {
		_, err := s.Get("key")
		require.ErrorIs(t, err, memory.ErrNotFound)
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
		require.ErrorIs(t, err, memory.ErrNotFound)
	})
}
