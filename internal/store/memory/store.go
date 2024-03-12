package memory

import (
	"errors"
	"sync"
)

var ErrNotFound = errors.New("key not found")

type Store struct {
	data map[string]string
	mu   sync.RWMutex
}

func New() *Store {
	return &Store{data: make(map[string]string)}
}

func (s *Store) Get(key string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.data[key]
	if !ok {
		return "", ErrNotFound
	}
	return v, nil
}

func (s *Store) Set(key string, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = value
	return nil
}
