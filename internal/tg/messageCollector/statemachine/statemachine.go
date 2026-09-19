package statemachine

import (
	"sync"
)

type StateMachine interface {
	Set(userID int64, state int8) error
	Get(userID int64) (int8, error)
	Delete(userID int64)
}
type Storage struct {
	mu    sync.RWMutex
	store map[int64]int8
}

func New() *Storage {
	return &Storage{
		store: make(map[int64]int8),
	}
}
func (s *Storage) Set(userID int64, state int8) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.store[userID] = state
	return nil
}
func (s *Storage) Get(userID int64) (int8, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	state, ok := s.store[userID]
	if !ok {
		return -1, nil
	}
	return state, nil
}
func (s *Storage) Delete(userID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.store, userID)
}
