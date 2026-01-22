// Package storage provides various storage implementations
package storage

import (
	"errors"
	"fmt"
	"sync"

	"univie.ac.at/efrsw/kvhash/internal/fs"
)

type KvStore interface {
	Insert(k uint, v string) error
	Delete(k uint) error
	Search(k uint) (string, error)
	Range(k1, k2 uint, proxied bool) ([]fs.KV, error)
}

type InMemoryStore struct {
	storage map[uint]string
	lock    sync.RWMutex
}

var ErrKeyNotFound = errors.New("key was not found in the store")

func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		storage: make(map[uint]string),
	}
}

func (s *InMemoryStore) Insert(k uint, v string) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.storage[k] = v
	return nil
}

func (s *InMemoryStore) Delete(k uint) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	delete(s.storage, k)
	return nil
}

func (s *InMemoryStore) Search(k uint) (string, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	if v, ok := s.storage[k]; ok {
		return v, nil
	}

	return "", fmt.Errorf("in-memory search: %w", ErrKeyNotFound)
}

func (s *InMemoryStore) Range(k1, k2 uint, proxied bool) ([]fs.KV, error) {
	s.lock.RLock()
	result := make([]fs.KV, 0)
	for k, v := range s.storage {
		if k >= k1 && k <= k2 {
			result = append(result, fs.KV{Key: k, Value: v})
		}
	}
	s.lock.RUnlock()

	return result, nil
}
