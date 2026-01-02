package store

import (
	"errors"
	"sync"
)

var ErrNotFound = errors.New("no value for key")

type MemoryStore struct {
	values map[string]string
	sync.RWMutex
}

func (s *MemoryStore) Set(key, value string) {
	s.Lock()
	defer s.Unlock()

	if s.values == nil {
		s.values = make(map[string]string)
	}

	s.values[key] = value
}

func (s *MemoryStore) Get(key string) (string, error) {
	s.RLock()
	defer s.RUnlock()

	if s.values == nil {
		s.values = make(map[string]string)
	}

	value, ok := s.values[key]
	if !ok {
		return "", ErrNotFound
	}

	return value, nil
}

func (s *MemoryStore) Delete(key string) bool {
	s.Lock()
	defer s.Unlock()

	_, ok := s.values[key]
	delete(s.values, key)

	return ok
}
