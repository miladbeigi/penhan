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
