package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Job struct {
	Key         string      `json:"key"`
	Board       string      `json:"board"`
	Company     string      `json:"company"`
	Title       string      `json:"title"`
	Location    string      `json:"location"`
	URL         string      `json:"url"`
	Category    Category    `json:"category"`
	Eligibility Eligibility `json:"eligibility"`
	Entry       bool        `json:"entry"`
	FirstSeen   time.Time   `json:"first_seen"`
	LastSeen    time.Time   `json:"last_seen"`
	Open        bool        `json:"open"`
}

// Store keeps every matching job in one JSON file. Plenty for thousands of rows;
// swap for SQLite later if it ever matters.
type Store struct {
	mu   sync.Mutex
	path string
	Jobs map[string]*Job
}

func LoadStore(path string) (*Store, error) {
	s := &Store{path: path, Jobs: map[string]*Job{}}
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, &s.Jobs); err != nil {
		return nil, err
	}
	return s, nil
}

// Save writes to a temp file and renames it, so a crash can't leave a half-written file.
func (s *Store) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, err := json.MarshalIndent(s.Jobs, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

// ApplyScan merges one board's latest matches into the store.
// Returns how many were never seen before. Jobs that disappeared are marked closed.
func (s *Store) ApplyScan(board string, matches []Job, now time.Time) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	seen := map[string]bool{}
	added := 0
	for _, m := range matches {
		seen[m.Key] = true
		if old, ok := s.Jobs[m.Key]; ok {
			old.Title, old.Location, old.URL = m.Title, m.Location, m.URL
			old.Category, old.Eligibility = m.Category, m.Eligibility
			old.Entry = m.Entry
			old.LastSeen, old.Open = now, true
			continue
		}
		cp := m
		cp.FirstSeen, cp.LastSeen, cp.Open = now, now, true
		s.Jobs[m.Key] = &cp
		added++
	}
	for k, j := range s.Jobs {
		if j.Board == board && j.Open && !seen[k] {
			j.Open = false
		}
	}
	return added
}

// OpenJobs returns copies of currently open jobs, best matches first.
func (s *Store) OpenJobs() []Job {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []Job
	for _, j := range s.Jobs {
		if j.Open {
			out = append(out, *j)
		}
	}
	sortJobs(out)
	return out
}