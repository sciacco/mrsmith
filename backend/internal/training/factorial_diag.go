package training

import (
	"context"
	"errors"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
	"github.com/sciacco/mrsmith/pkg/factorial"
)

// Read-only diagnostics over Factorial data (anagrafica, team, modulo
// formazione): no writes to Factorial, no local persistence.

type factorialStatusResponse struct {
	Configured bool   `json:"configured"`
	OK         bool   `json:"ok"`
	Error      string `json:"error,omitempty"`
}

type factorialPersonRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type factorialEmployeeRow struct {
	ID           string              `json:"id"`
	FullName     string              `json:"fullName"`
	Email        string              `json:"email,omitempty"`
	LoginEmail   string              `json:"loginEmail,omitempty"`
	Manager      *factorialPersonRef `json:"manager,omitempty"`
	Active       bool                `json:"active"`
	TerminatedOn *factorial.Date     `json:"terminatedOn,omitempty"`
}

type factorialTeamRow struct {
	ID          string               `json:"id"`
	Name        string               `json:"name"`
	Description string               `json:"description,omitempty"`
	Members     []factorialPersonRef `json:"members"`
	Leads       []factorialPersonRef `json:"leads"`
}

type factorialTrainingRow struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Code       string   `json:"code,omitempty"`
	Year       *int64   `json:"year,omitempty"`
	Status     string   `json:"status,omitempty"`
	Catalog    bool     `json:"catalog"`
	External   bool     `json:"external"`
	Provider   string   `json:"provider,omitempty"`
	Cost       string   `json:"cost,omitempty"`
	Categories []string `json:"categories,omitempty"`
}

type factorialMembershipRow struct {
	ID           string          `json:"id"`
	AccessID     string          `json:"accessId,omitempty"`
	TrainingID   string          `json:"trainingId,omitempty"`
	EmployeeID   string          `json:"employeeId"`
	EmployeeName string          `json:"employeeName,omitempty"`
	Status       string          `json:"status,omitempty"`
	DueDate      *factorial.Date `json:"dueDate,omitempty"`
	CompletedAt  *factorial.Time `json:"completedAt,omitempty"`
}

type factorialClassRow struct {
	ID                        string          `json:"id"`
	TrainingID                string          `json:"trainingId,omitempty"`
	Name                      string          `json:"name,omitempty"`
	Description               string          `json:"description,omitempty"`
	StartDate                 *factorial.Date `json:"startDate,omitempty"`
	EndDate                   *factorial.Date `json:"endDate,omitempty"`
	Cost                      string          `json:"cost,omitempty"`
	IndirectCost              string          `json:"indirectCost,omitempty"`
	SalaryCost                string          `json:"salaryCost,omitempty"`
	SubsidizedCost            string          `json:"subsidizedCost,omitempty"`
	GrossCost                 string          `json:"grossCost,omitempty"`
	NetCost                   string          `json:"netCost,omitempty"`
	Currency                  string          `json:"currency,omitempty"`
	PaymentStatus             string          `json:"paymentStatus,omitempty"`
	CompletedAttendancesCount *int64          `json:"completedAttendancesCount,omitempty"`
	TotalAttendancesCount     *int64          `json:"totalAttendancesCount,omitempty"`
}

type factorialSessionRow struct {
	ID              string          `json:"id"`
	TrainingID      string          `json:"trainingId,omitempty"`
	TrainingClassID string          `json:"trainingClassId,omitempty"`
	Name            string          `json:"name,omitempty"`
	Description     string          `json:"description,omitempty"`
	StartsAt        *factorial.Time `json:"startsAt,omitempty"`
	EndsAt          *factorial.Time `json:"endsAt,omitempty"`
	DueDate         *factorial.Date `json:"dueDate,omitempty"`
	Duration        string          `json:"duration,omitempty"`
	Modality        string          `json:"modality,omitempty"`
	Schedule        string          `json:"schedule,omitempty"`
	Location        string          `json:"location,omitempty"`
	Status          string          `json:"status,omitempty"`
	ParentID        string          `json:"parentId,omitempty"`
}

