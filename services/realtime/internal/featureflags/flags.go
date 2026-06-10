package featureflags

import (
	"encoding/json"
	"os"
	"sync"
)

type Feature struct {
	Name        string `json:"name"`
	Enabled     bool   `json:"enabled"`
	Description string `json:"description"`
	Rollout     int    `json:"rollout"`
}

type Store struct {
	mu      sync.RWMutex
	features map[string]Feature
}

func NewStore() *Store {
	s := &Store{
		features: make(map[string]Feature),
	}
	s.loadFromEnv()
	return s
}

func (s *Store) loadFromEnv() {
	flagsJSON := os.Getenv("FEATURE_FLAGS")
	if flagsJSON == "" {
		return
	}

	var features []Feature
	if err := json.Unmarshal([]byte(flagsJSON), &features); err != nil {
		return
	}

	for _, f := range features {
		s.features[f.Name] = f
	}
}

func (s *Store) IsEnabled(name string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	f, ok := s.features[name]
	if !ok {
		return false
	}
	return f.Enabled
}

func (s *Store) GetRollout(name string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	f, ok := s.features[name]
	if !ok {
		return 0
	}
	return f.Rollout
}

func (s *Store) Set(name string, enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	f := s.features[name]
	f.Name = name
	f.Enabled = enabled
	s.features[name] = f
}

func (s *Store) SetRollout(name string, rollout int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	f := s.features[name]
	f.Name = name
	f.Rollout = rollout
	s.features[name] = f
}

func (s *Store) List() []Feature {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []Feature
	for _, f := range s.features {
		result = append(result, f)
	}
	return result
}

func (s *Store) Evaluate(name string, userID string) bool {
	if !s.IsEnabled(name) {
		return false
	}

	rollout := s.GetRollout(name)
	if rollout <= 0 {
		return true
	}
	if rollout >= 100 {
		return true
	}

	hash := 0
	for _, c := range userID {
		hash = (hash*31 + int(c)) & 0x7fffffff
	}
	return (hash % 100) < rollout
}
