package state

import (
	"testing"
)

func TestGeneratePlan(t *testing.T) {
	s := NewState()

	// Add secrets
	s.UpdateHash("db/password", "sha256:abc123")
	s.UpdateHash("api/key", "sha256:def456")

	// Simulate remote state
	remoteState := NewState()
	remoteState.UpdateHash("db/password", "sha256:abc123") // Same
	remoteState.UpdateHash("api/key", "sha256:xyz789")     // Different

	plan := GeneratePlan(s, remoteState)

	if plan.Add != 0 {
		t.Errorf("Add = %d, want 0", plan.Add)
	}

	if plan.Update != 1 {
		t.Errorf("Update = %d, want 1", plan.Update)
	}

	if plan.Conflict != 0 {
		t.Errorf("Conflict = %d, want 0", plan.Conflict)
	}
}

// A secret changed only on the remote since the last sync must be a conflict:
// pushing would silently overwrite the remote edit.
func TestGeneratePlanRemoteOnlyChangeIsConflict(t *testing.T) {
	local := NewState()
	local.Secrets["db"] = SecretEntry{LocalHash: "sha256:aaa", Status: "synced"}

	remote := NewState()
	remote.Secrets["db"] = SecretEntry{LocalHash: "sha256:bbb", Status: "remote_changed"}

	plan := GeneratePlan(local, remote)

	if plan.Conflict != 1 {
		t.Errorf("Conflict = %d, want 1", plan.Conflict)
	}
	if plan.Update != 0 {
		t.Errorf("Update = %d, want 0", plan.Update)
	}
}

// Both sides changed but ended up with identical content: nothing to do.
func TestGeneratePlanIdenticalContentIsNotConflict(t *testing.T) {
	local := NewState()
	local.Secrets["db"] = SecretEntry{LocalHash: "sha256:same", Status: "local_changed"}

	remote := NewState()
	remote.Secrets["db"] = SecretEntry{LocalHash: "sha256:same", Status: "remote_changed"}

	plan := GeneratePlan(local, remote)

	if plan.Conflict != 0 {
		t.Errorf("Conflict = %d, want 0", plan.Conflict)
	}
	if plan.Update != 0 {
		t.Errorf("Update = %d, want 0", plan.Update)
	}
}

func TestGeneratePlanNewSecrets(t *testing.T) {
	s := NewState()
	s.UpdateHash("db/password", "sha256:abc123")
	s.UpdateHash("api/key", "sha256:def456")

	remoteState := NewState()
	remoteState.UpdateHash("db/password", "sha256:abc123")

	plan := GeneratePlan(s, remoteState)

	if plan.Add != 1 {
		t.Errorf("Add = %d, want 1", plan.Add)
	}
}
