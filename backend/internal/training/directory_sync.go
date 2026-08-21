package training

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/sciacco/mrsmith/internal/platform/directory"
)

const (
	directorySyncLockKey    = 7412330001
	directorySyncSampleSize = 20
)

const (
	directoryActionAdoptPerson     = "adopt_person"
	directoryActionCreatePerson    = "create_person"
	directoryActionUpdatePerson    = "update_person"
	directoryActionTerminatePerson = "terminate_person"
	directoryActionAdoptTeam       = "adopt_team"
	directoryActionCreateTeam      = "create_team"
	directoryActionRenameTeam      = "rename_team"
	directoryActionOpenMembership  = "open_membership"
	directoryActionCloseMembership = "close_membership"
	directoryActionSetLead         = "set_lead"
	directoryActionUnsetLead       = "unset_lead"
)

type directoryLocalMembership struct {
	ID     string
	TeamID string
	Lead   bool
}

type directoryLocalPerson struct {
	ID         string
	ExternalID string
	// Exempt: gestione manuale — la sincronizzazione ignora la persona.
	Exempt      bool
	FirstName   string
	LastName    string
	Email       string
	Status      string
	Memberships []directoryLocalMembership
}

type directoryLocalTeam struct {
	ID         string
	ExternalID string
	Code       string
	Name       string
}

type directoryLocalState struct {
	People []directoryLocalPerson
	Teams  []directoryLocalTeam
}

type directorySyncAction struct {
	Type             string     `json:"type"`
	Label            string     `json:"label"`
	Detail           string     `json:"detail,omitempty"`
	PersonID         string     `json:"personId,omitempty"`
	PersonExternalID string     `json:"personExternalId,omitempty"`
	TeamID           string     `json:"teamId,omitempty"`
	TeamExternalID   string     `json:"teamExternalId,omitempty"`
	MembershipID     string     `json:"membershipId,omitempty"`
	FirstName        string     `json:"firstName,omitempty"`
	LastName         string     `json:"lastName,omitempty"`
	Email            string     `json:"email,omitempty"`
	Status           string     `json:"status,omitempty"`
	TerminatedOn     *time.Time `json:"terminatedOn,omitempty"`
	TeamName         string     `json:"teamName,omitempty"`
	TeamCode         string     `json:"teamCode,omitempty"`
	Lead             bool       `json:"lead,omitempty"`
}

// DirectorySyncSourceStats reports what the adapter read and dropped.
type DirectorySyncSourceStats struct {
	People              int `json:"people"`
	Teams               int `json:"teams"`
	SkippedNoLoginEmail int `json:"skippedNoLoginEmail"`
	SkippedDuplicate    int `json:"skippedDuplicate"`
	SkippedMembership   int `json:"skippedMembership"`
	// ExemptLocal: persone locali in gestione manuale, ignorate dalla sync.
	ExemptLocal int `json:"exemptLocal"`
}

// DirectorySyncStats is the persisted summary of a run.
type DirectorySyncStats struct {
	Counts  map[string]int      `json:"counts"`
	Samples map[string][]string `json:"samples"`
	Total   int                 `json:"total"`
	Source  DirectorySyncSource `json:"source"`
}

type DirectorySyncSource struct {
	FetchedAt string                   `json:"fetchedAt,omitempty"`
	Stats     DirectorySyncSourceStats `json:"stats"`
}

// DirectorySyncRun is one reconciliation attempt, dry run included.
type DirectorySyncRun struct {
	ID         string              `json:"id"`
	StartedAt  string              `json:"startedAt"`
	FinishedAt string              `json:"finishedAt,omitempty"`
	Status     string              `json:"status"`
	DryRun     bool                `json:"dryRun"`
	Actor      string              `json:"actor"`
	Stats      *DirectorySyncStats `json:"stats,omitempty"`
	Error      string              `json:"error,omitempty"`
}

func directoryNormalizeName(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(value), " "))
}

func directoryPersonLabel(lastName, firstName, email string) string {
	name := strings.TrimSpace(strings.TrimSpace(lastName) + " " + strings.TrimSpace(firstName))
	if name != "" {
		return name
	}
	return email
}

func directoryTeamCode(name string, used map[string]bool) string {
	base := importCodeFromName(name, "TEAM")
	if len(base) > 40 {
		base = base[:40]
	}
	code := base
	for suffix := 2; used[code]; suffix++ {
		code = fmt.Sprintf("%s_%d", base, suffix)
	}
	used[code] = true
	return code
}

