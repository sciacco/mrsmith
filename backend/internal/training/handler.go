package training

import (
	"database/sql"
	"log/slog"
	"net/http"
	"strings"

	"github.com/sciacco/mrsmith/internal/acl"
	"github.com/sciacco/mrsmith/internal/auth"
	"github.com/sciacco/mrsmith/internal/authz"
	"github.com/sciacco/mrsmith/internal/notifications"
	"github.com/sciacco/mrsmith/internal/platform/applaunch"
	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

type Deps struct {
	DB              *sql.DB
	Notifier        notifications.Notifier
	Logger          *slog.Logger
	StorageDir      string
	StorageMaxBytes int64
	TrainingAppURL  string
	StaticDir       string
}

type handler struct {
	store           *SQLStore
	notifier        notifications.Notifier
	logger          *slog.Logger
	storage         StorageAdapter
	storageMaxBytes int64
	trainingAppURL  string
	staticDir       string
}

func RegisterRoutes(mux *http.ServeMux, deps Deps) {
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	storage, err := NewLocalStorage(deps.StorageDir)
	if err != nil {
		logger.Error("training storage disabled", "component", "training", "error", err)
	}
	if deps.StorageMaxBytes <= 0 {
		deps.StorageMaxBytes = defaultStorageMaxBytes
	}

	h := &handler{
		store:           NewSQLStore(deps.DB),
		notifier:        deps.Notifier,
		logger:          logger.With("component", "training"),
		storage:         storage,
		storageMaxBytes: deps.StorageMaxBytes,
		trainingAppURL:  deps.TrainingAppURL,
		staticDir:       deps.StaticDir,
	}

	protect := acl.RequireRole(applaunch.TrainingAppAccessRoles()...)
	peopleProtect := acl.RequireRole(applaunch.TrainingPeopleAdminRoles()...)

	mux.Handle("GET /training/v1/health", protect(http.HandlerFunc(h.handleHealth)))
	mux.Handle("GET /training/v1/me", protect(h.requireStore(http.HandlerFunc(h.handleMe))))
	mux.Handle("GET /training/v1/workspace", protect(h.requireStore(http.HandlerFunc(h.handleWorkspace))))
	mux.Handle("GET /training/v1/lookups", protect(h.requireStore(http.HandlerFunc(h.handleLookups))))
	mux.Handle("GET /training/v1/exports/{kind}", protect(h.requireStore(http.HandlerFunc(h.handleExport))))
	mux.Handle("POST /training/v1/requests", protect(h.requireStore(http.HandlerFunc(h.handleCreateRequest))))
	mux.Handle("POST /training/v1/requests/{id}/transition", protect(h.requireStore(http.HandlerFunc(h.handleTransitionRequest))))
	mux.Handle("POST /training/v1/awards", protect(h.requireStore(http.HandlerFunc(h.handleCreateAward))))
	mux.Handle("POST /training/v1/enrollments/{id}/documents", protect(h.requireStore(http.HandlerFunc(h.handleUploadEnrollmentDocument))))
	mux.Handle("POST /training/v1/awards/{id}/documents", protect(h.requireStore(http.HandlerFunc(h.handleUploadAwardDocument))))
	mux.Handle("GET /training/v1/documents/{id}/download", protect(h.requireStore(http.HandlerFunc(h.handleDownloadDocument))))
	mux.Handle(
		"GET /training/v1/people/workspace",
		protect(peopleProtect(h.requireStore(http.HandlerFunc(h.handleWorkspace)))),
	)
	mux.Handle("POST /training/v1/people/enrollments", protect(peopleProtect(h.requireStore(http.HandlerFunc(h.handleCreateEnrollment)))))
	mux.Handle("PUT /training/v1/people/enrollments/{id}", protect(peopleProtect(h.requireStore(http.HandlerFunc(h.handleUpdateEnrollment)))))
	mux.Handle("POST /training/v1/people/enrollments/{id}/transition", protect(peopleProtect(h.requireStore(http.HandlerFunc(h.handleTransitionEnrollment)))))
	mux.Handle("POST /training/v1/people/documents/{id}/validate", protect(peopleProtect(h.requireStore(http.HandlerFunc(h.handleValidateDocument)))))
	mux.Handle("POST /training/v1/people/imports/training-plan", protect(peopleProtect(h.requireStore(http.HandlerFunc(h.handleImportTrainingPlan)))))
	mux.Handle("POST /training/v1/people/jobs/run", protect(peopleProtect(h.requireStore(http.HandlerFunc(h.handleRunJobs)))))
	mux.Handle("PUT /training/v1/people/awards/{id}", protect(peopleProtect(h.requireStore(http.HandlerFunc(h.handleUpdateAward)))))
	mux.Handle("POST /training/v1/people/vendors", protect(peopleProtect(h.requireStore(http.HandlerFunc(h.handleUpsertVendor)))))
	mux.Handle("PUT /training/v1/people/vendors/{id}", protect(peopleProtect(h.requireStore(http.HandlerFunc(h.handleUpsertVendor)))))
	mux.Handle("POST /training/v1/people/teams", protect(peopleProtect(h.requireStore(http.HandlerFunc(h.handleUpsertTeam)))))
	mux.Handle("PUT /training/v1/people/teams/{id}", protect(peopleProtect(h.requireStore(http.HandlerFunc(h.handleUpsertTeam)))))
	mux.Handle("POST /training/v1/people/skill-areas", protect(peopleProtect(h.requireStore(http.HandlerFunc(h.handleUpsertSkillArea)))))
	mux.Handle("PUT /training/v1/people/skill-areas/{id}", protect(peopleProtect(h.requireStore(http.HandlerFunc(h.handleUpsertSkillArea)))))
	mux.Handle("POST /training/v1/people/certifications", protect(peopleProtect(h.requireStore(http.HandlerFunc(h.handleUpsertCertification)))))
	mux.Handle("PUT /training/v1/people/certifications/{id}", protect(peopleProtect(h.requireStore(http.HandlerFunc(h.handleUpsertCertification)))))
	mux.Handle("POST /training/v1/people/courses", protect(peopleProtect(h.requireStore(http.HandlerFunc(h.handleUpsertCourse)))))
	mux.Handle("PUT /training/v1/people/courses/{id}", protect(peopleProtect(h.requireStore(http.HandlerFunc(h.handleUpsertCourse)))))
	mux.Handle("POST /training/v1/people/plans", protect(peopleProtect(h.requireStore(http.HandlerFunc(h.handleUpsertPlan)))))
	mux.Handle("PUT /training/v1/people/plans/{id}", protect(peopleProtect(h.requireStore(http.HandlerFunc(h.handleUpsertPlan)))))
	mux.Handle("POST /training/v1/people/mandatory-rules", protect(peopleProtect(h.requireStore(http.HandlerFunc(h.handleUpsertMandatoryRule)))))
	mux.Handle("PUT /training/v1/people/mandatory-rules/{id}", protect(peopleProtect(h.requireStore(http.HandlerFunc(h.handleUpsertMandatoryRule)))))
}

func (h *handler) requireStore(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h.store == nil {
			h.logger.Warn(
				"training database dependency missing",
				"operation", "require_store",
				"path", r.URL.Path,
			)
			httputil.Error(w, http.StatusServiceUnavailable, "training_database_not_configured")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (h *handler) handleHealth(w http.ResponseWriter, _ *http.Request) {
	httputil.JSON(w, http.StatusOK, map[string]any{
		"ok":                 true,
		"databaseConfigured": h.store != nil,
		"storageConfigured":  h.storage != nil,
		"notifications":      h.notifier != nil,
		"appUrlConfigured":   strings.TrimSpace(h.trainingAppURL) != "",
		"staticHosting":      strings.TrimSpace(h.staticDir) != "",
	})
}

func (h *handler) handleMe(w http.ResponseWriter, r *http.Request) {
	principal, ok := principalFromRequest(r)
	if !ok {
		httputil.Error(w, http.StatusUnauthorized, "missing_auth_claims")
		return
	}
	employee, err := h.store.GetEmployeeByEmail(r.Context(), principal.Email)
	if err != nil {
		httputil.InternalError(w, r, err, "load training current user", "operation", "training.me")
		return
	}
	httputil.JSON(w, http.StatusOK, MeResponse{
		Principal:         principal,
		Employee:          employee,
		OnboardingPending: employee == nil,
	})
}

func (h *handler) handleWorkspace(w http.ResponseWriter, r *http.Request) {
	principal, ok := principalFromRequest(r)
	if !ok {
		httputil.Error(w, http.StatusUnauthorized, "missing_auth_claims")
		return
	}
	workspace, err := h.store.Workspace(r.Context(), principal)
	if err != nil {
		httputil.InternalError(w, r, err, "load training workspace", "operation", "training.workspace")
		return
	}
	httputil.JSON(w, http.StatusOK, workspace)
}

func (h *handler) handleLookups(w http.ResponseWriter, r *http.Request) {
	principal, ok := principalFromRequest(r)
	if !ok {
		httputil.Error(w, http.StatusUnauthorized, "missing_auth_claims")
		return
	}
	lookups, err := h.store.Lookups(r.Context(), principal)
	if err != nil {
		httputil.InternalError(w, r, err, "load training lookups", "operation", "training.lookups")
		return
	}
	httputil.JSON(w, http.StatusOK, lookups)
}

func principalFromRequest(r *http.Request) (Principal, bool) {
	claims, ok := auth.GetClaims(r.Context())
	if !ok {
		return Principal{}, false
	}
	return Principal{
		Subject:       claims.Subject,
		Email:         normalizeEmail(claims.Email),
		Name:          claims.Name,
		Roles:         append([]string(nil), claims.Roles...),
		IsPeopleAdmin: authz.HasAnyRole(claims.Roles, applaunch.TrainingPeopleAdminRoles()...),
	}, true
}
