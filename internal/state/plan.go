package state

// Plan represents the sync plan between local and remote state.
type Plan struct {
	Add      int
	Update   int
	Delete   int
	Conflict int
	Changes  []PlanChange
}

// PlanChange represents a single change in the plan.
type PlanChange struct {
	Path   string
	Action string // "add", "update", "delete", "conflict"
}

// GeneratePlan compares local and remote state to determine sync operations.
func GeneratePlan(local, remote *State) *Plan {
	plan := &Plan{}

	// Check local secrets
	for path, localEntry := range local.Secrets {
		remoteEntry, exists := remote.Secrets[path]

		if !exists {
			plan.Add++
			plan.Changes = append(plan.Changes, PlanChange{
				Path:   path,
				Action: "add",
			})
			continue
		}

		// Identical content needs no action, whatever the statuses say.
		if localEntry.LocalHash == remoteEntry.LocalHash {
			continue
		}

		// The remote changed since the last sync: pushing would overwrite
		// someone else's edit, so it must be an explicit (--force) decision.
		if remoteEntry.Status == "remote_changed" {
			plan.Conflict++
			plan.Changes = append(plan.Changes, PlanChange{
				Path:   path,
				Action: "conflict",
			})
			continue
		}

		plan.Update++
		plan.Changes = append(plan.Changes, PlanChange{
			Path:   path,
			Action: "update",
		})
	}

	// Check for deletions
	for path := range remote.Secrets {
		if _, exists := local.Secrets[path]; !exists {
			plan.Delete++
			plan.Changes = append(plan.Changes, PlanChange{
				Path:   path,
				Action: "delete",
			})
		}
	}

	return plan
}