// diffDirectory compares the external snapshot with the local state and returns
// the ordered plan: teams first (later actions resolve team ids), then people,
// then memberships. No I/O.
func diffDirectory(snapshot directory.Snapshot, local directoryLocalState) []directorySyncAction {
	actions := make([]directorySyncAction, 0)

	localTeamByExternal := map[string]directoryLocalTeam{}
	localTeamByName := map[string]directoryLocalTeam{}
	usedCodes := map[string]bool{}
	for _, team := range local.Teams {
		usedCodes[team.Code] = true
		if team.ExternalID != "" {
			localTeamByExternal[team.ExternalID] = team
			continue
		}
		key := directoryNormalizeName(team.Name)
		if _, exists := localTeamByName[key]; !exists {
			localTeamByName[key] = team
		}
	}

	teamIDByExternal := map[string]string{}
	syncedTeamIDs := map[string]string{}
	for _, sourceTeam := range snapshot.Teams {
		if existing, ok := localTeamByExternal[sourceTeam.ExternalID]; ok {
			teamIDByExternal[sourceTeam.ExternalID] = existing.ID
			syncedTeamIDs[existing.ID] = sourceTeam.ExternalID
			if existing.Name != sourceTeam.Name {
				actions = append(actions, directorySyncAction{
					Type:           directoryActionRenameTeam,
					Label:          sourceTeam.Name,
					Detail:         existing.Name,
					TeamID:         existing.ID,
					TeamExternalID: sourceTeam.ExternalID,
					TeamName:       sourceTeam.Name,
				})
			}
			continue
		}
		key := directoryNormalizeName(sourceTeam.Name)
		if candidate, ok := localTeamByName[key]; ok {
			delete(localTeamByName, key)
			teamIDByExternal[sourceTeam.ExternalID] = candidate.ID
			syncedTeamIDs[candidate.ID] = sourceTeam.ExternalID
			actions = append(actions, directorySyncAction{
				Type:           directoryActionAdoptTeam,
				Label:          sourceTeam.Name,
				TeamID:         candidate.ID,
				TeamExternalID: sourceTeam.ExternalID,
				TeamName:       sourceTeam.Name,
			})
			if candidate.Name != sourceTeam.Name {
				actions = append(actions, directorySyncAction{
					Type:           directoryActionRenameTeam,
					Label:          sourceTeam.Name,
					Detail:         candidate.Name,
					TeamID:         candidate.ID,
					TeamExternalID: sourceTeam.ExternalID,
					TeamName:       sourceTeam.Name,
				})
			}
			continue
		}
		actions = append(actions, directorySyncAction{
			Type:           directoryActionCreateTeam,
			Label:          sourceTeam.Name,
			TeamExternalID: sourceTeam.ExternalID,
			TeamName:       sourceTeam.Name,
			TeamCode:       directoryTeamCode(sourceTeam.Name, usedCodes),
		})
	}

	localPersonByExternal := map[string]directoryLocalPerson{}
	localPersonByEmail := map[string]directoryLocalPerson{}
	for _, person := range local.People {
		if person.ExternalID != "" {
			localPersonByExternal[person.ExternalID] = person
		}
		if _, exists := localPersonByEmail[person.Email]; !exists {
			localPersonByEmail[person.Email] = person
		}
	}

	handledLocalIDs := map[string]bool{}
	membershipOwners := make([]directorySyncAction, 0)
	activeSourceByExternal := map[string]directory.Person{}
	localByExternalAfterMatch := map[string]directoryLocalPerson{}

	for _, sourcePerson := range snapshot.People {
		label := directoryPersonLabel(sourcePerson.LastName, sourcePerson.FirstName, sourcePerson.LoginEmail)
		matched, ok := localPersonByExternal[sourcePerson.ExternalID]
		if !ok {
			if candidate, okEmail := localPersonByEmail[sourcePerson.LoginEmail]; okEmail && candidate.ExternalID == "" {
				matched, ok = candidate, true
				if !candidate.Exempt {
					actions = append(actions, directorySyncAction{
						Type:             directoryActionAdoptPerson,
						Label:            label,
						PersonID:         candidate.ID,
						PersonExternalID: sourcePerson.ExternalID,
						Email:            sourcePerson.LoginEmail,
					})
				}
			}
		}
		if !ok {
			if !sourcePerson.Active {
				continue
			}
			actions = append(actions, directorySyncAction{
				Type:             directoryActionCreatePerson,
				Label:            label,
				PersonExternalID: sourcePerson.ExternalID,
				FirstName:        sourcePerson.FirstName,
				LastName:         sourcePerson.LastName,
				Email:            sourcePerson.LoginEmail,
				Status:           "active",
			})
			activeSourceByExternal[sourcePerson.ExternalID] = sourcePerson
			continue
		}

		handledLocalIDs[matched.ID] = true
		// Gestione manuale: la persona resta agganciata (niente doppioni da
		// create_person) ma la sincronizzazione non la tocca — nessun
		// aggiornamento, nessuna cessazione, appartenenze intatte.
		if matched.Exempt {
			continue
		}
		localByExternalAfterMatch[sourcePerson.ExternalID] = matched

		if !sourcePerson.Active {
			if matched.Status != "terminated" {
				actions = append(actions, directorySyncAction{
					Type:             directoryActionTerminatePerson,
					Label:            label,
					PersonID:         matched.ID,
					PersonExternalID: sourcePerson.ExternalID,
					TerminatedOn:     sourcePerson.TerminatedOn,
				})
				continue
			}
			if directoryIdentityChanged(matched, sourcePerson) {
				actions = append(actions, directoryUpdatePersonAction(label, matched, sourcePerson, ""))
			}
			continue
		}

		activeSourceByExternal[sourcePerson.ExternalID] = sourcePerson
		status := ""
		if matched.Status == "terminated" {
			status = "active"
		}
		if directoryIdentityChanged(matched, sourcePerson) || status != "" {
			actions = append(actions, directoryUpdatePersonAction(label, matched, sourcePerson, status))
		}
	}

	for _, person := range local.People {
		if person.ExternalID == "" || person.Exempt || handledLocalIDs[person.ID] || person.Status == "terminated" {
			continue
		}
		actions = append(actions, directorySyncAction{
			Type:             directoryActionTerminatePerson,
			Label:            directoryPersonLabel(person.LastName, person.FirstName, person.Email),
			PersonID:         person.ID,
			PersonExternalID: person.ExternalID,
		})
	}

	desiredLead := map[string]map[string]bool{}
	teamNameByExternal := map[string]string{}
	for _, sourceTeam := range snapshot.Teams {
		teamNameByExternal[sourceTeam.ExternalID] = sourceTeam.Name
		for _, memberID := range sourceTeam.MemberExternalIDs {
			if _, ok := activeSourceByExternal[memberID]; !ok {
				continue
			}
			if desiredLead[memberID] == nil {
				desiredLead[memberID] = map[string]bool{}
			}
			if _, exists := desiredLead[memberID][sourceTeam.ExternalID]; !exists {
				desiredLead[memberID][sourceTeam.ExternalID] = false
			}
		}
		for _, leadID := range sourceTeam.LeadExternalIDs {
			if _, ok := activeSourceByExternal[leadID]; !ok {
				continue
			}
			if desiredLead[leadID] == nil {
				desiredLead[leadID] = map[string]bool{}
			}
			desiredLead[leadID][sourceTeam.ExternalID] = true
		}
	}

	for _, sourcePerson := range snapshot.People {
		if _, ok := activeSourceByExternal[sourcePerson.ExternalID]; !ok {
			continue
		}
		label := directoryPersonLabel(sourcePerson.LastName, sourcePerson.FirstName, sourcePerson.LoginEmail)
		desired := desiredLead[sourcePerson.ExternalID]
		matched, hasLocal := localByExternalAfterMatch[sourcePerson.ExternalID]

		actualByTeamID := map[string]directoryLocalMembership{}
		if hasLocal {
			for _, membership := range matched.Memberships {
				if _, managed := syncedTeamIDs[membership.TeamID]; !managed {
					continue
				}
				actualByTeamID[membership.TeamID] = membership
			}
		}

		desiredTeamExternalIDs := make([]string, 0, len(desired))
		for teamExternalID := range desired {
			desiredTeamExternalIDs = append(desiredTeamExternalIDs, teamExternalID)
		}
		sort.Strings(desiredTeamExternalIDs)

		coveredTeamIDs := map[string]bool{}
		for _, teamExternalID := range desiredTeamExternalIDs {
			lead := desired[teamExternalID]
			teamID := teamIDByExternal[teamExternalID]
			if teamID != "" {
				if membership, ok := actualByTeamID[teamID]; ok {
					coveredTeamIDs[teamID] = true
					if membership.Lead == lead {
						continue
					}
					actionType := directoryActionUnsetLead
					if lead {
						actionType = directoryActionSetLead
					}
					membershipOwners = append(membershipOwners, directorySyncAction{
						Type:             actionType,
						Label:            label,
						PersonID:         matched.ID,
						PersonExternalID: sourcePerson.ExternalID,
						TeamID:           teamID,
						TeamExternalID:   teamExternalID,
						MembershipID:     membership.ID,
						TeamName:         teamNameByExternal[teamExternalID],
						Lead:             lead,
					})
					continue
				}
			}
			action := directorySyncAction{
				Type:             directoryActionOpenMembership,
				Label:            label,
				PersonExternalID: sourcePerson.ExternalID,
				TeamExternalID:   teamExternalID,
				TeamID:           teamID,
				TeamName:         teamNameByExternal[teamExternalID],
				Lead:             lead,
			}
			if hasLocal {
				action.PersonID = matched.ID
			}
			membershipOwners = append(membershipOwners, action)
		}

		if !hasLocal {
			continue
		}
		actualTeamIDs := make([]string, 0, len(actualByTeamID))
		for teamID := range actualByTeamID {
			actualTeamIDs = append(actualTeamIDs, teamID)
		}
		sort.Strings(actualTeamIDs)
		for _, teamID := range actualTeamIDs {
			if coveredTeamIDs[teamID] {
				continue
			}
			membershipOwners = append(membershipOwners, directorySyncAction{
				Type:             directoryActionCloseMembership,
				Label:            label,
				PersonID:         matched.ID,
				PersonExternalID: sourcePerson.ExternalID,
				TeamID:           teamID,
				TeamExternalID:   syncedTeamIDs[teamID],
				TeamName:         teamNameByExternal[syncedTeamIDs[teamID]],
				MembershipID:     actualByTeamID[teamID].ID,
			})
		}
	}

	return append(actions, membershipOwners...)
}