type factorialAttendanceRow struct {
	ID                        string `json:"id"`
	SessionAccessMembershipID string `json:"sessionAccessMembershipId,omitempty"`
	AccessID                  string `json:"accessId,omitempty"`
	EmployeeID                string `json:"employeeId,omitempty"`
	Status                    string `json:"status,omitempty"`
	CompletedDuration         string `json:"completedDuration,omitempty"`
}

type factorialSessionParticipantRow struct {
	SessionAccessMembershipID string                   `json:"sessionAccessMembershipId"`
	SessionID                 string                   `json:"sessionId,omitempty"`
	AccessID                  string                   `json:"accessId,omitempty"`
	EmployeeID                string                   `json:"employeeId,omitempty"`
	FirstName                 string                   `json:"firstName,omitempty"`
	LastName                  string                   `json:"lastName,omitempty"`
	JobTitle                  string                   `json:"jobTitle,omitempty"`
	Attendances               []factorialAttendanceRow `json:"attendances"`
}

func (h *handler) factorialOrUnavailable(w http.ResponseWriter) bool {
	if h.factorial == nil {
		httputil.Error(w, http.StatusServiceUnavailable, "Factorial non configurato: impostare FACTORIAL_API_KEY")
		return false
	}
	return true
}

func (h *handler) writeFactorialError(w http.ResponseWriter, r *http.Request, err error, action string) {
	if apiErr, ok := errors.AsType[*factorial.APIError](err); ok {
		h.logger.Error("factorial API error", "action", action, "status", apiErr.StatusCode, "error", err)
		httputil.Error(w, http.StatusBadGateway, "Errore Factorial: "+apiErr.Error())
		return
	}
	httputil.InternalError(w, r, err, "Errore di comunicazione con Factorial", "action", action)
}

func (h *handler) factorialEmployeeIndex(ctx context.Context) (map[string]factorial.EmployeesEmployee, error) {
	employees, err := h.factorial.Employees.Employees.All(ctx, &factorial.EmployeesEmployeesListParams{})
	if err != nil {
		return nil, err
	}
	index := make(map[string]factorial.EmployeesEmployee, len(employees))
	for _, e := range employees {
		if e.ID != nil {
			index[*e.ID] = e
		}
	}
	return index, nil
}

func employeeDisplayName(e factorial.EmployeesEmployee) string {
	if e.FullName != nil && strings.TrimSpace(*e.FullName) != "" {
		return strings.TrimSpace(*e.FullName)
	}
	return strings.TrimSpace(deref(e.FirstName) + " " + deref(e.LastName))
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func personRef(index map[string]factorial.EmployeesEmployee, id string) factorialPersonRef {
	ref := factorialPersonRef{ID: id}
	if e, ok := index[id]; ok {
		ref.Name = employeeDisplayName(e)
	}
	return ref
}

func (h *handler) handleFactorialStatus(w http.ResponseWriter, r *http.Request) {
	resp := factorialStatusResponse{Configured: h.factorial != nil}
	if h.factorial != nil {
		limit := 1
		_, err := h.factorial.Employees.Employees.List(r.Context(), &factorial.EmployeesEmployeesListParams{Limit: &limit})
		if err != nil {
			resp.Error = err.Error()
		} else {
			resp.OK = true
		}
	}
	httputil.JSON(w, http.StatusOK, resp)
}

func (h *handler) handleFactorialEmployees(w http.ResponseWriter, r *http.Request) {
	if !h.factorialOrUnavailable(w) {
		return
	}
	index, err := h.factorialEmployeeIndex(r.Context())
	if err != nil {
		h.writeFactorialError(w, r, err, "training.factorial_employees")
		return
	}
	rows := make([]factorialEmployeeRow, 0, len(index))
	for id, e := range index {
		row := factorialEmployeeRow{
			ID:           id,
			FullName:     employeeDisplayName(e),
			Email:        deref(e.Email),
			LoginEmail:   deref(e.LoginEmail),
			Active:       e.Active != nil && *e.Active,
			TerminatedOn: e.TerminatedOn,
		}
		if e.ManagerID != nil && *e.ManagerID != "" {
			ref := personRef(index, *e.ManagerID)
			row.Manager = &ref
		}
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].FullName < rows[j].FullName })
	httputil.JSON(w, http.StatusOK, map[string]any{"employees": rows})
}

