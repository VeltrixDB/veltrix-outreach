package main

import (
	"encoding/json"
	"os"
	"time"
)

const cooldownDays = 60

type SeenStore struct {
	Seen map[string]time.Time `json:"seen"`
}

func newSeenStore() *SeenStore {
	return &SeenStore{Seen: make(map[string]time.Time)}
}

func loadSeen(path string) (*SeenStore, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return newSeenStore(), nil
	}
	var s SeenStore
	if err := json.Unmarshal(data, &s); err != nil {
		return newSeenStore(), nil
	}
	if s.Seen == nil {
		s.Seen = make(map[string]time.Time)
	}
	// Prune entries older than cooldown so people re-appear after 60 days.
	cutoff := time.Now().AddDate(0, 0, -cooldownDays)
	for handle, t := range s.Seen {
		if t.Before(cutoff) {
			delete(s.Seen, handle)
		}
	}
	return &s, nil
}

func (s *SeenStore) Has(handle string) bool {
	t, ok := s.Seen[handle]
	if !ok {
		return false
	}
	return time.Since(t) < cooldownDays*24*time.Hour
}

func (s *SeenStore) Mark(handle string) {
	s.Seen[handle] = time.Now()
}

func (s *SeenStore) Save(path string) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