func directoryIdentityChanged(local directoryLocalPerson, source directory.Person) bool {
	if source.FirstName != "" && source.FirstName != local.FirstName {
		return true
	}
	if source.LastName != "" && source.LastName != local.LastName {
		return true
	}
	if source.LoginEmail != "" && source.LoginEmail != local.Email {
		return true
	}
	return false
}

func directoryUpdatePersonAction(label string, local directoryLocalPerson, source directory.Person, status string) directorySyncAction {
	action := directorySyncAction{
		Type:             directoryActionUpdatePerson,
		Label:            label,
		PersonID:         local.ID,
		PersonExternalID: source.ExternalID,
		FirstName:        local.FirstName,
		LastName:         local.LastName,
		Email:            local.Email,
		Status:           status,
	}
	if source.FirstName != "" {
		action.FirstName = source.FirstName
	}
	if source.LastName != "" {
		action.LastName = source.LastName
	}
	if source.LoginEmail != "" {
		action.Email = source.LoginEmail
	}
	if status == "active" {
		action.Detail = "riattivazione"
	}
	return action
}

func directorySyncStats(actions []directorySyncAction, snapshot directory.Snapshot, exemptLocal int) DirectorySyncStats {
	stats := DirectorySyncStats{
		Counts:  map[string]int{},
		Samples: map[string][]string{},
		Total:   len(actions),
		Source: DirectorySyncSource{
			Stats: DirectorySyncSourceStats{
				People:              len(snapshot.People),
				Teams:               len(snapshot.Teams),
				SkippedNoLoginEmail: snapshot.SkippedNoLoginEmail,
				SkippedDuplicate:    snapshot.SkippedDuplicate,
				SkippedMembership:   snapshot.SkippedMembership,
				ExemptLocal:         exemptLocal,
			},
		},
	}
	if !snapshot.FetchedAt.IsZero() {
		stats.Source.FetchedAt = snapshot.FetchedAt.UTC().Format(time.RFC3339)
	}
	for _, action := range actions {
		stats.Counts[action.Type]++
		if len(stats.Samples[action.Type]) >= directorySyncSampleSize {
			continue
		}
		entry := action.Label
		switch action.Type {
		case directoryActionOpenMembership, directoryActionCloseMembership, directoryActionSetLead, directoryActionUnsetLead:
			if action.TeamName != "" {
				entry = action.Label + " — " + action.TeamName
			}
		}
		stats.Samples[action.Type] = append(stats.Samples[action.Type], entry)
	}
	return stats
}

