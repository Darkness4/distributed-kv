package raftpebble

import (
	"errors"

	"github.com/cockroachdb/pebble"
	"github.com/hashicorp/raft"
	"github.com/lni/goutils/syncutil"
)

var (
	// Prefix names to distingish between logs and conf
	prefixLog  = []byte{0x00}
	prefixConf = []byte{0x01}

	// ErrKeyNotFound is an error indicating a given key does not exist
	// for hashicorp raft vote meta stable get check, if err != nil && err.Error() != "not found"
	ErrKeyNotFound = errors.New("not found")
)

const (
	maxLogFileSize = 1024 * 1024 * 128
)

// KV is a pebble based LogStore StableStore type.
type PebbleKVStore struct {
	db    *pebble.DB
	dbSet chan struct{}
	event *eventListener

	options *options

	defaultWriteOpts *pebble.WriteOptions
}

// LogDBCallback is a callback function called by the LogDB
// eg: do some metrics export
type LogDBCallback func(busy bool)

type eventListener struct {
	kv      *PebbleKVStore
	stopper *syncutil.Stopper
}

func (l *eventListener) close() {
	l.stopper.Stop()
}

func (l *eventListener) notify() {
	l.stopper.RunWorker(func() {
		select {
		case <-l.kv.dbSet:
			if l.kv.options.callback != nil {
				memSizeThreshold := l.kv.options.config.KVWriteBufferSize *
					l.kv.options.config.KVMaxWriteBufferNumber * 19 / 20
				l0FileNumThreshold := l.kv.options.config.KVLevel0StopWritesTrigger - 1
				m := l.kv.db.Metrics()
				busy := m.MemTable.Size >= memSizeThreshold ||
					uint64(m.Levels[0].Sublevels) >= l0FileNumThreshold
				l.kv.options.callback(busy)
			}
		default:
		}
	})
}

func (l *eventListener) onCompactionEnd(pebble.CompactionInfo) {
	l.notify()
}

func (l *eventListener) onFlushEnd(pebble.FlushInfo) {
	l.notify()
}

func (l *eventListener) onWALCreated(pebble.WALCreateInfo) {
	l.notify()
}

// New uses the supplied config to open the Pebble db and prepare it
// for using as a raft backend pebble kv store.
// level no compression for raft meta/log store
func New(options ...Option) (*PebbleKVStore, error) {
	// config defined options
	kvStoreOpts := getOptions(options...)
	config := kvStoreOpts.config
	logger := kvStoreOpts.logger
	fs := kvStoreOpts.fs
	walDir := kvStoreOpts.walDir

	// pebble options
	numOfLevels := int64(config.KVNumOfLevels)
	lopts := make([]pebble.LevelOptions, 0)
	sz := config.KVTargetFileSizeBase
	for l := int64(0); l < numOfLevels; l++ {
		opt := pebble.LevelOptions{
			Compression:    pebble.NoCompression,
			BlockSize:      int(config.KVBlockSize),
			TargetFileSize: int64(sz),
		}
		sz = sz * config.KVTargetFileSizeMultiplier
		lopts = append(lopts, opt)
	}
	cache := pebble.NewCache(int64(config.KVLRUCacheSize))
	opts := &pebble.Options{
		Levels:                      lopts,
		MaxManifestFileSize:         maxLogFileSize,
		MemTableSize:                config.KVWriteBufferSize,
		MemTableStopWritesThreshold: int(config.KVMaxWriteBufferNumber),
		LBaseMaxBytes:               int64(config.KVMaxBytesForLevelBase),
		L0CompactionFileThreshold:   int(config.KVLevel0FileNumCompactionTrigger),
		L0StopWritesThreshold:       int(config.KVLevel0StopWritesTrigger),
		Cache:                       cache,
		Logger:                      logger,
		FS:                          fs,
		WALDir:                      walDir,
		FormatMajorVersion:          pebble.FormatNewest,
	}

	kv := &PebbleKVStore{
		options: kvStoreOpts,
		dbSet:   make(chan struct{}),
	}
	event := &eventListener{
		kv:      kv,
		stopper: syncutil.NewStopper(),
	}
	opts.EventListener = &pebble.EventListener{
		WALCreated:    event.onWALCreated,
		FlushEnd:      event.onFlushEnd,
		CompactionEnd: event.onCompactionEnd,
	}

	if kvStoreOpts.pebbleOptions != nil {
		opts = kvStoreOpts.pebbleOptions
	}

	pdb, err := pebble.Open(kvStoreOpts.dir, opts)
	if err != nil {
		return nil, err
	}
	cache.Unref()
	kv.db = pdb
	kv.setEventListener(event)
	//kv.defaultWriteOpts = &pebble.WriteOptions{Sync: true}
	kv.defaultWriteOpts = &pebble.WriteOptions{Sync: false}
	return kv, nil
}

