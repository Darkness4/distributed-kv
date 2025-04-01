package distributed

import (
	"crypto/tls"
	dkvv1 "distributed-kv/gen/dkv/v1"
	"distributed-kv/internal/raftpebble"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/hashicorp/raft"
	"google.golang.org/protobuf/proto"
)

const (
	retainSnapshotCount = 2
)

type Store struct {
	// RaftDir is the directory where the Stable and Logs data is stored.
	RaftDir string
	// RaftBind is the address to bind the Raft server.
	RaftBind string
	// RaftID is the ID of the local node.
	RaftID string
	// RaftAdvertisedAddr is the address other nodes should use to communicate with this node.
	RaftAdvertisedAddr raft.ServerAddress

	fsm  *FSM
	raft *raft.Raft

	shutdownCh chan struct{}

	StoreOptions
}

type StoreOptions struct {
	serverTLSConfig *tls.Config
	clientTLSConfig *tls.Config
	raftConfig      *raft.Config
}

type StoreOption func(*StoreOptions)

func WithServerTLSConfig(config *tls.Config) StoreOption {
	return func(o *StoreOptions) {
		o.serverTLSConfig = config
	}
}

func WithClientTLSConfig(config *tls.Config) StoreOption {
	return func(o *StoreOptions) {
		o.clientTLSConfig = config
	}
}

func WithRaftConfig(config *raft.Config) StoreOption {
	return func(o *StoreOptions) {
		o.raftConfig = config
	}
}

func applyStoreOptions(opts []StoreOption) StoreOptions {
	options := StoreOptions{
		raftConfig: raft.DefaultConfig(),
	}
	for _, o := range opts {
		o(&options)
	}
	return options
}

func NewStore(
	raftDir, raftBind, raftID string,
	raftAdvertisedAddr raft.ServerAddress,
	storer Storer,
	opts ...StoreOption,
) *Store {
	o := applyStoreOptions(opts)
	return &Store{
		RaftDir:            raftDir,
		RaftBind:           raftBind,
		RaftID:             raftID,
		RaftAdvertisedAddr: raftAdvertisedAddr,
		fsm:                NewFSM(storer),
		shutdownCh:         make(chan struct{}),
		StoreOptions:       o,
	}
}

func (s *Store) Open(bootstrap bool) error {
	// Setup Raft configuration.
	config := s.raftConfig
	config.LocalID = raft.ServerID(s.RaftID)

	// Create the snapshot store. This allows the Raft to truncate the log.
	fss, err := raft.NewFileSnapshotStore(s.RaftDir, retainSnapshotCount, os.Stderr)
	if err != nil {
		return fmt.Errorf("file snapshot store: %s", err)
	}

	// Create the log store and stable store.
	ldb, err := raftpebble.New(raftpebble.WithDBDirPath(filepath.Join(s.RaftDir, "logs.dat")))
	if err != nil {
		return fmt.Errorf("new pebble: %s", err)
	}
	sdb, err := raftpebble.New(raftpebble.WithDBDirPath(filepath.Join(s.RaftDir, "stable.dat")))
	if err != nil {
		return fmt.Errorf("new pebble: %s", err)
	}

	// Instantiate the transport.
	lis, err := net.Listen("tcp", s.RaftBind)
	if err != nil {
		return err
	}
	transport := raft.NewNetworkTransport(&TLSStreamLayer{
		Listener:          lis,
		AdvertizedAddress: raft.ServerAddress(s.RaftAdvertisedAddr),
		ServerTLSConfig:   s.serverTLSConfig,
		ClientTLSConfig:   s.clientTLSConfig,
	}, 3, 10*time.Second, os.Stderr)

	// Instantiate the Raft systems.
	ra, err := raft.NewRaft(config, s.fsm, ldb, sdb, fss, transport)
	if err != nil {
		return fmt.Errorf("new raft: %s", err)
	}
	s.raft = ra

	// Check if there is an existing state, if not bootstrap.
	hasState, err := raft.HasExistingState(
		ldb,
		sdb,
		fss,
	)
	if err != nil {
		return err
	}
	if bootstrap && !hasState {
		slog.Info(
			"bootstrapping new raft node",
			"id",
			config.LocalID,
			"addr",
			transport.LocalAddr(),
		)
		config := raft.Configuration{
			Servers: []raft.Server{
				{
					ID:      config.LocalID,
					Address: transport.LocalAddr(),
				},
			},
		}
		err = s.raft.BootstrapCluster(config).Error()
	}
	return err
}