// RunDirectorySync reconciles the local anagrafica with the external directory.
// dryRun computes the plan without opening the write transaction; the run is
// recorded either way.
func (s *SQLStore) RunDirectorySync(ctx context.Context, provider directory.Provider, actor string, dryRun bool) (DirectorySyncRun, error) {
	if s == nil || s.db == nil {
		return DirectorySyncRun{}, serviceUnavailableError("training_database_not_configured", "database Training non configurato")
	}
	if provider == nil {
		return DirectorySyncRun{}, serviceUnavailableError("factorial_not_configured", "directory esterna non configurata")
	}

	// The advisory lock must live on the same session for its whole scope, so
	// the run holds a dedicated connection.
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return DirectorySyncRun{}, fmt.Errorf("acquire training directory sync connection: %w", err)
	}
	defer conn.Close()

	var locked bool
	if err := conn.QueryRowContext(ctx, `SELECT pg_try_advisory_lock($1)`, directorySyncLockKey).Scan(&locked); err != nil {
		return DirectorySyncRun{}, fmt.Errorf("lock training directory sync: %w", err)
	}
	if !locked {
		return DirectorySyncRun{}, conflictError("sync_already_running", "sincronizzazione anagrafica già in corso")
	}
	defer func() {
		_, _ = conn.ExecContext(context.WithoutCancel(ctx), `SELECT pg_advisory_unlock($1)`, directorySyncLockKey)
	}()

	run, err := s.startDirectorySyncRun(ctx, conn, actor, dryRun)
	if err != nil {
		return DirectorySyncRun{}, err
	}

	stats, syncErr := s.executeDirectorySync(ctx, conn, provider, dryRun)
	if syncErr != nil {
		if failErr := s.finishDirectorySyncRun(ctx, conn, run.ID, "failed", nil, syncErr.Error()); failErr != nil {
			return DirectorySyncRun{}, failErr
		}
		return DirectorySyncRun{}, syncErr
	}
	if err := s.finishDirectorySyncRun(ctx, conn, run.ID, "ok", &stats, ""); err != nil {
		return DirectorySyncRun{}, err
	}
	return s.directorySyncRun(ctx, conn, run.ID)
}

