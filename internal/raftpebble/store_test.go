package raftpebble

import (
	"bytes"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"testing"

	"github.com/cockroachdb/pebble"
	"github.com/cockroachdb/pebble/vfs"
	"github.com/hashicorp/raft"
	"github.com/stretchr/testify/assert"
)

func mockCallBack(busy bool) {
	fmt.Printf("busy %t\n", busy)
}

func testPebbleKVStore(t testing.TB) (kvStore *PebbleKVStore, walDir, dir string) {
	dir, err := os.MkdirTemp("", "raft-pebble")
	if err != nil {
		t.Fatalf("err. %s", err)
	}
	os.RemoveAll(dir)

	walDir, err = os.MkdirTemp("", "raft-pebble-wal")
	if err != nil {
		t.Fatalf("err. %s", err)
	}
	os.RemoveAll(walDir)

	if os.Getenv("mock") == "" {
		kvStore, err = New(WithDbDirPath(dir))
	} else {
		kvStore, err = New(
			WithConfig(GetDefaultRaftLogRocksDBConfig()),
			WithLogger(pebble.DefaultLogger),
			WithFS(vfs.Default),
			WithWalDirPath(walDir),
			WithDbDirPath(dir),
			WithLogDBCallback(mockCallBack),
			WithPebbleOptions(nil),
		)
	}

	if err != nil {
		t.Fatalf("err. %s", err)
	}

	return kvStore, walDir, dir
}

func TestPebbleKVStore_Implements(t *testing.T) {
	var store interface{} = &PebbleKVStore{}
	if _, ok := store.(raft.StableStore); !ok {
		t.Fatalf("PebbleKVStore does not implement raft.StableStore")
	}
	if _, ok := store.(raft.LogStore); !ok {
		t.Fatalf("PebbleKVStore does not implement raft.LogStore")
	}
}

func TestPebbleKVStore_Empty(t *testing.T) {
	store, walDir, dir := testPebbleKVStore(t)
	defer func() {
		store.Close()
		os.RemoveAll(walDir)
		os.RemoveAll(dir)
	}()

	// Should get 0 index on empty log
	idx, err := store.FirstIndex()
	assert.Nil(t, err)
	assert.Equal(t, idx, uint64(0))

	// Should get 0 index on empty log
	idx, err = store.LastIndex()
	assert.Nil(t, err)
	assert.Equal(t, idx, uint64(0))

	// Should get 0 index on empty log
	idx, err = store.FirstIndex()
	assert.Nil(t, err)
	assert.Equal(t, idx, uint64(0))
}

func TestPebbleKVStore_Empty_FirstIndex(t *testing.T) {
	store, walDir, dir := testPebbleKVStore(t)
	defer func() {
		store.Close()
		os.RemoveAll(walDir)
		os.RemoveAll(dir)
	}()

	// Should get 0 index on empty log
	idx, err := store.FirstIndex()
	assert.Nil(t, err)
	assert.Equal(t, idx, uint64(0))

	// Set prefixConf meta
	// Attempt to set the k/v pair
	k, v := prefixLog, uint64(123)
	if err := store.SetUint64(k, v); err != nil {
		t.Fatalf("err: %s", err)
	}

	for i := 0; i < 3; i++ {
		// Read back the value
		val, err := store.GetUint64(k)
		if err != nil {
			t.Fatalf("err: %s", err)
		}
		if val != v {
			t.Fatalf("bad: %v", val)
		}
	}

	// Should get 0 index on empty log
	idx, err = store.FirstIndex()
	assert.Nil(t, err)
	assert.Equal(t, idx, uint64(0))
}

func TestPebbleKVStore_SetLogs(t *testing.T) {
	store, walDir, dir := testPebbleKVStore(t)
	defer func() {
		store.Close()
		os.RemoveAll(walDir)
		os.RemoveAll(dir)
	}()

	// Create a set of logs
	logs := []*raft.Log{
		{
			Index: 1,
			Term:  1,
			Type:  0,
			Data:  []byte("log1"),
		},
		{
			Index: 2,
			Term:  1,
			Type:  0,
			Data:  []byte("log2"),
		},
	}

	// Attempt to store the logs
	if err := store.StoreLogs(logs); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Ensure we stored them all
	result1, result2 := new(raft.Log), new(raft.Log)
	if err := store.GetLog(1, result1); err != nil {
		t.Fatalf("err: %s", err)
	}
	assert.Equal(t, logs[0], result1)

	if err := store.GetLog(2, result2); err != nil {
		t.Fatalf("err: %s", err)
	}
	assert.Equal(t, logs[1], result2)
}

