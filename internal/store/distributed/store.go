package distributed

import (
	"distributed-kv/internal/raftpebble"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/hashicorp/raft"
)

const (
	retainSnapshotCount = 2
	raftTimeout         = 10 * time.Second
)

type Store struct {
	// RaftDir is the directory where the Stable and Logs data is stored.
	RaftDir string
	// RaftBind is the address to bind the Raft server.
	RaftBind string

	fsm  *FSM
	raft *raft.Raft
}

func NewStore(raftDir, raftBind string, storer Storer) *Store {
	return &Store{
		RaftDir:  raftDir,
		RaftBind: raftBind,
		fsm:      NewFSM(storer),
	}
}

func (s *Store) Open(localID string, bootstrap bool) error {
	// Setup Raft configuration.
	config := raft.DefaultConfig()
	config.LocalID = raft.ServerID(localID)

	// Setup Raft communication.
	addr, err := net.ResolveTCPAddr("tcp", s.RaftBind)
	if err != nil {
		return err
	}
	// Create the snapshot store. This allows the Raft to truncate the log.
	fss, err := raft.NewFileSnapshotStore(s.RaftDir, retainSnapshotCount, os.Stderr)
	if err != nil {
		return fmt.Errorf("file snapshot store: %s", err)
	}

	// Create the log store and stable store.
	ldb, err := raftpebble.New(raftpebble.WithDbDirPath(filepath.Join(s.RaftDir, "logs.dat")))
	if err != nil {
		return fmt.Errorf("new pebble: %s", err)
	}
	sdb, err := raftpebble.New(raftpebble.WithDbDirPath(filepath.Join(s.RaftDir, "stable.dat")))
	if err != nil {
		return fmt.Errorf("new pebble: %s", err)
	}

	// Instantiate the transport.
	transport, err := raft.NewTCPTransport(s.RaftBind, addr, 3, 10*time.Second, os.Stderr)
	if err != nil {
		return err
	}

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
