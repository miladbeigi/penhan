package state

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// State manages the state of secrets for tracking changes.
type State struct {
	Version int                    `json:"version"`
	Secrets map[string]SecretEntry `json:"secrets"`
}

// SecretEntry represents the state of a single secret.
type SecretEntry struct {
	LocalHash  string    `json:"local_hash"`
	RemoteHash string    `json:"remote_hash"`
	LastSynced time.Time `json:"last_synced"`
	Status     string    `json:"status"`
}

// NewState creates a new empty state.
func NewState() *State {
	return &State{
		Version: 1,
		Secrets: make(map[string]SecretEntry),
	}
}

// Load reads state from a JSON file.
func Load(path string) (*State, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read state file: %w", err)
	}

	s := &State{}
	if err := json.Unmarshal(data, s); err != nil {
		return nil, fmt.Errorf("parse state: %w", err)
	}

	return s, nil
}

// Save writes state to a JSON file.
func Save(s *State, path string) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}

	return os.WriteFile(path, data, 0o644)
}

// UpdateHash sets the local hash for a secret and marks status as local_changed.
func (s *State) UpdateHash(path, hash string) {
	entry, ok := s.Secrets[path]
	if !ok {
		entry = SecretEntry{}
	}

	entry.LocalHash = hash
	entry.Status = "local_changed"
	s.Secrets[path] = entry
}

// UpdateRemoteHash sets the remote hash for a secret and marks status as remote_changed.
func (s *State) UpdateRemoteHash(path, hash string) {
	entry, ok := s.Secrets[path]
	if !ok {
		entry = SecretEntry{}
	}

	entry.RemoteHash = hash
	entry.Status = "remote_changed"
	s.Secrets[path] = entry
}

// MarkSynced marks a secret as synced and copies remote hash to local.
func (s *State) MarkSynced(path string) {
	entry, ok := s.Secrets[path]
	if !ok {
		return
	}

	entry.LastSynced = time.Now()
	entry.Status = "synced"
	entry.LocalHash = entry.RemoteHash
	s.Secrets[path] = entry
}
