package distributed_test

import (
	dkvv1 "distributed-kv/gen/dkv/v1"
	"distributed-kv/internal/store/distributed"
	"distributed-kv/mocks/mockdistributed"
	"io"
	"strings"
	"testing"

	"github.com/hashicorp/raft"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func TestFSM(t *testing.T) {
	t.Parallel()

	// Arrange
	storer := mockdistributed.NewStorer(t)
	fsm := distributed.NewFSM(storer)

	t.Run("Apply", func(t *testing.T) {
		tests := []struct {
			title    string
			command  protoreflect.ProtoMessage
			expectFn func(s *mockdistributed.Storer)
			assertFn func(t *testing.T, res interface{})
		}{
			{
				title: "Set",
				command: &dkvv1.Command{
					Command: &dkvv1.Command_Set{
						Set: &dkvv1.SetRequest{
							Key:   "key",
							Value: "value",
						},
					},
				},
				expectFn: func(s *mockdistributed.Storer) {
					s.EXPECT().Set("key", "value").Return(nil)
				},
				assertFn: func(_ *testing.T, res interface{}) {
					require.Nil(t, res)
				},
			},
			{
				title: "Delete",
				command: &dkvv1.Command{
					Command: &dkvv1.Command_Delete{
						Delete: &dkvv1.DeleteRequest{
							Key: "key",
						},
					},
				},
				expectFn: func(s *mockdistributed.Storer) {
					s.EXPECT().Delete("key").Return(nil)
				},
				assertFn: func(t *testing.T, res interface{}) {
					require.Nil(t, res)
				},
			},
			{
				title:   "Invalid command",
				command: &dkvv1.Command{},
				assertFn: func(t *testing.T, res interface{}) {
					require.Error(t, res.(error))
				},
			},
		}

		for _, test := range tests {
			t.Run(test.title, func(t *testing.T) {
				// Arrange
				if test.expectFn != nil {
					test.expectFn(storer)
				}

				// Act
				data, err := proto.Marshal(test.command)
				require.NoError(t, err)
				res := fsm.Apply(&raft.Log{
					Data: data,
				})

				// Assert
				if test.assertFn != nil {
					test.assertFn(t, res)
				}
			})
		}
	})

	t.Run("Restore", func(t *testing.T) {
		// Arrange
		snapshot := io.NopCloser(strings.NewReader(`key1,value1`))
		storer.EXPECT().Clear()
		storer.EXPECT().Set("key1", "value1").Return(nil)

		// Act
		err := fsm.Restore(snapshot)

		// Assert
		require.NoError(t, err)
	})

	t.Run("Snapshot", func(t *testing.T) {
		// Test: Get the snapshot
		// Arrange
		storer.EXPECT().Dump().Return(map[string]string{
			"key1": "value1",
		})

		// Act
		snapshot, err := fsm.Snapshot()

		// Assert
		require.NoError(t, err)
		require.NotNil(t, snapshot)

		// Test: Persist the snapshot
		// Arrange
		res := &strings.Builder{}
		sink := &MockSnapshotSink{
			Writer: res,
		}

		// Act
		err = snapshot.Persist(sink)

		// Assert
		require.NoError(t, err)
		require.Equal(t, 0, sink.calledCancelCounter)
		require.Equal(t, 1, sink.callCloseCounter)
		require.Equal(t, "key1,value1\n", res.String())
	})
}

var _ raft.SnapshotSink = (*MockSnapshotSink)(nil)

type MockSnapshotSink struct {
	io.Writer
	calledCancelCounter int
	callCloseCounter    int
}

func (m *MockSnapshotSink) Cancel() error {
	m.calledCancelCounter++
	return nil
}

func (m *MockSnapshotSink) Close() error {
	m.callCloseCounter++
	return nil
}

func (m *MockSnapshotSink) ID() string {
	return ""
}