func TestPebbleKVStore_FirstIndex(t *testing.T) {
	store, walDir, dir := testPebbleKVStore(t)
	defer func() {
		store.Close()
		os.RemoveAll(walDir)
		os.RemoveAll(dir)
	}()

	// Set a mock raft log
	logs := []*raft.Log{
		{
			Index: 1,
			Term:  1,
			Type:  0,
			Data:  []byte("log1"),
		},
		{
			Index: 2,
			Term:  1,
			Type:  0,
			Data:  []byte("log2"),
		},
		{
			Index: 3,
			Term:  1,
			Type:  0,
			Data:  []byte("log3"),
		},
	}
	if err := store.StoreLogs(logs); err != nil {
		t.Fatalf("bad: %s", err)
	}

	// Fetch the first Raft index
	idx, err := store.FirstIndex()
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if idx != 1 {
		t.Fatalf("bad index: %d", idx)
	}
}

func TestPebbleKVStore_Empty_LastIndex(t *testing.T) {
	store, walDir, dir := testPebbleKVStore(t)
	defer func() {
		store.Close()
		os.RemoveAll(walDir)
		os.RemoveAll(dir)
	}()

	// Should get 0 index on empty log
	idx, err := store.LastIndex()
	assert.Nil(t, err)
	assert.Equal(t, idx, uint64(0))

	// Set prefixConf meta
	// Attempt to set the k/v pair
	k, v := prefixLog, uint64(123)
	if err := store.SetUint64(k, v); err != nil {
		t.Fatalf("err: %s", err)
	}
	// Read back the value
	val, err := store.GetUint64(k)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if val != v {
		t.Fatalf("bad: %v", val)
	}

	// Should get 0 index on empty log
	idx, err = store.LastIndex()
	assert.Nil(t, err)
	assert.Equal(t, idx, uint64(0))
}

func TestPebbleKVStore_LastIndex(t *testing.T) {
	store, walDir, dir := testPebbleKVStore(t)
	defer func() {
		store.Close()
		os.RemoveAll(walDir)
		os.RemoveAll(dir)
	}()

	// Set a mock raft log
	logs := []*raft.Log{
		{
			Index: 1,
			Term:  1,
			Type:  0,
			Data:  []byte("log1"),
		},
		{
			Index: 2,
			Term:  1,
			Type:  0,
			Data:  []byte("log2"),
		},
		{
			Index: 3,
			Term:  1,
			Type:  0,
			Data:  []byte("log3"),
		},
	}
	if err := store.StoreLogs(logs); err != nil {
		t.Fatalf("bad: %s", err)
	}

	// Fetch the last Raft index
	idx, err := store.LastIndex()
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if idx != 3 {
		t.Fatalf("bad index: %d", idx)
	}
}

func TestPebbleKVStore_GetLog(t *testing.T) {
	store, walDir, dir := testPebbleKVStore(t)
	defer func() {
		store.Close()
		os.RemoveAll(walDir)
		os.RemoveAll(dir)
	}()

	log := new(raft.Log)

	// Should return an error on non-existent log
	if err := store.GetLog(1, log); err != raft.ErrLogNotFound {
		t.Fatalf("expected raft log not found error, got: %v", err)
	}

	// Set a mock raft log
	logs := []*raft.Log{
		{
			Index: 1,
			Term:  1,
			Type:  0,
			Data:  []byte("log1"),
		},
		{
			Index: 2,
			Term:  1,
			Type:  0,
			Data:  []byte("log2"),
		},
		{
			Index: 3,
			Term:  1,
			Type:  0,
			Data:  []byte("log3"),
		},
	}
	if err := store.StoreLogs(logs); err != nil {
		t.Fatalf("bad: %s", err)
	}

	// Should return the proper log
	if err := store.GetLog(2, log); err != nil {
		t.Fatalf("err: %s", err)
	}
	if !reflect.DeepEqual(log, logs[1]) {
		t.Fatalf("bad: %#v", log)
	}
}

func TestPebbleKVStore_SetLog(t *testing.T) {
	store, walDir, dir := testPebbleKVStore(t)
	defer func() {
		store.Close()
		os.RemoveAll(walDir)
		os.RemoveAll(dir)
	}()

	// Create the log
	log := &raft.Log{
		Index: 1,
		Term:  0,
		Type:  0,
		Data:  []byte("log1"),
	}

	// Attempt to store the log
	if err := store.StoreLog(log); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Retrieve the log again
	result := new(raft.Log)
	if err := store.GetLog(1, result); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Ensure the log comes back the same
	if !reflect.DeepEqual(log, result) {
		t.Fatalf("bad: %v", result)
	}
}

