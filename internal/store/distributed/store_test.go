package distributed_test

import (
	"distributed-kv/internal/store/distributed"
	"distributed-kv/internal/store/memory"
	internaltls "distributed-kv/internal/tls"
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/raft"
	"github.com/stretchr/testify/require"
)

const (
	caCert   = "certs/ca/tls.test.crt"
	peerCert = "certs/peer/tls.test.crt"
	peerKey  = "certs/peer/tls.test.key"
)

func getRandomAddress(t *testing.T) string {
	t.Helper()

	l, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	defer l.Close()

	_, port, err := net.SplitHostPort(l.Addr().String())
	require.NoError(t, err)
	return net.JoinHostPort("localhost", port)
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
		addr := getRandomAddress(t)
		store := memory.New()
		s := distributed.NewStore(tmp, addr, "node1", raft.ServerAddress(addr), store)
		t.Cleanup(func() {
			err = s.Shutdown()
			require.NoError(t, err)
		})

		// Act
		err = s.Open(true)

		// Assert
		require.NoError(t, err)

		id, err := s.WaitForLeader(5 * time.Second)
		require.NoError(t, err)
		require.Equal(t, raft.ServerID("node1"), id)
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
			addr := getRandomAddress(t)
			store := memory.New()
			peerServerTLSConfig, err := internaltls.SetupServerTLSConfig(peerCert, peerKey, caCert)
			require.NoError(t, err)
			peerClientTLSConfig, err := internaltls.SetupClientTLSConfig(
				peerCert,
				peerKey,
				caCert,
			)
			require.NoError(t, err)
			s := distributed.NewStore(
				tmp,
				addr,
				fmt.Sprintf("node%d", i),
				raft.ServerAddress(addr),
				store,
				distributed.WithClientTLSConfig(peerClientTLSConfig),
				distributed.WithServerTLSConfig(peerServerTLSConfig),
			)
			t.Cleanup(func() {
				err = s.Shutdown()
				require.NoError(t, err)
			})
			stores[i] = s
		}

		t.Run("Join and Bootstrap", func(t *testing.T) {
			for i := 0; i < nodes; i++ {
				s := stores[i]
				err := s.Open(i == 0)
				require.NoError(t, err)

				// Act & assert
				if i > 0 {
					err = stores[0].Join(
						raft.ServerID(fmt.Sprintf("node%d", i)),
						raft.ServerAddress(s.RaftBind),
					)
					require.NoError(t, err)
				} else {
					id, err := s.WaitForLeader(5 * time.Second)
					require.NoError(t, err)
					require.Equal(t, raft.ServerID("node0"), id)
				}
			}
		})

		t.Run("Set and Get", func(t *testing.T) {
			t.Run("Set a key", func(t *testing.T) {
				// Act: Set a key
				err := stores[0].Set("key1", "value1")
				require.NoError(t, err)

				// Assert: Get the key from all nodes
				require.Eventually(t, func() bool {
					for i := 0; i < nodes; i++ {
						got, err := stores[i].Get("key1")
						if err != nil {
							return false
						}
						if got != "value1" {
							return false
						}
					}
					return true
				}, 500*time.Millisecond, 50*time.Millisecond)
			})

			// Act: Set key as non-leader
			t.Run("Set a key as non-leader", func(t *testing.T) {
				err := stores[1].Set("key2", "value")
				require.NoError(t, err)

				time.Sleep(50 * time.Millisecond)

				// Assert: Get the key from all nodes
				require.Eventually(t, func() bool {
					for i := 0; i < nodes; i++ {
						got, err := stores[i].Get("key2")
						if err != nil {
							return false
						}
						if got != "value" {
							return false
						}
					}
					return true
				}, 10*time.Second, 50*time.Millisecond)
			})

			t.Run("Set a key with node1 kicked out", func(t *testing.T) {
				// Act
				err := stores[0].Leave("node1")
				require.NoError(t, err)

				time.Sleep(50 * time.Millisecond)

				err = stores[0].Set("key1", "value2")
				require.NoError(t, err)

				// Assert
				require.Eventually(t, func() bool {
					for i := 0; i < nodes; i++ {
						if i == 1 {
							got, err := stores[i].Get("key1")
							if err != nil {
								return false
							}
							if got != "value1" {
								return false
							}
						} else {
							got, err := stores[i].Get("key1")
							if err != nil {
								return false
							}
							if got != "value2" {
								return false
							}
						}
					}
					return true
				}, 500*time.Millisecond, 50*time.Millisecond)
			})

			// Act: Set the key again, but with node1 back in (convergence test)
			t.Run("Set a key with node1 back in", func(t *testing.T) {
				err := stores[0].Join("node1", raft.ServerAddress(stores[1].RaftBind))
				require.NoError(t, err)

				time.Sleep(50 * time.Millisecond)

				got, err := stores[1].Get("key1")
				require.NoError(t, err)
				require.Equal(t, "value2", got)
			})

			// Act: Set the key again, but with node0 shutdown (fault tolerence test)
			t.Run("Set a key with node0 shutdown", func(t *testing.T) {
				err := stores[0].Shutdown()
				require.NoError(t, err)

				var nextLeader string
				require.Eventually(t, func() bool {
					id, err := stores[1].WaitForLeader(15 * time.Second)
					require.NoError(t, err)

					nextLeader = string(id)

					return id != "node0"
				}, 15*time.Second, 1*time.Second)

				var leader *distributed.Store
				for _, s := range stores {
					if s.RaftID == nextLeader {
						leader = s
					}
				}

				err = leader.Set("key1", "value3")
				require.NoError(t, err)

				require.Eventually(t, func() bool {
					for i := 1; i < nodes; i++ {
						got, err := stores[i].Get("key1")
						if err != nil {
							return false
						}
						if got != "value3" {
							return false
						}
					}
					return true
				}, 500*time.Millisecond, 50*time.Millisecond)
			})
		})
	})
}
