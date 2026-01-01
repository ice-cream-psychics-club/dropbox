package store

import (
	"errors"
	"sync"
)

var ErrNotFound = errors.New("no cursor for account - yet")

type MemoryStore struct {
	Cursors map[string]string
	sync.RWMutex
}

func (s *MemoryStore) Set(account, cursor string) {
	s.Lock()
	defer s.Unlock()

	if s.Cursors == nil {
		s.Cursors = make(map[string]string)
	}
	
	s.Cursors[account] = cursor
}

func (s *MemoryStore) Get(account string) (string, error) {
	s.RLock()
	defer s.RUnlock()

	if s.Cursors == nil {
		s.Cursors = make(map[string]string)
	}

	cursor, ok := s.Cursors[account]
	if !ok {
		return "", ErrNotFound
	}

	return cursor, nil
}
