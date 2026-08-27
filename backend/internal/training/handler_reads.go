package training

import (
	"net/http"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

// registerDomainReadRoutes registra le letture di dominio non ancora
// esposte dal backend (#152, slice 1 del task 6): catalogo, persone e
// anagrafiche.
func (h *handler) registerDomainReadRoutes(mux *http.ServeMux, protect func(http.Handler) http.Handler) {
	mux.Handle("GET /training/v1/courses", protect(h.requireStore(http.HandlerFunc(h.handleListCourses))))
	mux.Handle("GET /training/v1/courses/{id}", protect(h.requireStore(http.HandlerFunc(h.handleGetCourse))))
	mux.Handle("GET /training/v1/people", protect(h.requireStore(http.HandlerFunc(h.handleListPeople))))
	mux.Handle("GET /training/v1/people/{id}", protect(h.requireStore(http.HandlerFunc(h.handleGetPerson))))
	mux.Handle("GET /training/v1/teams", protect(h.requireStore(http.HandlerFunc(h.handleListTeams))))
	mux.Handle("GET /training/v1/vendors", protect(h.requireStore(http.HandlerFunc(h.handleListVendors))))
	mux.Handle("GET /training/v1/skill-areas", protect(h.requireStore(http.HandlerFunc(h.handleListSkillAreas))))
	mux.Handle("GET /training/v1/certifications", protect(h.requireStore(http.HandlerFunc(h.handleListCertificationCatalog))))
	mux.Handle("GET /training/v1/certifications/{id}", protect(h.requireStore(http.HandlerFunc(h.handleGetCertification))))
}

// handleRead e il pendant in lettura di handleUpsert (handler_actions.go):
// autentica e risponde JSON, la query resta nella closure del chiamante.
func (h *handler) handleRead(w http.ResponseWriter, r *http.Request, op string, fn func() (any, error)) {
	if _, ok := h.principalOrUnauthorized(w, r); !ok {
		return
	}
	data, err := fn()
	if err != nil {
		h.writeActionError(w, r, err, op)
		return
	}
	httputil.JSON(w, http.StatusOK, data)
}

func (h *handler) handleListCourses(w http.ResponseWriter, r *http.Request) {
	h.handleRead(w, r, "training.list_courses", func() (any, error) {
		courses, err := h.store.ListCourses(r.Context())
		return CourseListResponse{Courses: courses}, err
	})
}

func (h *handler) handleGetCourse(w http.ResponseWriter, r *http.Request) {
	h.handleRead(w, r, "training.get_course", func() (any, error) {
		return h.store.GetCourseDetail(r.Context(), r.PathValue("id"))
	})
}

func (h *handler) handleListPeople(w http.ResponseWriter, r *http.Request) {
	h.handleRead(w, r, "training.list_people", func() (any, error) {
		people, err := h.store.ListPeople(r.Context())
		return PersonListResponse{People: people}, err
	})
}

func (h *handler) handleGetPerson(w http.ResponseWriter, r *http.Request) {
	h.handleRead(w, r, "training.get_person", func() (any, error) {
		return h.store.GetPersonDetail(r.Context(), r.PathValue("id"))
	})
}

func (h *handler) handleListTeams(w http.ResponseWriter, r *http.Request) {
	h.handleRead(w, r, "training.list_teams", func() (any, error) {
		teams, err := h.store.ListTeams(r.Context())
		return TeamListResponse{Teams: teams}, err
	})
}

func (h *handler) handleListVendors(w http.ResponseWriter, r *http.Request) {
	h.handleRead(w, r, "training.list_vendors", func() (any, error) {
		vendors, err := h.store.ListVendors(r.Context())
		return VendorListResponse{Vendors: vendors}, err
	})
}

func (h *handler) handleListSkillAreas(w http.ResponseWriter, r *http.Request) {
	h.handleRead(w, r, "training.list_skill_areas", func() (any, error) {
		skillAreas, err := h.store.ListSkillAreas(r.Context())
		return SkillAreaListResponse{SkillAreas: skillAreas}, err
	})
}

func (h *handler) handleListCertificationCatalog(w http.ResponseWriter, r *http.Request) {
	h.handleRead(w, r, "training.list_certification_catalog", func() (any, error) {
		certifications, err := h.store.ListCertificationCatalog(r.Context())
		return CertificationCatalogResponse{Certifications: certifications}, err
	})
}

func (h *handler) handleGetCertification(w http.ResponseWriter, r *http.Request) {
	h.handleRead(w, r, "training.get_certification", func() (any, error) {
		return h.store.GetCertificationDetail(r.Context(), r.PathValue("id"))
	})
}
