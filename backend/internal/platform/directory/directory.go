// Package directory defines the contract for an external people directory
// (anagrafica + team) consumed by the mini-apps. The domain types here carry
// no provider-specific concept: every source rule lives in the adapter.
package directory

import (
	"context"
	"time"
)

// Person is a directory person identified by the external system id. LoginEmail
// is the join key against local records.
type Person struct {
	ExternalID   string
	FirstName    string
	LastName     string
	LoginEmail   string
	Active       bool
	TerminatedOn *time.Time
}

// Team is a directory team with its membership expressed by person external ids.
type Team struct {
	ExternalID        string
	Name              string
	LeadExternalIDs   []string
	MemberExternalIDs []string
}

// Snapshot is a point-in-time read of the directory. Skipped* carry the records
// the adapter dropped while normalizing, so callers can surface them.
type Snapshot struct {
	People              []Person
	Teams               []Team
	FetchedAt           time.Time
	SkippedNoLoginEmail int
	SkippedDuplicate    int
	SkippedMembership   int
}

// Provider reads the external directory.
type Provider interface {
	Snapshot(ctx context.Context) (Snapshot, error)
}
