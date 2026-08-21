package training

import (
	"context"
	"errors"
	"net/http"
	"sort"
	"strings"

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
	EmployeeID   string          `json:"employeeId"`
	EmployeeName string          `json:"employeeName,omitempty"`
	Status       string          `json:"status,omitempty"`
	DueDate      *factorial.Date `json:"dueDate,omitempty"`
	CompletedAt  *factorial.Time `json:"completedAt,omitempty"`
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
			ID:        deref(t.ID),
			Name:      deref(t.Name),
			Code:      deref(t.Code),
			Year:      t.Year,
			Catalog:   t.Catalog != nil && *t.Catalog,
			External:  t.External != nil && *t.External,
			Provider:  deref(t.ExternalProvider),
			Cost:      deref(t.TotalCostDecimal),
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