func (s *SQLStore) executeDirectorySync(ctx context.Context, conn *sql.Conn, provider directory.Provider, dryRun bool) (DirectorySyncStats, error) {
	snapshot, err := provider.Snapshot(ctx)
	if err != nil {
		return DirectorySyncStats{}, err
	}
	local, err := s.directoryLocalState(ctx, conn)
	if err != nil {
		return DirectorySyncStats{}, err
	}
	exemptLocal := 0
	for _, person := range local.People {
		if person.Exempt {
			exemptLocal++
		}
	}
	actions := diffDirectory(snapshot, local)
	stats := directorySyncStats(actions, snapshot, exemptLocal)
	if dryRun {
		return stats, nil
	}
	if err := s.withTx(ctx, func(tx *sql.Tx) error {
		return s.applyDirectoryPlan(ctx, tx, actions)
	}); err != nil {
		return DirectorySyncStats{}, err
	}
	return stats, nil
}

func (s *SQLStore) directoryLocalState(ctx context.Context, conn *sql.Conn) (directoryLocalState, error) {
	state := directoryLocalState{}

	teamRows, err := conn.QueryContext(ctx, `
SELECT id::text, COALESCE(external_id, ''), code, name
FROM training.team
ORDER BY name, code`)
	if err != nil {
		return state, fmt.Errorf("list training teams for directory sync: %w", err)
	}
	defer teamRows.Close()
	for teamRows.Next() {
		var team directoryLocalTeam
		if err := teamRows.Scan(&team.ID, &team.ExternalID, &team.Code, &team.Name); err != nil {
			return state, fmt.Errorf("scan training team for directory sync: %w", err)
		}
		state.Teams = append(state.Teams, team)
	}
	if err := teamRows.Err(); err != nil {
		return state, err
	}

	personRows, err := conn.QueryContext(ctx, `
SELECT
  e.id::text,
  COALESCE(e.external_id, ''),
  e.directory_exempt,
  e.first_name,
  e.last_name,
  e.email::text,
  e.status::text,
  COALESCE(
    (
      SELECT json_agg(json_build_object(
        'id', tm.id::text,
        'teamId', tm.team_id::text,
        'lead', tm.role = 'lead'
      ) ORDER BY tm.team_id)
      FROM training.team_membership tm
      WHERE tm.employee_id = e.id
        AND tm.start_date <= now()
        AND (tm.end_date IS NULL OR tm.end_date >= now())
    ),
    '[]'::json
  )::text
FROM training.employee e
ORDER BY e.email`)
	if err != nil {
		return state, fmt.Errorf("list training people for directory sync: %w", err)
	}
	defer personRows.Close()
	for personRows.Next() {
		var (
			person      directoryLocalPerson
			memberships string
		)
		if err := personRows.Scan(
			&person.ID,
			&person.ExternalID,
			&person.Exempt,
			&person.FirstName,
			&person.LastName,
			&person.Email,
			&person.Status,
			&memberships,
		); err != nil {
			return state, fmt.Errorf("scan training person for directory sync: %w", err)
		}
		var parsed []struct {
			ID     string `json:"id"`
			TeamID string `json:"teamId"`
			Lead   bool   `json:"lead"`
		}
		if err := json.Unmarshal([]byte(memberships), &parsed); err != nil {
			return state, fmt.Errorf("decode training memberships for directory sync: %w", err)
		}
		for _, item := range parsed {
			person.Memberships = append(person.Memberships, directoryLocalMembership{ID: item.ID, TeamID: item.TeamID, Lead: item.Lead})
		}
		state.People = append(state.People, person)
	}
	return state, personRows.Err()
}