func TestPebbleKVStore_DeleteRange(t *testing.T) {
	store, walDir, dir := testPebbleKVStore(t)
	defer func() {
		store.Close()
		os.RemoveAll(walDir)
		os.RemoveAll(dir)
	}()

	// Create a set of logs
	logs := []*raft.Log{
		{
			Index: 1,
			Term:  1,
			Type:  0,
			Data:  []byte("log1"),
		},
		{
			Index: 2,
			Term:  1,
			Type:  0,
			Data:  []byte("log2"),
		},
		{
			Index: 3,
			Term:  1,
			Type:  0,
			Data:  []byte("log3"),
		},
	}

	// Attempt to store the logs
	if err := store.StoreLogs(logs); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Attempt to delete a range of logs
	if err := store.DeleteRange(1, 2); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Ensure the logs were deleted
	if err := store.GetLog(1, new(raft.Log)); err != raft.ErrLogNotFound {
		t.Fatalf("should have deleted log1 got err:%v", err)
	}
	if err := store.GetLog(2, new(raft.Log)); err != raft.ErrLogNotFound {
		t.Fatalf("should have deleted log2 got err:%v", err)
	}
}

func TestPebbleKVStore_Set_Get(t *testing.T) {
	store, walDir, dir := testPebbleKVStore(t)
	defer func() {
		store.Close()
		os.RemoveAll(walDir)
		os.RemoveAll(dir)
	}()

	// Returns error on non-existent key
	if _, err := store.Get([]byte("bad")); err != ErrKeyNotFound {
		t.Fatalf("expected not found error, got: %q", err)
	}

	for j := 0; j < 3; j++ {
		k, v := []byte("hello"+strconv.Itoa(j)), []byte("world")

		// Try to set a k/v pair
		if err := store.Set(k, v); err != nil {
			t.Fatalf("err: %s", err)
		}

		for i := 0; i < 2; i++ {
			// Try to read it back
			val, err := store.Get(k)
			if err != nil {
				t.Fatalf("err: %s", err)
			}
			if !bytes.Equal(val, v) {
				t.Fatalf("i:%d key: %s get: %s want: %s", i, k, val, v)
			}
			// test pebble.DB get from LazyValue.Value, modify
			// https://github.com/hashicorp/raft/blob/v1.5.0/raft.go#L1623 no modify
			// if have modify op, Get use copy, open to test
			/*
				if len(val) > 0 {
					val[0] = 0x1
				}
			*/
		}
	}
}

func TestPebbleKVStore_SetUint64_GetUint64(t *testing.T) {
	store, walDir, dir := testPebbleKVStore(t)
	defer func() {
		store.Close()
		os.RemoveAll(walDir)
		os.RemoveAll(dir)
	}()

	// Returns error on non-existent key
	if _, err := store.GetUint64([]byte("bad")); err != ErrKeyNotFound {
		t.Fatalf("expected not found error, got: %q", err)
	}

	k, v := []byte("abc"), uint64(123)

	// Attempt to set the k/v pair
	if err := store.SetUint64(k, v); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Read back the value
	val, err := store.GetUint64(k)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if val != v {
		t.Fatalf("bad: %v", val)
	}
}

func TestPebbleKVStore_RaftMetaOp(t *testing.T) {
	store, walDir, dir := testPebbleKVStore(t)
	defer func() {
		store.Close()
		os.RemoveAll(walDir)
		os.RemoveAll(dir)
	}()

	err := store.Set([]byte("store"), []byte("set"))
	assert.Nil(t, err)

	value, err := store.Get([]byte("store"))
	assert.Nil(t, err)
	assert.EqualValues(t, value, "set")

	err = store.SetUint64([]byte("index"), 111)
	assert.Nil(t, err)

	index, err := store.GetUint64([]byte("index"))
	assert.EqualValues(t, 111, index)
	assert.Nil(t, err)
}

func TestPebbleKVStore_RaftLogOp(t *testing.T) {
	store, walDir, dir := testPebbleKVStore(t)
	defer func() {
		store.Close()
		os.RemoveAll(walDir)
		os.RemoveAll(dir)
	}()

	var logs []*raft.Log
	for i := 0; i < 20; i++ {
		logs = append(logs, &raft.Log{
			Index: uint64(i),
			Term:  uint64(i),
			Type:  0,
			Data:  []byte(fmt.Sprintf("data%d", i)),
		})
	}

	err := store.StoreLogs(logs)
	assert.Nil(t, err)

	index, err := store.FirstIndex()
	assert.Nil(t, err)
	assert.EqualValues(t, 0, index)

	index, err = store.LastIndex()
	assert.Nil(t, err)
	assert.EqualValues(t, 19, index)

	for i := 0; i < 10; i++ {
		logptr := new(raft.Log)
		err = store.GetLog(uint64(i), logptr)
		assert.Nil(t, err)
		assert.EqualValues(t, i, logptr.Index)
	}

	err = store.DeleteRange(0, 10)
	assert.Nil(t, err)

	for i := 0; i < 10; i++ {
		logptr := new(raft.Log)
		err = store.GetLog(uint64(i), logptr)
		assert.ErrorIs(t, err, raft.ErrLogNotFound)
	}

	logptr := new(raft.Log)
	err = store.GetLog(11, logptr)
	assert.Nil(t, err)
	assert.EqualValues(t, 11, logptr.Index)
}
