package raftpebble

import "reflect"

const (
	//defaultLogDBShards uint64 = 16
	// single raft log store
	defaultLogDBShards uint64 = 1
)

// RaftLogRocksDBConfig
// pebble add sst block lru cache for read
// others more detail see rocksdb guid wiki
type RaftLogRocksDBConfig struct {
	Shards                             uint64
	KVKeepLogFileNum                   uint64
	KVMaxBackgroundCompactions         uint64
	KVMaxBackgroundFlushes             uint64
	KVLRUCacheSize                     uint64
	KVWriteBufferSize                  uint64
	KVMaxWriteBufferNumber             uint64
	KVLevel0FileNumCompactionTrigger   uint64
	KVLevel0SlowdownWritesTrigger      uint64
	KVLevel0StopWritesTrigger          uint64
	KVMaxBytesForLevelBase             uint64
	KVMaxBytesForLevelMultiplier       uint64
	KVTargetFileSizeBase               uint64
	KVTargetFileSizeMultiplier         uint64
	KVLevelCompactionDynamicLevelBytes uint64
	KVRecycleLogFileNum                uint64
	KVNumOfLevels                      uint64
	KVBlockSize                        uint64
	SaveBufferSize                     uint64
	MaxSaveBufferSize                  uint64
}

// GetDefaultRaftLogRocksDBConfig returns the default configurations for the LogDB
// storage engine. The default LogDB configuration use up to 8GBytes memory.
func GetDefaultRaftLogRocksDBConfig() RaftLogRocksDBConfig {
	return GetLargeMemRaftLogRocksDBConfig()
}

// GetTinyMemRaftLogRocksDBConfig returns a LogDB config aimed for minimizing memory
// size. When using the returned config, LogDB takes up to 256MBytes memory.
func GetTinyMemRaftLogRocksDBConfig() RaftLogRocksDBConfig {
	cfg := getDefaultRaftLogRocksDBConfig()
	cfg.KVWriteBufferSize = 4 * 1024 * 1024
	cfg.KVMaxWriteBufferNumber = 4
	return cfg
}

// GetSmallMemRaftLogRocksDBConfig returns a LogDB config aimed to keep memory size at
// low level. When using the returned config, LogDB takes up to 1GBytes memory.
func GetSmallMemRaftLogRocksDBConfig() RaftLogRocksDBConfig {
	cfg := getDefaultRaftLogRocksDBConfig()
	cfg.KVWriteBufferSize = 16 * 1024 * 1024
	cfg.KVMaxWriteBufferNumber = 4
	return cfg
}

// GetMediumMemRaftLogRocksDBConfig returns a LogDB config aimed to keep memory size at
// medium level. When using the returned config, LogDB takes up to 4GBytes
// memory.
func GetMediumMemRaftLogRocksDBConfig() RaftLogRocksDBConfig {
	cfg := getDefaultRaftLogRocksDBConfig()
	cfg.KVWriteBufferSize = 64 * 1024 * 1024
	cfg.KVMaxWriteBufferNumber = 4
	return cfg
}

// GetLargeMemRaftLogRocksDBConfig returns a LogDB config aimed to keep memory size to be
// large for good I/O performance. It is the default setting used by the system.
// When using the returned config, LogDB takes up to 8GBytes memory.
func GetLargeMemRaftLogRocksDBConfig() RaftLogRocksDBConfig {
	return getDefaultRaftLogRocksDBConfig()
}

func getDefaultRaftLogRocksDBConfig() RaftLogRocksDBConfig {
	return RaftLogRocksDBConfig{
		Shards:                             defaultLogDBShards,
		KVMaxBackgroundCompactions:         2,
		KVMaxBackgroundFlushes:             2,
		KVLRUCacheSize:                     0,
		KVKeepLogFileNum:                   16,
		KVWriteBufferSize:                  128 * 1024 * 1024,
		KVMaxWriteBufferNumber:             4,
		KVLevel0FileNumCompactionTrigger:   8,
		KVLevel0SlowdownWritesTrigger:      17,
		KVLevel0StopWritesTrigger:          24,
		KVMaxBytesForLevelBase:             4 * 1024 * 1024 * 1024,
		KVMaxBytesForLevelMultiplier:       2,
		KVTargetFileSizeBase:               16 * 1024 * 1024,
		KVTargetFileSizeMultiplier:         2,
		KVLevelCompactionDynamicLevelBytes: 0,
		KVRecycleLogFileNum:                0,
		KVNumOfLevels:                      7,
		KVBlockSize:                        32 * 1024,
		SaveBufferSize:                     32 * 1024,
		MaxSaveBufferSize:                  64 * 1024 * 1024,
	}
}

// MemorySizeMB returns the estimated upper bound memory size used by the LogDB
// storage engine. The returned value is in MBytes.
func (cfg *RaftLogRocksDBConfig) MemorySizeMB() uint64 {
	ss := cfg.KVWriteBufferSize * cfg.KVMaxWriteBufferNumber
	bs := ss * cfg.Shards
	return bs / (1024 * 1024)
}

// IsEmpty returns a boolean value indicating whether the RaftLogRocksDBConfig instance
// is empty.
func (cfg *RaftLogRocksDBConfig) IsEmpty() bool {
	return reflect.DeepEqual(cfg, &RaftLogRocksDBConfig{})
}