func (s *SQLStore) applyDirectoryPlan(ctx context.Context, tx *sql.Tx, actions []directorySyncAction) error {
	teamIDByExternal := map[string]string{}
	personIDByExternal := map[string]string{}
	syncPrincipal := Principal{IsPeopleAdmin: true}

	for _, action := range actions {
		switch action.Type {
		case directoryActionAdoptTeam:
			if err := s.directoryExecAudited(ctx, tx, syncPrincipal, "team", action.TeamID, "directory_adopt", `
UPDATE training.team
SET external_id = $2
WHERE id = $1::uuid`, action.TeamID, action.TeamExternalID); err != nil {
				return err
			}
			teamIDByExternal[action.TeamExternalID] = action.TeamID
		case directoryActionRenameTeam:
			if err := s.directoryExecAudited(ctx, tx, syncPrincipal, "team", action.TeamID, "directory_rename", `
UPDATE training.team
SET name = $2
WHERE id = $1::uuid`, action.TeamID, action.TeamName); err != nil {
				return err
			}
			teamIDByExternal[action.TeamExternalID] = action.TeamID
		case directoryActionCreateTeam:
			var teamID string
			if err := tx.QueryRowContext(ctx, `
INSERT INTO training.team (code, name, external_id, is_active)
VALUES ($1, $2, $3, true)
RETURNING id::text`, action.TeamCode, action.TeamName, action.TeamExternalID).Scan(&teamID); err != nil {
				return fmt.Errorf("create training directory team: %w", err)
			}
			if err := s.directoryAudit(ctx, tx, syncPrincipal, "team", teamID, "directory_create", nil); err != nil {
				return err
			}
			teamIDByExternal[action.TeamExternalID] = teamID
		case directoryActionAdoptPerson:
			if err := s.directoryExecAudited(ctx, tx, syncPrincipal, "employee", action.PersonID, "directory_adopt", `
UPDATE training.employee
SET external_id = $2,
    updated_at = now()
WHERE id = $1::uuid`, action.PersonID, action.PersonExternalID); err != nil {
				return err
			}
			personIDByExternal[action.PersonExternalID] = action.PersonID
		case directoryActionCreatePerson:
			var personID string
			if err := tx.QueryRowContext(ctx, `
INSERT INTO training.employee (external_id, first_name, last_name, email, status)
VALUES ($1, $2, $3, $4, 'active'::training.employee_status)
RETURNING id::text`, action.PersonExternalID, action.FirstName, action.LastName, action.Email).Scan(&personID); err != nil {
				return fmt.Errorf("create training directory person: %w", err)
			}
			if err := s.directoryAudit(ctx, tx, syncPrincipal, "employee", personID, "directory_create", nil); err != nil {
				return err
			}
			personIDByExternal[action.PersonExternalID] = personID
		case directoryActionUpdatePerson:
			if err := s.directoryUpdatePerson(ctx, tx, syncPrincipal, action); err != nil {
				return err
			}
			personIDByExternal[action.PersonExternalID] = action.PersonID
		case directoryActionTerminatePerson:
			if err := s.directoryTerminatePerson(ctx, tx, syncPrincipal, action); err != nil {
				return err
			}
		case directoryActionOpenMembership:
			personID := action.PersonID
			if personID == "" {
				personID = personIDByExternal[action.PersonExternalID]
			}
			teamID := action.TeamID
			if teamID == "" {
				teamID = teamIDByExternal[action.TeamExternalID]
			}
			if personID == "" || teamID == "" {
				return fmt.Errorf("unresolved training membership target for %s", action.Label)
			}
			var membershipID string
			if err := tx.QueryRowContext(ctx, `
INSERT INTO training.team_membership (employee_id, team_id, role, start_date)
VALUES ($1::uuid, $2::uuid, $3, now())
ON CONFLICT (employee_id, team_id) WHERE end_date IS NULL
DO UPDATE SET role = EXCLUDED.role
RETURNING id::text`, personID, teamID, directoryMembershipRole(action.Lead)).Scan(&membershipID); err != nil {
				return fmt.Errorf("open training directory membership: %w", err)
			}
			if err := s.directoryAudit(ctx, tx, syncPrincipal, "team_membership", membershipID, "directory_open", nil); err != nil {
				return err
			}
		case directoryActionCloseMembership:
			if err := s.directoryExecAudited(ctx, tx, syncPrincipal, "team_membership", action.MembershipID, "directory_close", `
UPDATE training.team_membership
SET end_date = now()
WHERE id = $1::uuid`, action.MembershipID); err != nil {
				return err
			}
		case directoryActionSetLead, directoryActionUnsetLead:
			if err := s.directoryExecAudited(ctx, tx, syncPrincipal, "team_membership", action.MembershipID, "directory_lead", `
UPDATE training.team_membership
SET role = $2
WHERE id = $1::uuid`, action.MembershipID, directoryMembershipRole(action.Lead)); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown training directory action %q", action.Type)
		}
	}
	return nil
}

