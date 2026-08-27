package state

import (
	"path/filepath"
	"testing"
)

func TestStateLoadSave(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")

	// Create new state
	s := NewState()
	s.UpdateHash("db/password", "sha256:abc123")

	// Save
	if err := Save(s, statePath); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Load
	loaded, err := Load(statePath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Verify
	entry, ok := loaded.Secrets["db/password"]
	if !ok {
		t.Fatal("Secret not found in state")
	}

	if entry.LocalHash != "sha256:abc123" {
		t.Errorf("LocalHash = %q, want %q", entry.LocalHash, "sha256:abc123")
	}
}

func TestStatusDetection(t *testing.T) {
	s := NewState()

	// New secret
	s.UpdateHash("db/password", "sha256:abc123")
	entry := s.Secrets["db/password"]
	if entry.Status != "local_changed" {
		t.Errorf("Status = %q, want %q", entry.Status, "local_changed")
	}

	// Mark synced
	s.MarkSynced("db/password")
	entry = s.Secrets["db/password"]
	if entry.Status != "synced" {
		t.Errorf("Status = %q, want %q", entry.Status, "synced")
	}

	// Remote changed
	s.UpdateRemoteHash("db/password", "sha256:def456")
	entry = s.Secrets["db/password"]
	if entry.Status != "remote_changed" {
		t.Errorf("Status = %q, want %q", entry.Status, "remote_changed")
	}
}
