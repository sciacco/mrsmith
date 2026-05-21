package training

import (
	"net/http"
	"strings"
	"time"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

func (h *handler) handleListCatalogCourses(w http.ResponseWriter, r *http.Request) {
	_, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	filters := CatalogListFilters{
		SkillArea: strings.TrimSpace(r.URL.Query().Get("skill_area")),
		Vendor:    strings.TrimSpace(r.URL.Query().Get("fornitore")),
		Stato:     strings.TrimSpace(r.URL.Query().Get("stato")),
		Search:    strings.TrimSpace(r.URL.Query().Get("q")),
		Year:      time.Now().Year(),
	}
	if y := strings.TrimSpace(r.URL.Query().Get("year")); y != "" {
		if parsed, err := strconvAtoi(y); err == nil {
			filters.Year = parsed
		}
	}
	courses, err := h.store.ListCatalogCoursesFiltered(r.Context(), filters)
	if err != nil {
		h.writeActionError(w, r, err, "training.catalog_list")
		return
	}
	httputil.JSON(w, http.StatusOK, CatalogListResponse{Courses: courses})
}

func (h *handler) handleArchiveCourse(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	resp, err := h.store.ArchiveCourse(r.Context(), principal, r.PathValue("id"))
	if err != nil {
		h.writeActionError(w, r, err, "training.archive_course")
		return
	}
	httputil.JSON(w, http.StatusOK, resp)
}
