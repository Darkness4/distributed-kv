package persisted

import (
	"path/filepath"

	"github.com/cockroachdb/pebble"
)

type Store struct {
	*pebble.DB
}

func New(path string) *Store {
	path = filepath.Join(path, "pebble")
	db, err := pebble.Open(path, &pebble.Options{})
	if err != nil {
		panic(err)
	}
	return &Store{db}
}

func (s *Store) Get(key string) (string, error) {
	v, closer, err := s.DB.Get([]byte(key))
	if err != nil {
		return "", err
	}
	defer closer.Close()
	return string(v), nil
}

func (s *Store) Set(key string, value string) error {
	return s.DB.Set([]byte(key), []byte(value), pebble.Sync)
}

func (s *Store) Delete(key string) error {
	return s.DB.Delete([]byte(key), pebble.Sync)
}

func (s *Store) Dump() map[string]string {
	data := make(map[string]string)
	iter, err := s.NewIter(nil)
	if err != nil {
		panic(err)
	}
	for iter.First(); iter.Valid(); iter.Next() {
		data[string(iter.Key())] = string(iter.Value())
	}
	iter.Close()
	return data
}

func (s *Store) Clear() {
	iter, err := s.NewIter(nil)
	if err != nil {
		panic(err)
	}
	for iter.First(); iter.Valid(); iter.Next() {
		if err := s.DB.Delete(iter.Key(), pebble.Sync); err != nil {
			panic(err)
		}
	}
	_ = iter.Close()
}

func (s *Store) Close() error {
	return s.DB.Close()
}