func (s *PebbleKVStore) setEventListener(event *eventListener) {
	if s.db == nil || s.event != nil {
		panic("unexpected kv state")
	}
	s.event = event
	close(s.dbSet)
	// force a WALCreated event as the one issued when opening the DB didn't get
	// handled
	event.onWALCreated(pebble.WALCreateInfo{})
}

// Close the Raft log
func (s *PebbleKVStore) Close() error {
	s.event.close()
	return s.db.Close()
}

// log store

// FirstIndex returns the first known index from the Raft log.
// use SeekPrefixGE,Reverse iteration (Prev) is not supported when an iterator is in prefix iteration mode.
// https://pkg.go.dev/github.com/cockroachdb/pebble#Iterator.SeekPrefixGE
// so use lowerBound,UpperBound for iter prefixLog
// notice: if not found return 0, nil
func (s *PebbleKVStore) FirstIndex() (first uint64, err error) {
	iter, err := s.db.NewIter(&pebble.IterOptions{
		LowerBound: prefixLog,
		UpperBound: prefixConf,
		KeyTypes:   pebble.IterKeyTypePointsAndRanges,
	})
	if err != nil {
		return 0, err
	}

	defer func() {
		err = FirstError(err, iter.Close())
	}()

	if iter.First() {
		first = bytesToUint64(iter.Key()[len(prefixLog):])
	}

	return
}

// LastIndex returns the last known index from the Raft log.
// use SeekPrefixGE,Reverse iteration (Prev) is not supported when an iterator is in prefix iteration mode.
// https://pkg.go.dev/github.com/cockroachdb/pebble#Iterator.SeekPrefixGE
// so use lowerBound,UpperBound for iter prefixLog
// notice: if not found return 0, nil
func (s *PebbleKVStore) LastIndex() (last uint64, err error) {
	iter, err := s.db.NewIter(&pebble.IterOptions{
		LowerBound: prefixLog,
		UpperBound: prefixConf,
		KeyTypes:   pebble.IterKeyTypePointsAndRanges,
	})
	if err != nil {
		return 0, err
	}

	defer func() {
		err = FirstError(err, iter.Close())
	}()

	if iter.Last() {
		last = bytesToUint64(iter.Key()[len(prefixLog):])
	}

	return
}

// GetLog gets a log entry from Pebble at a given index.
// notice: if index log not found return raft ErrLogNotFound
func (s *PebbleKVStore) GetLog(index uint64, log *raft.Log) (err error) {
	key := append(prefixLog, uint64ToBytes(index)...)
	val, closer, err := s.db.Get(key)
	defer func() {
		if closer != nil {
			err = FirstError(err, closer.Close())
		}
	}()
	// close err when get fail
	if err != nil && err != pebble.ErrNotFound {
		return
	}
	if val == nil {
		return raft.ErrLogNotFound
	}

	return decodeMsgPack(val, log)
}

