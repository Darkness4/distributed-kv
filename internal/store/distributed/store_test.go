package distributed_test

import (
	"distributed-kv/internal/store/distributed"
	"distributed-kv/internal/store/memory"
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func getRandomPort(t *testing.T) string {
	t.Helper()

	l, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	defer l.Close()

	return l.Addr().String()
}

func TestStore(t *testing.T) {
	t.Parallel()

	t.Run("Open", func(t *testing.T) {
		// Arrange
		tmp, err := os.MkdirTemp("", "raft-test")
		require.NoError(t, err)
		t.Cleanup(func() {
			_ = os.RemoveAll(tmp)
		})
		p := getRandomPort(t)
		store := memory.New()
		s := distributed.NewStore(tmp, p, store)
		t.Cleanup(func() {
			err = s.Shutdown()
			require.NoError(t, err)
		})

		// Act
		err = s.Open("node1", true)

		// Assert
		require.NoError(t, err)
	})

	t.Run("Consensus", func(t *testing.T) {
		nodes := 3
		stores := make([]*distributed.Store, nodes)

		// Arrange
		for i := 0; i < nodes; i++ {
			tmp, err := os.MkdirTemp("", "raft-test")
			require.NoError(t, err)
			t.Cleanup(func() {
				_ = os.RemoveAll(tmp)
			})
			p := getRandomPort(t)
			store := memory.New()
			s := distributed.NewStore(tmp, p, store)
			t.Cleanup(func() {
				err = s.Shutdown()
				require.NoError(t, err)
			})
			stores[i] = s
		}

		t.Run("Join and Bootstrap", func(t *testing.T) {
			for i := 0; i < nodes; i++ {
				s := stores[i]
				err := s.Open(fmt.Sprintf("node%d", i), i == 0)
				require.NoError(t, err)

				// Act & assert
				if i > 0 {
					err = stores[0].Join(fmt.Sprintf("node%d", i), s.RaftBind)
					require.NoError(t, err)
				} else {
					err = s.WaitForLeader(5 * time.Second)
					require.NoError(t, err)
				}
			}
		})

		// t.Run("Set/Get", func(t *testing.T) {
		// 	stores[0].
		// })
	})
}
