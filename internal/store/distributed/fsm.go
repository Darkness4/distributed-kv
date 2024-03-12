package distributed

import (
	dkvv1 "distributed-kv/gen/dkv/v1"
	"encoding/csv"
	"errors"
	"io"

	"github.com/hashicorp/raft"
	"google.golang.org/protobuf/proto"
)

var _ raft.FSM = (*FSM)(nil)

type Storer interface {
	Delete(key string) error
	Set(key, value string) error
	Dump() map[string]string
	Clear()
}

type FSM struct {
	storer Storer
}

func NewFSM(storer Storer) *FSM {
	return &FSM{storer: storer}
}

// Apply execute the command from the Raft log entry.
func (f *FSM) Apply(l *raft.Log) interface{} {
	// Unpack the data
	var cmd dkvv1.Command
	if err := proto.Unmarshal(l.Data, &cmd); err != nil {
		return err
	}

	// Apply the command
	switch c := cmd.Command.(type) {
	case *dkvv1.Command_Set:
		return f.storer.Set(c.Set.Key, c.Set.Value)
	case *dkvv1.Command_Delete:
		return f.storer.Delete(c.Delete.Key)
	}

	return errors.New("unknown command")
}

// Restore restores the state of the FSM from a snapshot.
func (f *FSM) Restore(snapshot io.ReadCloser) error {
	f.storer.Clear()
	r := csv.NewReader(snapshot)
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if err := f.storer.Set(record[0], record[1]); err != nil {
			return err
		}
	}
	return nil
}

// Snapshot dumps the state of the FSM to a snapshot.
//
// nolint: ireturn
func (f *FSM) Snapshot() (raft.FSMSnapshot, error) {
	return &fsmSnapshot{store: f.storer.Dump()}, nil
}

var _ raft.FSMSnapshot = (*fsmSnapshot)(nil)

type fsmSnapshot struct {
	store map[string]string
}

// Persist should dump all necessary state to the WriteCloser 'sink',
// and call sink.Close() when finished or call sink.Cancel() on error.
func (f *fsmSnapshot) Persist(sink raft.SnapshotSink) error {
	err := func() error {
		csvWriter := csv.NewWriter(sink)
		for k, v := range f.store {
			if err := csvWriter.Write([]string{k, v}); err != nil {
				return err
			}
		}
		csvWriter.Flush()
		if err := csvWriter.Error(); err != nil {
			return err
		}
		return sink.Close()
	}()

	if err != nil {
		if err = sink.Cancel(); err != nil {
			panic(err)
		}
	}

	return err
}

// Release is invoked when we are finished with the snapshot.
func (f *fsmSnapshot) Release() {}