func (s *Store) Join(id raft.ServerID, addr raft.ServerAddress) error {
	slog.Info("request node to join", "id", id, "addr", addr)

	configFuture := s.raft.GetConfiguration()
	if err := configFuture.Error(); err != nil {
		slog.Error("failed to get raft configuration", "error", err)
		return err
	}
	// Check if the server has already joined
	for _, srv := range configFuture.Configuration().Servers {
		// If a node already exists with either the joining node's ID or address,
		// that node may need to be removed from the config first.
		if srv.ID == id || srv.Address == addr {
			// However if *both* the ID and the address are the same, then nothing -- not even
			// a join operation -- is needed.
			if srv.Address == addr && srv.ID == id {
				slog.Info(
					"node already member of cluster, ignoring join request",
					"id",
					id,
					"addr",
					addr,
				)
				return nil
			}

			if err := s.raft.RemoveServer(id, 0, 0).Error(); err != nil {
				return fmt.Errorf("error removing existing node %s at %s: %s", id, addr, err)
			}
		}
	}

	// Add the new server
	return s.raft.AddVoter(id, addr, 0, 0).Error()
}

func (s *Store) Leave(id raft.ServerID) error {
	slog.Info("request node to leave", "id", id)
	return s.raft.RemoveServer(id, 0, 0).Error()
}

func (s *Store) WaitForLeader(timeout time.Duration) (raft.ServerID, error) {
	slog.Info("waiting for leader", "timeout", timeout)
	timeoutCh := time.After(timeout)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-s.shutdownCh:
			return "", errors.New("shutdown")
		case <-timeoutCh:
			return "", errors.New("timed out waiting for leader")
		case <-ticker.C:
			addr, id := s.raft.LeaderWithID()
			if addr != "" {
				slog.Info("leader found", "addr", addr, "id", id)
				return id, nil
			}
		}
	}
}

func (s *Store) Shutdown() error {
	slog.Warn("shutting down store")
	select {
	case s.shutdownCh <- struct{}{}:
	default:
	}

	if s.raft != nil {
		if err := s.raft.Shutdown().Error(); err != nil {
			return err
		}
		s.raft = nil
	}
	s.fsm.storer.Clear()
	return nil
}

func (s *Store) ShutdownCh() <-chan struct{} {
	return s.shutdownCh
}

func (s *Store) apply(req *dkvv1.Command) (any, error) {
	b, err := proto.Marshal(req)
	if err != nil {
		return nil, err
	}
	addr, id := s.raft.LeaderWithID()
	if addr == "" || id == "" {
		return nil, errors.New("no leader")
	}
	timeout := 10 * time.Second

	if id != raft.ServerID(s.RaftID) {
		slog.Warn("forwarding apply to leader", "leader", id, "addr", addr)
		return nil, s.raft.ForwardApply(id, addr, b, timeout)
	}

	future := s.raft.Apply(b, timeout)
	if err := future.Error(); err != nil {
		return nil, err
	}
	res := future.Response()
	if err, ok := res.(error); ok {
		return nil, err
	}
	return res, nil
}

func (s *Store) Set(key string, value string) error {
	_, err := s.apply(&dkvv1.Command{
		Command: &dkvv1.Command_Set{
			Set: &dkvv1.SetRequest{
				Key:   key,
				Value: value,
			},
		},
	})
	return err
}

func (s *Store) Delete(key string) error {
	_, err := s.apply(&dkvv1.Command{
		Command: &dkvv1.Command_Delete{
			Delete: &dkvv1.DeleteRequest{
				Key: key,
			},
		},
	})
	return err
}

func (s *Store) Get(key string) (string, error) {
	return s.fsm.storer.Get(key)
}

func (s *Store) GetLeader() (raft.ServerAddress, raft.ServerID) {
	return s.raft.LeaderWithID()
}

func (s *Store) GetServers() ([]raft.Server, error) {
	configFuture := s.raft.GetConfiguration()
	if err := configFuture.Error(); err != nil {
		return nil, err
	}
	return configFuture.Configuration().Servers, nil
}
