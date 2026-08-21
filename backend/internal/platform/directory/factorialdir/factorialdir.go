// Package factorialdir adapts the Factorial HR API to the directory contract.
// Every rule specific to that source lives here.
package factorialdir

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/sciacco/mrsmith/internal/platform/directory"
	"github.com/sciacco/mrsmith/pkg/factorial"
)

type Adapter struct {
	client *factorial.Client
}

func New(client *factorial.Client) *Adapter {
	if client == nil {
		return nil
	}
	return &Adapter{client: client}
}

func (a *Adapter) Snapshot(ctx context.Context) (directory.Snapshot, error) {
	employees, err := a.client.Employees.Employees.All(ctx, &factorial.EmployeesEmployeesListParams{})
	if err != nil {
		return directory.Snapshot{}, err
	}
	teams, err := a.client.Teams.Teams.All(ctx, &factorial.TeamsTeamsListParams{})
	if err != nil {
		return directory.Snapshot{}, err
	}
	return buildSnapshot(employees, teams, time.Now()), nil
}

// buildSnapshot normalizes the raw Factorial payload:
//   - one person per login email (a rehire has several records: the active one
//     wins, otherwise the most recently terminated);
//   - active people without a login email are dropped and counted, since the
//     login email is the only join key against local records;
//   - team membership is restricted to active people, because Factorial keeps
//     the memberships of terminated employees.
func buildSnapshot(employees []factorial.EmployeesEmployee, teams []factorial.TeamsTeam, fetchedAt time.Time) directory.Snapshot {
	snapshot := directory.Snapshot{FetchedAt: fetchedAt}

	byEmail := map[string][]directory.Person{}
	for _, e := range employees {
		externalID := strings.TrimSpace(deref(e.ID))
		if externalID == "" {
			continue
		}
		active := e.Active != nil && *e.Active
		email := normalizeEmail(deref(e.LoginEmail))
		if email == "" {
			if active {
				snapshot.SkippedNoLoginEmail++
			}
			continue
		}
		first, last := splitName(deref(e.FirstName), deref(e.LastName), deref(e.FullName))
		byEmail[email] = append(byEmail[email], directory.Person{
			ExternalID:   externalID,
			FirstName:    first,
			LastName:     last,
			LoginEmail:   email,
			Active:       active,
			TerminatedOn: dateOrNil(e.TerminatedOn),
		})
	}

	activeIDs := map[string]struct{}{}
	people := make([]directory.Person, 0, len(byEmail))
	for _, records := range byEmail {
		winner := pickPerson(records)
		snapshot.SkippedDuplicate += len(records) - 1
		people = append(people, winner)
		if winner.Active {
			activeIDs[winner.ExternalID] = struct{}{}
		}
	}
	sort.Slice(people, func(i, j int) bool { return people[i].LoginEmail < people[j].LoginEmail })
	snapshot.People = people

	rows := make([]directory.Team, 0, len(teams))
	for _, t := range teams {
		externalID := strings.TrimSpace(deref(t.ID))
		name := strings.TrimSpace(deref(t.Name))
		if externalID == "" || name == "" {
			continue
		}
		members, skippedMembers := filterActive(t.EmployeeIDs, activeIDs)
		leads, skippedLeads := filterActive(t.LeadIDs, activeIDs)
		snapshot.SkippedMembership += skippedMembers + skippedLeads
		rows = append(rows, directory.Team{
			ExternalID:        externalID,
			Name:              name,
			MemberExternalIDs: members,
			LeadExternalIDs:   leads,
		})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Name < rows[j].Name })
	snapshot.Teams = rows

	return snapshot
}

func pickPerson(records []directory.Person) directory.Person {
	best := records[0]
	for _, candidate := range records[1:] {
		if morePreferred(candidate, best) {
			best = candidate
		}
	}
	return best
}

func morePreferred(candidate directory.Person, current directory.Person) bool {
	if candidate.Active != current.Active {
		return candidate.Active
	}
	switch {
	case candidate.TerminatedOn == nil && current.TerminatedOn == nil:
		return candidate.ExternalID > current.ExternalID
	case candidate.TerminatedOn == nil:
		return true
	case current.TerminatedOn == nil:
		return false
	case candidate.TerminatedOn.Equal(*current.TerminatedOn):
		return candidate.ExternalID > current.ExternalID
	default:
		return candidate.TerminatedOn.After(*current.TerminatedOn)
	}
}

func filterActive(ids []string, activeIDs map[string]struct{}) ([]string, int) {
	kept := make([]string, 0, len(ids))
	skipped := 0
	seen := map[string]struct{}{}
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := activeIDs[id]; !ok {
			skipped++
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		kept = append(kept, id)
	}
	sort.Strings(kept)
	return kept, skipped
}

func splitName(first, last, full string) (string, string) {
	first = strings.Join(strings.Fields(first), " ")
	last = strings.Join(strings.Fields(last), " ")
	if first != "" && last != "" {
		return first, last
	}
	parts := strings.Fields(full)
	if len(parts) < 2 {
		return first, last
	}
	if first == "" {
		first = parts[0]
	}
	if last == "" {
		last = strings.Join(parts[1:], " ")
	}
	return first, last
}

func dateOrNil(date *factorial.Date) *time.Time {
	if date == nil || date.Time.IsZero() {
		return nil
	}
	value := date.Time
	return &value
}

func normalizeEmail(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func deref(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
