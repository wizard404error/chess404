package match

import (
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
)

type MemoryMatchStore struct {
	mu       sync.RWMutex
	states   map[string][]byte
	secrets  map[string]map[string]string
	history  map[string][]byte
	events   map[string][]byte
	presence map[string][]byte
	seqs     map[string]*int64
}

func NewMemoryMatchStore() *MemoryMatchStore {
	return &MemoryMatchStore{
		states:   make(map[string][]byte),
		secrets:  make(map[string]map[string]string),
		history:  make(map[string][]byte),
		events:   make(map[string][]byte),
		presence: make(map[string][]byte),
		seqs:     make(map[string]*int64),
	}
}

func (s *MemoryMatchStore) getSeqPtr(matchID string) *int64 {
	if ptr, ok := s.seqs[matchID]; ok {
		return ptr
	}
	var val int64
	s.seqs[matchID] = &val
	return s.seqs[matchID]
}

func (s *MemoryMatchStore) SaveState(matchID string, snapshot any) error {
	data, err := json.Marshal(snapshot)
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.states[matchID] = data
	return nil
}

func (s *MemoryMatchStore) LoadState(matchID string, into any) error {
	s.mu.RLock()
	data, ok := s.states[matchID]
	s.mu.RUnlock()
	if !ok {
		return fmt.Errorf("match not found")
	}
	return json.Unmarshal(data, into)
}

func (s *MemoryMatchStore) SaveSecrets(matchID string, white, black string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.secrets[matchID] = map[string]string{"white": white, "black": black}
	return nil
}

func (s *MemoryMatchStore) LoadSecrets(matchID string) (white, black string, err error) {
	s.mu.RLock()
	secrets, ok := s.secrets[matchID]
	s.mu.RUnlock()
	if !ok {
		return "", "", fmt.Errorf("secrets not found")
	}
	return secrets["white"], secrets["black"], nil
}

func (s *MemoryMatchStore) SaveHistory(matchID string, history []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.history[matchID] = history
	return nil
}

func (s *MemoryMatchStore) LoadHistory(matchID string) ([]byte, error) {
	s.mu.RLock()
	data, ok := s.history[matchID]
	s.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("history not found")
	}
	return data, nil
}

func (s *MemoryMatchStore) SaveEvents(matchID string, events []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events[matchID] = events
	return nil
}

func (s *MemoryMatchStore) LoadEvents(matchID string) ([]byte, error) {
	s.mu.RLock()
	data, ok := s.events[matchID]
	s.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("events not found")
	}
	return data, nil
}

func (s *MemoryMatchStore) SavePresence(matchID string, presence []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.presence[matchID] = presence
	return nil
}

func (s *MemoryMatchStore) LoadPresence(matchID string) ([]byte, error) {
	s.mu.RLock()
	data, ok := s.presence[matchID]
	s.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("presence not found")
	}
	return data, nil
}

func (s *MemoryMatchStore) IncSeq(matchID string) (int64, error) {
	s.mu.Lock()
	ptr := s.getSeqPtr(matchID)
	s.mu.Unlock()
	return atomic.AddInt64(ptr, 1), nil
}

func (s *MemoryMatchStore) LoadSeq(matchID string) (int64, error) {
	s.mu.RLock()
	ptr, ok := s.seqs[matchID]
	s.mu.RUnlock()
	if !ok {
		return 0, nil
	}
	return atomic.LoadInt64(ptr), nil
}

func (s *MemoryMatchStore) DeleteMatch(matchID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.states, matchID)
	delete(s.secrets, matchID)
	delete(s.history, matchID)
	delete(s.events, matchID)
	delete(s.presence, matchID)
	delete(s.seqs, matchID)
	return nil
}

func (s *MemoryMatchStore) ListActiveMatchIDs() ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ids := make([]string, 0, len(s.states))
	for id := range s.states {
		ids = append(ids, id)
	}
	return ids, nil
}

func (s *MemoryMatchStore) Ping() error { return nil }

func (s *MemoryMatchStore) Close() error { return nil }
