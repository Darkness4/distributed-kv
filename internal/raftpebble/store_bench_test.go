package raftpebble

import (
	"os"
	"testing"

	"github.com/hashicorp/raft"
	raftbench "github.com/hashicorp/raft/bench"
)

func BenchmarkPebbleKVStoreStore_FirstIndex(b *testing.B) {
	store, walDir, dir := testPebbleKVStore(b)
	defer func() {
		store.Close()
		os.RemoveAll(walDir)
		os.RemoveAll(dir)
	}()

	raftbench.FirstIndex(b, store)
}

func BenchmarkPebbleKVStoreStore_LastIndex(b *testing.B) {
	store, walDir, dir := testPebbleKVStore(b)
	defer func() {
		store.Close()
		os.RemoveAll(walDir)
		os.RemoveAll(dir)
	}()

	raftbench.LastIndex(b, store)
}

func BenchmarkPebbleKVStoreStore_GetLog(b *testing.B) {
	store, walDir, dir := testPebbleKVStore(b)
	defer func() {
		store.Close()
		os.RemoveAll(walDir)
		os.RemoveAll(dir)
	}()

	raftbench.GetLog(b, store)
}

func BenchmarkPebbleKVStoreStore_StoreLog(b *testing.B) {
	store, walDir, dir := testPebbleKVStore(b)
	defer func() {
		store.Close()
		os.RemoveAll(walDir)
		os.RemoveAll(dir)
	}()

	raftbench.StoreLog(b, store)
}

func BenchmarkPebbleKVStoreStore_StoreLogs(b *testing.B) {
	store, walDir, dir := testPebbleKVStore(b)
	defer func() {
		store.Close()
		os.RemoveAll(walDir)
		os.RemoveAll(dir)
	}()

	raftbench.StoreLogs(b, store)
}

func BenchmarkPebbleKVStoreStore_DeleteRange(b *testing.B) {
	store, walDir, dir := testPebbleKVStore(b)
	defer func() {
		store.Close()
		os.RemoveAll(walDir)
		os.RemoveAll(dir)
	}()

	raftbench.DeleteRange(b, store)
}

func BenchmarkPebbleKVStoreStore_Set(b *testing.B) {
	store, walDir, dir := testPebbleKVStore(b)
	defer func() {
		store.Close()
		os.RemoveAll(walDir)
		os.RemoveAll(dir)
	}()

	raftbench.Set(b, store)
}

func BenchmarkPebbleKVStoreStore_Get(b *testing.B) {
	store, walDir, dir := testPebbleKVStore(b)
	defer func() {
		store.Close()
		os.RemoveAll(walDir)
		os.RemoveAll(dir)
	}()

	raftbench.Get(b, store)
}

func BenchmarkPebbleKVStoreStore_SetUint64(b *testing.B) {
	store, walDir, dir := testPebbleKVStore(b)
	defer func() {
		store.Close()
		os.RemoveAll(walDir)
		os.RemoveAll(dir)
	}()

	raftbench.SetUint64(b, store)
}

func BenchmarkPebbleKVStoreStore_GetUint64(b *testing.B) {
	store, walDir, dir := testPebbleKVStore(b)
	defer func() {
		store.Close()
		os.RemoveAll(walDir)
		os.RemoveAll(dir)
	}()

	raftbench.GetUint64(b, store)
}

func BenchmarkSet(b *testing.B) {
	store, walDir, dir := testPebbleKVStore(b)
	defer func() {
		store.Close()
		os.RemoveAll(walDir)
		os.RemoveAll(dir)
	}()
	for n := 0; n < b.N; n++ {
		_ = store.Set(uint64ToBytes(uint64(n)), []byte("val"))
	}
}

func BenchmarkGet(b *testing.B) {
	store, walDir, dir := testPebbleKVStore(b)
	defer func() {
		store.Close()
		os.RemoveAll(walDir)
		os.RemoveAll(dir)
	}()

	for n := 0; n < b.N; n++ {
		_ = store.Set(uint64ToBytes(uint64(n)), []byte("val"))
	}

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		_, _ = store.Get(uint64ToBytes(uint64(n)))
	}
}

func BenchmarkStoreLogs(b *testing.B) {
	store, walDir, dir := testPebbleKVStore(b)
	defer func() {
		store.Close()
		os.RemoveAll(walDir)
		os.RemoveAll(dir)
	}()

	for n := 0; n < b.N; n++ {
		_ = store.StoreLogs([]*raft.Log{
			{
				Index: uint64(n),
				Term:  uint64(n),
			},
		})
	}
}

func BenchmarkGetLog(b *testing.B) {
	store, walDir, dir := testPebbleKVStore(b)
	defer func() {
		store.Close()
		os.RemoveAll(walDir)
		os.RemoveAll(dir)
	}()

	for n := 0; n < b.N; n++ {
		_ = store.StoreLogs([]*raft.Log{
			{
				Index: uint64(n),
				Term:  uint64(n),
			},
		})
	}

	b.ResetTimer()

	ralog := new(raft.Log)
	for n := 0; n < b.N; n++ {
		_ = store.GetLog(uint64(n), ralog)
	}
}
