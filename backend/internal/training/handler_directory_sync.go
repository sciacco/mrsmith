package training

import (
	"net/http"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

type directorySyncRequest struct {
	DryRun bool `json:"dryRun"`
}

func (h *handler) handleListDirectorySyncRuns(w http.ResponseWriter, r *http.Request) {
	runs, err := h.store.ListDirectorySyncRuns(r.Context(), 20)
	if err != nil {
		httputil.InternalError(w, r, err, "load training directory sync runs", "operation", "training.directory_sync_runs")
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"runs": runs})
}

func (h *handler) handleRunDirectorySync(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	if !principal.IsPeopleAdmin {
		httputil.JSON(w, http.StatusForbidden, map[string]string{
			"error":   "people_role_required",
			"message": "azione riservata a People",
		})
		return
	}
	if h.directory == nil {
		httputil.JSON(w, http.StatusServiceUnavailable, map[string]string{
			"error":   "factorial_not_configured",
			"message": "directory esterna non configurata: impostare FACTORIAL_API_KEY",
		})
		return
	}
	input, ok := decodeJSONBody[directorySyncRequest](w, r)
	if !ok {
		return
	}
	run, err := h.store.RunDirectorySync(r.Context(), h.directory, "manual:"+principal.Email, input.DryRun)
	if err != nil {
		h.writeActionError(w, r, err, "training.directory_sync")
		return
	}
	httputil.JSON(w, http.StatusOK, run)
}