func (h *handler) handleFactorialTeams(w http.ResponseWriter, r *http.Request) {
	if !h.factorialOrUnavailable(w) {
		return
	}
	index, err := h.factorialEmployeeIndex(r.Context())
	if err != nil {
		h.writeFactorialError(w, r, err, "training.factorial_teams")
		return
	}
	teams, err := h.factorial.Teams.Teams.All(r.Context(), &factorial.TeamsTeamsListParams{})
	if err != nil {
		h.writeFactorialError(w, r, err, "training.factorial_teams")
		return
	}
	rows := make([]factorialTeamRow, 0, len(teams))
	for _, t := range teams {
		row := factorialTeamRow{
			ID:          deref(t.ID),
			Name:        deref(t.Name),
			Description: deref(t.Description),
			Members:     make([]factorialPersonRef, 0, len(t.EmployeeIDs)),
			Leads:       make([]factorialPersonRef, 0, len(t.LeadIDs)),
		}
		for _, id := range t.EmployeeIDs {
			row.Members = append(row.Members, personRef(index, id))
		}
		for _, id := range t.LeadIDs {
			row.Leads = append(row.Leads, personRef(index, id))
		}
		sort.Slice(row.Members, func(i, j int) bool { return row.Members[i].Name < row.Members[j].Name })
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Name < rows[j].Name })
	httputil.JSON(w, http.StatusOK, map[string]any{"teams": rows})
}

func (h *handler) handleFactorialTrainings(w http.ResponseWriter, r *http.Request) {
	if !h.factorialOrUnavailable(w) {
		return
	}
	trainings, err := h.factorial.Trainings.Trainings.All(r.Context(), &factorial.TrainingsTrainingsListParams{})
	if err != nil {
		h.writeFactorialError(w, r, err, "training.factorial_trainings")
		return
	}
	categoryNames := map[string]string{}
	if categories, err := h.factorial.Trainings.Categories.All(r.Context(), &factorial.TrainingsCategoriesListParams{}); err == nil {
		for _, c := range categories {
			if c.ID != nil {
				categoryNames[*c.ID] = deref(c.Name)
			}
		}
	} else {
		h.logger.Warn("factorial training categories unavailable", "error", err)
	}
	rows := make([]factorialTrainingRow, 0, len(trainings))
	for _, t := range trainings {
		row := factorialTrainingRow{
			ID:       deref(t.ID),
			Name:     deref(t.Name),
			Code:     deref(t.Code),
			Year:     t.Year,
			Catalog:  t.Catalog != nil && *t.Catalog,
			External: t.External != nil && *t.External,
			Provider: deref(t.ExternalProvider),
			Cost:     deref(t.TotalCostDecimal),
		}
		if t.Status != nil {
			row.Status = string(*t.Status)
		}
		for _, id := range t.CategoryIDs {
			if name, ok := categoryNames[id]; ok && name != "" {
				row.Categories = append(row.Categories, name)
			}
		}
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Name < rows[j].Name })
	httputil.JSON(w, http.StatusOK, map[string]any{"trainings": rows})
}

func (h *handler) handleFactorialTrainingMemberships(w http.ResponseWriter, r *http.Request) {
	if !h.factorialOrUnavailable(w) {
		return
	}
	trainingID := strings.TrimSpace(r.PathValue("id"))
	if trainingID == "" {
		httputil.Error(w, http.StatusBadRequest, "id corso mancante")
		return
	}
	index, err := h.factorialEmployeeIndex(r.Context())
	if err != nil {
		h.writeFactorialError(w, r, err, "training.factorial_memberships")
		return
	}
	memberships, err := h.factorial.Trainings.TrainingMemberships.All(r.Context(), &factorial.TrainingsTrainingMembershipsListParams{TrainingID: &trainingID})
	if err != nil {
		h.writeFactorialError(w, r, err, "training.factorial_memberships")
		return
	}
	rows := make([]factorialMembershipRow, 0, len(memberships))
	for _, m := range memberships {
		row := factorialMembershipRow{
			ID:          deref(m.ID),
			AccessID:    deref(m.AccessID),
			TrainingID:  deref(m.TrainingID),
			EmployeeID:  deref(m.EmployeeID),
			DueDate:     m.TrainingDueDate,
			CompletedAt: m.TrainingCompletedAt,
		}
		if m.Status != nil {
			row.Status = string(*m.Status)
		}
		if row.EmployeeID != "" {
			row.EmployeeName = personRef(index, row.EmployeeID).Name
		}
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].EmployeeName < rows[j].EmployeeName })
	httputil.JSON(w, http.StatusOK, map[string]any{"memberships": rows})
}