// StoreLog stores a single raft log.
func (s *PebbleKVStore) StoreLog(log *raft.Log) (err error) {
	//return s.StoreLogs([]*raft.Log{log})
	return s.storeLog(log)
}

// storeLog stores a single raft log.
func (s *PebbleKVStore) storeLog(log *raft.Log) (err error) {
	key := append(prefixLog, uint64ToBytes(log.Index)...)
	val, err := encodeMsgPack(log)
	if err != nil {
		return err
	}

	//return s.db.Set(key, val.Bytes(), &pebble.WriteOptions{Sync: true})
	return s.db.Set(key, val.Bytes(), &pebble.WriteOptions{Sync: false})
}

// StoreLogs stores a set of raft logs.
func (s *PebbleKVStore) StoreLogs(logs []*raft.Log) (err error) {
	wb := s.db.NewBatch()
	defer func() {
		err = FirstError(err, wb.Close())
	}()

	for _, log := range logs {
		key := append(prefixLog, uint64ToBytes(log.Index)...)
		val, err := encodeMsgPack(log)
		if err != nil {
			return err
		}

		//err = wb.Set(key, val.Bytes(), &pebble.WriteOptions{Sync: true})
		err = wb.Set(key, val.Bytes(), &pebble.WriteOptions{Sync: false})
		if err != nil {
			return err
		}
	}

	//return s.db.Apply(wb, &pebble.WriteOptions{Sync: true})
	return s.db.Apply(wb, &pebble.WriteOptions{Sync: false})
}

// DeleteRange deletes logs within a given range inclusively.
func (s *PebbleKVStore) DeleteRange(min, max uint64) (err error) {
	//wo := &pebble.WriteOptions{Sync: true}
	wo := &pebble.WriteOptions{Sync: false}
	fk := append(prefixLog, uint64ToBytes(min)...)
	lk := append(prefixLog, uint64ToBytes(max+1)...)

	//return s.deleteRange(fk, lk, wo)
	return s.db.DeleteRange(fk, lk, wo)
}

// Set is used to set a key/value set outside of the raft log.
func (s *PebbleKVStore) Set(key []byte, val []byte) (err error) {
	confKey := append(prefixConf, key...)

	//return s.db.Set(confKey, val, &pebble.WriteOptions{Sync: true})
	return s.db.Set(confKey, val, &pebble.WriteOptions{Sync: false})
}

// Get is used to retrieve a value from the k/v store by key
// notice: if key/val not found return ErrKeyNotFound
func (s *PebbleKVStore) Get(key []byte) (value []byte, err error) {
	confKey := append(prefixConf, key...)
	err = s.GetValue(confKey, func(val []byte) error {
		if val == nil {
			err = ErrKeyNotFound
			return err
		}

		value = val
		// https://github.com/hashicorp/raft/blob/v1.5.0/raft.go#L1623 no modify
		// if have modify op, use copy, but add some GC
		/*
			value = make([]byte, len(val))
			copy(value, val)
		*/

		return nil
	})

	return
}

// SetUint64 is like Set, but handles uint64 values
func (s *PebbleKVStore) SetUint64(key []byte, val uint64) error {
	return s.Set(key, uint64ToBytes(val))
}

// GetUint64 is like Get, but return uint64 values
func (s *PebbleKVStore) GetUint64(key []byte) (term uint64, err error) {
	confKey := append(prefixConf, key...)
	err = s.GetValue(confKey, func(val []byte) error {
		if val == nil {
			err = ErrKeyNotFound
			return err
		}

		term = bytesToUint64(val)

		return nil
	})

	return
}

// GetValue defer closer.Close to do op safely when modify get value
func (s *PebbleKVStore) GetValue(key []byte, op func([]byte) error) (err error) {
	val, closer, err := s.db.Get(key)
	if err != nil && err != pebble.ErrNotFound {
		return
	}
	defer func() {
		if closer != nil {
			err = FirstError(err, closer.Close())
		}
	}()
	return op(val)
}