func directoryMembershipRole(lead bool) any {
	if lead {
		return "lead"
	}
	return nil
}

func (s *SQLStore) directoryUpdatePerson(ctx context.Context, tx *sql.Tx, principal Principal, action directorySyncAction) error {
	before, err := entitySnapshot(ctx, tx, "employee", action.PersonID)
	if err != nil {
		return err
	}
	// UNIQUE(email): una collisione fa fallire la tx e marca la run failed,
	// visibile in console.
	if _, err := tx.ExecContext(ctx, `
UPDATE training.employee
SET first_name = $2,
    last_name = $3,
    email = COALESCE(NULLIF($4, ''), email),
    status = COALESCE(NULLIF($5, '')::training.employee_status, status),
    termination_date = CASE WHEN NULLIF($5, '') = 'active' THEN NULL ELSE termination_date END,
    updated_at = now()
WHERE id = $1::uuid`, action.PersonID, action.FirstName, action.LastName, action.Email, action.Status); err != nil {
		return fmt.Errorf("update training directory person: %w", err)
	}
	return s.directoryAudit(ctx, tx, principal, "employee", action.PersonID, "directory_update", before)
}

func (s *SQLStore) directoryTerminatePerson(ctx context.Context, tx *sql.Tx, principal Principal, action directorySyncAction) error {
	before, err := entitySnapshot(ctx, tx, "employee", action.PersonID)
	if err != nil {
		return err
	}
	var terminatedOn any
	if action.TerminatedOn != nil {
		terminatedOn = action.TerminatedOn.Format("2006-01-02")
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE training.employee
SET status = 'terminated'::training.employee_status,
    termination_date = COALESCE($2::date, termination_date, CURRENT_DATE),
    updated_at = now()
WHERE id = $1::uuid`, action.PersonID, terminatedOn); err != nil {
		return fmt.Errorf("terminate training directory person: %w", err)
	}
	if err := s.directoryAudit(ctx, tx, principal, "employee", action.PersonID, "directory_terminate", before); err != nil {
		return err
	}
	rows, err := tx.QueryContext(ctx, `
UPDATE training.team_membership
SET end_date = now()
WHERE employee_id = $1::uuid
  AND end_date IS NULL
RETURNING id::text`, action.PersonID)
	if err != nil {
		return fmt.Errorf("close training directory memberships: %w", err)
	}
	defer rows.Close()
	membershipIDs := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return fmt.Errorf("scan closed training membership: %w", err)
		}
		membershipIDs = append(membershipIDs, id)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, id := range membershipIDs {
		if err := s.directoryAudit(ctx, tx, principal, "team_membership", id, "directory_close", nil); err != nil {
			return err
		}
	}
	return nil
}

func (s *SQLStore) directoryExecAudited(
	ctx context.Context,
	tx *sql.Tx,
	principal Principal,
	entityType string,
	entityID string,
	action string,
	stmt string,
	args ...any,
) error {
	before, err := entitySnapshot(ctx, tx, entityType, entityID)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, stmt, args...); err != nil {
		return fmt.Errorf("apply training directory %s: %w", action, err)
	}
	return s.directoryAudit(ctx, tx, principal, entityType, entityID, action, before)
}

func (s *SQLStore) directoryAudit(
	ctx context.Context,
	tx *sql.Tx,
	principal Principal,
	entityType string,
	entityID string,
	action string,
	before json.RawMessage,
) error {
	after, err := entitySnapshot(ctx, tx, entityType, entityID)
	if err != nil {
		return err
	}
	return s.audit(ctx, tx, principal, entityType, entityID, action, before, after)
}

func (s *SQLStore) startDirectorySyncRun(ctx context.Context, conn *sql.Conn, actor string, dryRun bool) (DirectorySyncRun, error) {
	var run DirectorySyncRun
	if err := conn.QueryRowContext(ctx, `
INSERT INTO training.directory_sync_run (status, dry_run, actor)
VALUES ('running', $1, $2)
RETURNING id::text`, dryRun, actor).Scan(&run.ID); err != nil {
		return DirectorySyncRun{}, fmt.Errorf("start training directory sync run: %w", err)
	}
	return run, nil
}

func (s *SQLStore) finishDirectorySyncRun(
	ctx context.Context,
	conn *sql.Conn,
	id string,
	status string,
	stats *DirectorySyncStats,
	syncError string,
) error {
	var payload any
	if stats != nil {
		raw, err := json.Marshal(stats)
		if err != nil {
			return fmt.Errorf("marshal training directory sync stats: %w", err)
		}
		payload = raw
	}
	if _, err := conn.ExecContext(context.WithoutCancel(ctx), `
UPDATE training.directory_sync_run
SET status = $2,
    finished_at = now(),
    stats = $3::jsonb,
    error = NULLIF($4, '')
WHERE id = $1::uuid`, id, status, payload, syncError); err != nil {
		return fmt.Errorf("finish training directory sync run: %w", err)
	}
	return nil
}

func (s *SQLStore) directorySyncRun(ctx context.Context, conn *sql.Conn, id string) (DirectorySyncRun, error) {
	row := conn.QueryRowContext(ctx, `
SELECT
  id::text,
  started_at,
  finished_at,
  status,
  dry_run,
  actor,
  COALESCE(stats::text, ''),
  COALESCE(error, '')
FROM training.directory_sync_run
WHERE id = $1::uuid`, id)
	run, err := scanDirectorySyncRun(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return DirectorySyncRun{}, notFoundError("directory_sync_run_not_found", "esecuzione non trovata")
		}
		return DirectorySyncRun{}, err
	}
	return run, nil
}

// ListDirectorySyncRuns returns the most recent runs, newest first.
func (s *SQLStore) ListDirectorySyncRuns(ctx context.Context, limit int) ([]DirectorySyncRun, error) {
	if limit <= 0 || limit > 200 {
		limit = 20
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT
  id::text,
  started_at,
  finished_at,
  status,
  dry_run,
  actor,
  COALESCE(stats::text, ''),
  COALESCE(error, '')
FROM training.directory_sync_run
ORDER BY started_at DESC
LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("list training directory sync runs: %w", err)
	}
	defer rows.Close()
	runs := make([]DirectorySyncRun, 0)
	for rows.Next() {
		run, err := scanDirectorySyncRun(rows)
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}

type directoryRunScanner interface {
	Scan(dest ...any) error
}

func scanDirectorySyncRun(scanner directoryRunScanner) (DirectorySyncRun, error) {
	var (
		run        DirectorySyncRun
		startedAt  time.Time
		finishedAt sql.NullTime
		stats      string
	)
	if err := scanner.Scan(
		&run.ID,
		&startedAt,
		&finishedAt,
		&run.Status,
		&run.DryRun,
		&run.Actor,
		&stats,
		&run.Error,
	); err != nil {
		return DirectorySyncRun{}, err
	}
	run.StartedAt = startedAt.UTC().Format(time.RFC3339)
	if finishedAt.Valid {
		run.FinishedAt = finishedAt.Time.UTC().Format(time.RFC3339)
	}
	if stats != "" {
		parsed := DirectorySyncStats{}
		if err := json.Unmarshal([]byte(stats), &parsed); err != nil {
			return DirectorySyncRun{}, fmt.Errorf("decode training directory sync stats: %w", err)
		}
		run.Stats = &parsed
	}
	return run, nil
}