func (h *handler) handleFactorialTrainingStructure(w http.ResponseWriter, r *http.Request) {
	if !h.factorialOrUnavailable(w) {
		return
	}
	trainingID := strings.TrimSpace(r.PathValue("id"))
	if trainingID == "" {
		httputil.Error(w, http.StatusBadRequest, "id corso mancante")
		return
	}
	classes, err := h.factorial.Trainings.TrainingClasses.All(r.Context(), &factorial.TrainingsTrainingClassesListParams{TrainingID: &trainingID})
	if err != nil {
		h.writeFactorialError(w, r, err, "training.factorial_training_structure")
		return
	}
	sessions, err := h.factorial.Trainings.Sessions.All(r.Context(), &factorial.TrainingsSessionsListParams{TrainingIDs: []string{trainingID}})
	if err != nil {
		h.writeFactorialError(w, r, err, "training.factorial_training_structure")
		return
	}
	classRows := make([]factorialClassRow, 0, len(classes))
	for _, c := range classes {
		row := factorialClassRow{
			ID:                        deref(c.ID),
			TrainingID:                deref(c.TrainingID),
			Name:                      deref(c.Name),
			Description:               deref(c.Description),
			StartDate:                 c.StartDate,
			EndDate:                   c.EndDate,
			Cost:                      deref(c.Cost),
			IndirectCost:              deref(c.IndirectCost),
			SalaryCost:                deref(c.SalaryCost),
			SubsidizedCost:            deref(c.SubsidizedCost),
			GrossCost:                 deref(c.GrossCost),
			NetCost:                   deref(c.NetCost),
			Currency:                  deref(c.Currency),
			CompletedAttendancesCount: c.CompletedAttendancesCount,
			TotalAttendancesCount:     c.TotalAttendancesCount,
		}
		if c.PaymentStatus != nil {
			row.PaymentStatus = string(*c.PaymentStatus)
		}
		classRows = append(classRows, row)
	}
	sort.Slice(classRows, func(i, j int) bool {
		ti, oki := classSortInstant(classRows[i])
		tj, okj := classSortInstant(classRows[j])
		if oki || okj {
			if !oki {
				return false
			}
			if !okj {
				return true
			}
			if !ti.Equal(tj) {
				return ti.Before(tj)
			}
		}
		if classRows[i].Name != classRows[j].Name {
			return classRows[i].Name < classRows[j].Name
		}
		return classRows[i].ID < classRows[j].ID
	})
	sessionRows := make([]factorialSessionRow, 0, len(sessions))
	for _, s := range sessions {
		row := factorialSessionRow{
			ID:              deref(s.ID),
			TrainingID:      deref(s.TrainingID),
			TrainingClassID: deref(s.TrainingClassID),
			Name:            deref(s.Name),
			Description:     deref(s.Description),
			StartsAt:        s.StartsAt,
			EndsAt:          s.EndsAt,
			DueDate:         s.DueDate,
			Duration:        deref(s.Duration),
			Location:        deref(s.Location),
			Status:          deref(s.Status),
			ParentID:        deref(s.ParentID),
		}
		if s.Modality != nil {
			row.Modality = string(*s.Modality)
		}
		if s.Schedule != nil {
			row.Schedule = string(*s.Schedule)
		}
		sessionRows = append(sessionRows, row)
	}
	sort.Slice(sessionRows, func(i, j int) bool {
		ti, oki := sessionSortInstant(sessionRows[i])
		tj, okj := sessionSortInstant(sessionRows[j])
		if oki || okj {
			if !oki {
				return false
			}
			if !okj {
				return true
			}
			if !ti.Equal(tj) {
				return ti.Before(tj)
			}
		}
		if sessionRows[i].Name != sessionRows[j].Name {
			return sessionRows[i].Name < sessionRows[j].Name
		}
		return sessionRows[i].ID < sessionRows[j].ID
	})
	httputil.JSON(w, http.StatusOK, map[string]any{
		"classes":  classRows,
		"sessions": sessionRows,
	})
}

