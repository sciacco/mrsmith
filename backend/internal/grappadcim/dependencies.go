package grappadcim

import "net/http"

type dependencyCheck struct {
	Key   string
	Label string
	Query string
}

func (h *Handler) runDependencyChecks(r *http.Request, id int, checks []dependencyCheck) (DependencySummary, error) {
	summary := DependencySummary{
		Allowed: true,
		Counts:  map[string]int{},
		Details: []DependencyDetail{},
	}
	for _, check := range checks {
		var count int
		if err := h.grappa.QueryRowContext(r.Context(), check.Query, id).Scan(&count); err != nil {
			return summary, err
		}
		summary.Counts[check.Key] = count
		if count > 0 {
			summary.Allowed = false
			summary.Details = append(summary.Details, DependencyDetail{Label: check.Label, Count: count})
		}
	}
	if !summary.Allowed {
		summary.Message = "Azione bloccata da dipendenze operative."
	}
	return summary, nil
}
