package training

import (
	"net/http"
	"strings"
	"time"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

func (h *handler) handleComplianceOverview(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	year := time.Now().Year()
	if y := strings.TrimSpace(r.URL.Query().Get("year")); y != "" {
		if parsed, err := strconvAtoi(y); err == nil {
			year = parsed
		}
	}
	team := strings.TrimSpace(r.URL.Query().Get("team"))
	deadlineDays := 30
	if d := strings.TrimSpace(r.URL.Query().Get("deadline_days")); d != "" {
		if parsed, err := strconvAtoi(d); err == nil && parsed > 0 {
			deadlineDays = parsed
		}
	}
	resp, err := h.store.ComplianceOverview(r.Context(), principal, year, team, deadlineDays)
	if err != nil {
		h.writeActionError(w, r, err, "training.compliance_overview")
		return
	}
	httputil.JSON(w, http.StatusOK, resp)
}
