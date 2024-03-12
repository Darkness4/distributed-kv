// nolint: ireturn
package raftpebble

import (
	"github.com/cockroachdb/pebble"
	"github.com/cockroachdb/pebble/vfs"
)

type options struct {
	config   RaftLogRocksDBConfig
	logger   pebble.Logger
	fs       vfs.FS
	walDir   string
	dir      string
	callback LogDBCallback

	// optional, more details see pebble Options
	// if use pebble options, config options can't use
	pebbleOptions *pebble.Options
}

type Option interface {
	apply(*options)
}

type pebbleKVStoreOption struct {
	f func(*options)
}

func (fdo *pebbleKVStoreOption) apply(do *options) {
	fdo.f(do)
}

func newOption(f func(*options)) *pebbleKVStoreOption {
	return &pebbleKVStoreOption{
		f: f,
	}
}

func WithConfig(config RaftLogRocksDBConfig) Option {
	return newOption(func(o *options) {
		o.config = config
	})
}

func WithLogger(logger pebble.Logger) Option {
	return newOption(func(o *options) {
		o.logger = logger
	})
}

func WithFS(fs vfs.FS) Option {
	return newOption(func(o *options) {
		o.fs = fs
	})
}

func WithWalDirPath(walDir string) Option {
	return newOption(func(o *options) {
		o.walDir = walDir
	})
}

func WithDbDirPath(dbDir string) Option {
	return newOption(func(o *options) {
		o.dir = dbDir
	})
}

func WithLogDBCallback(cb LogDBCallback) Option {
	return newOption(func(o *options) {
		o.callback = cb
	})
}

func WithPebbleOptions(opts *pebble.Options) Option {
	return newOption(func(o *options) {
		o.pebbleOptions = opts
	})
}

func getOptions(opts ...Option) *options {
	options := &options{
		config: GetDefaultRaftLogRocksDBConfig(),
		logger: pebble.DefaultLogger,
		fs:     vfs.Default,
	}

	for _, o := range opts {
		o.apply(options)
	}

	return options
}