func (h *handler) handleFactorialSessionParticipants(w http.ResponseWriter, r *http.Request) {
	if !h.factorialOrUnavailable(w) {
		return
	}
	sessionID := strings.TrimSpace(r.PathValue("id"))
	if sessionID == "" {
		httputil.Error(w, http.StatusBadRequest, "id sessione mancante")
		return
	}
	memberships, err := h.factorial.Trainings.SessionAccessMemberships.All(r.Context(), &factorial.TrainingsSessionAccessMembershipsListParams{SessionID: sessionID})
	if err != nil {
		h.writeFactorialError(w, r, err, "training.factorial_session_participants")
		return
	}
	attendances, err := h.factorial.Trainings.SessionAttendances.All(r.Context(), &factorial.TrainingsSessionAttendancesListParams{SessionID: &sessionID})
	if err != nil {
		h.writeFactorialError(w, r, err, "training.factorial_session_participants")
		return
	}
	membershipIDs := make(map[string]struct{}, len(memberships))
	for _, m := range memberships {
		if m.ID != nil && *m.ID != "" {
			membershipIDs[*m.ID] = struct{}{}
		}
	}
	byMembership := make(map[string][]factorialAttendanceRow, len(memberships))
	unmatched := make([]factorialAttendanceRow, 0)
	for _, a := range attendances {
		row := factorialAttendanceRow{
			ID:                        deref(a.ID),
			SessionAccessMembershipID: deref(a.SessionAccessMembershipID),
			AccessID:                  deref(a.AccessID),
			EmployeeID:                deref(a.EmployeeID),
			CompletedDuration:         deref(a.CompletedDuration),
		}
		if a.Status != nil {
			row.Status = string(*a.Status)
		}
		membershipID := row.SessionAccessMembershipID
		if membershipID == "" {
			unmatched = append(unmatched, row)
			continue
		}
		if _, ok := membershipIDs[membershipID]; !ok {
			unmatched = append(unmatched, row)
			continue
		}
		byMembership[membershipID] = append(byMembership[membershipID], row)
	}
	participants := make([]factorialSessionParticipantRow, 0, len(memberships))
	for _, m := range memberships {
		membershipID := deref(m.ID)
		atts := byMembership[membershipID]
		if atts == nil {
			atts = make([]factorialAttendanceRow, 0)
		}
		sort.Slice(atts, func(i, j int) bool { return atts[i].ID < atts[j].ID })
		participants = append(participants, factorialSessionParticipantRow{
			SessionAccessMembershipID: membershipID,
			SessionID:                 deref(m.SessionID),
			AccessID:                  deref(m.AccessID),
			EmployeeID:                deref(m.EmployeeID),
			FirstName:                 deref(m.FirstName),
			LastName:                  deref(m.LastName),
			JobTitle:                  deref(m.JobTitle),
			Attendances:               atts,
		})
	}
	sort.Slice(participants, func(i, j int) bool {
		a, b := participants[i], participants[j]
		if a.LastName != b.LastName {
			return a.LastName < b.LastName
		}
		if a.FirstName != b.FirstName {
			return a.FirstName < b.FirstName
		}
		return a.SessionAccessMembershipID < b.SessionAccessMembershipID
	})
	sort.Slice(unmatched, func(i, j int) bool { return unmatched[i].ID < unmatched[j].ID })
	httputil.JSON(w, http.StatusOK, map[string]any{
		"participants":         participants,
		"unmatchedAttendances": unmatched,
	})
}

func classSortInstant(c factorialClassRow) (time.Time, bool) {
	if c.StartDate != nil {
		return c.StartDate.Time, true
	}
	return time.Time{}, false
}

func sessionSortInstant(s factorialSessionRow) (time.Time, bool) {
	if s.StartsAt != nil {
		return s.StartsAt.Time, true
	}
	if s.DueDate != nil {
		return s.DueDate.Time, true
	}
	return time.Time{}, false
}
