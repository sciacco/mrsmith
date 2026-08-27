package training

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"strings"

	"github.com/sciacco/mrsmith/internal/acl"
	"github.com/sciacco/mrsmith/internal/auth"
	"github.com/sciacco/mrsmith/internal/authz"
	"github.com/sciacco/mrsmith/internal/notifications"
	"github.com/sciacco/mrsmith/internal/platform/applaunch"
	"github.com/sciacco/mrsmith/internal/platform/arak"
	"github.com/sciacco/mrsmith/internal/platform/directory"
	"github.com/sciacco/mrsmith/internal/platform/httputil"
	"github.com/sciacco/mrsmith/internal/platform/keycloak"
	"github.com/sciacco/mrsmith/pkg/factorial"
)

type RoleUserResolver interface {
	UsersByRealmRole(ctx context.Context, roleName string, opts keycloak.UsersByRealmRoleOptions) ([]keycloak.User, error)
}

type Deps struct {
	DB              *sql.DB
	Notifier        notifications.Notifier
	Logger          *slog.Logger
	RoleResolver    RoleUserResolver
	StorageDir      string
	StorageMaxBytes int64
	TrainingAppURL  string
	StaticDir       string
	Factorial       *factorial.Client
	Directory       directory.Provider
	Arak            *arak.Client
}

type handler struct {
	store           *SQLStore
	notifier        notifications.Notifier
	logger          *slog.Logger
	roleResolver    RoleUserResolver
	storage         StorageAdapter
	storageMaxBytes int64
	trainingAppURL  string
	staticDir       string
	factorial       *factorial.Client
	directory       directory.Provider
	arak            *arak.Client
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
		roleResolver:    deps.RoleResolver,
		storage:         storage,
		storageMaxBytes: deps.StorageMaxBytes,
		trainingAppURL:  deps.TrainingAppURL,
		staticDir:       deps.StaticDir,
		factorial:       deps.Factorial,
		directory:       deps.Directory,
		arak:            deps.Arak,
	}

	// Ruolo unico: tutte le route Training, letture comprese, richiedono
	// app_training_people_admin.
	protect := acl.RequireRole(applaunch.TrainingPeopleAdminRoles()...)

	mux.Handle("GET /training/v1/health", protect(http.HandlerFunc(h.handleHealth)))
	mux.Handle("GET /training/v1/me", protect(h.requireStore(http.HandlerFunc(h.handleMe))))
	mux.Handle("GET /training/v1/lookups", protect(h.requireStore(http.HandlerFunc(h.handleLookups))))
	mux.Handle("GET /training/v1/exports/{kind}", protect(h.requireStore(http.HandlerFunc(h.handleExport))))

	// Nucleo operativo: eventi, sessioni, iscrizioni e partecipazioni.
	mux.Handle("GET /training/v1/events", protect(h.requireStore(http.HandlerFunc(h.handleListEvents))))
	mux.Handle("POST /training/v1/events", protect(h.requireStore(http.HandlerFunc(h.handleCreateEvent))))
	mux.Handle("GET /training/v1/events/{id}", protect(h.requireStore(http.HandlerFunc(h.handleGetEvent))))
	mux.Handle("PUT /training/v1/events/{id}", protect(h.requireStore(http.HandlerFunc(h.handleUpdateEvent))))
	mux.Handle("POST /training/v1/events/{id}/cancel", protect(h.requireStore(http.HandlerFunc(h.handleCancelEvent))))
	mux.Handle("POST /training/v1/events/{id}/sessions", protect(h.requireStore(http.HandlerFunc(h.handleCreateSession))))
	mux.Handle("PUT /training/v1/sessions/{id}", protect(h.requireStore(http.HandlerFunc(h.handleUpdateSession))))
	mux.Handle("DELETE /training/v1/sessions/{id}", protect(h.requireStore(http.HandlerFunc(h.handleDeleteSession))))
	mux.Handle("POST /training/v1/events/{id}/enrollments", protect(h.requireStore(http.HandlerFunc(h.handleCreateEventEnrollment))))
	mux.Handle("PUT /training/v1/enrollments/{id}", protect(h.requireStore(http.HandlerFunc(h.handleUpdateEnrollmentFacts))))
	mux.Handle("POST /training/v1/enrollments/{id}/cancel", protect(h.requireStore(http.HandlerFunc(h.handleCancelEnrollment))))
	mux.Handle("POST /training/v1/enrollments/{id}/complete-historical", protect(h.requireStore(http.HandlerFunc(h.handleCompleteEnrollmentHistorical))))
	mux.Handle("POST /training/v1/enrollments/{id}/reopen", protect(h.requireStore(http.HandlerFunc(h.handleReopenEnrollment))))
	mux.Handle("POST /training/v1/enrollments/{id}/sessions/{sessionId}", protect(h.requireStore(http.HandlerFunc(h.handleAssignEnrollmentSession))))
	mux.Handle("DELETE /training/v1/enrollments/{id}/sessions/{sessionId}", protect(h.requireStore(http.HandlerFunc(h.handleRemoveEnrollmentSession))))
	mux.Handle("PATCH /training/v1/enrollments/{id}/sessions/{sessionId}", protect(h.requireStore(http.HandlerFunc(h.handleUpdateParticipation))))
	mux.Handle("POST /training/v1/events/{id}/expenses", protect(h.requireStore(http.HandlerFunc(h.handleCreateEventExpense))))
	mux.Handle("PUT /training/v1/expenses/{id}", protect(h.requireStore(http.HandlerFunc(h.handleReplaceEventExpensePO))))
	mux.Handle("PUT /training/v1/expenses/{id}/enrollments", protect(h.requireStore(http.HandlerFunc(h.handleReplaceEventExpenseEnrollments))))
	mux.Handle("DELETE /training/v1/expenses/{id}", protect(h.requireStore(http.HandlerFunc(h.handleDeleteEventExpense))))

	// Certificazioni, documenti e anagrafiche: creazione/modifica ai path
	// piatti (#152, riallineamento dai path /people/* superstiti del POC;
	// gli ultimi tre superstiti riallineati con #160).
	mux.Handle("POST /training/v1/awards", protect(h.requireStore(http.HandlerFunc(h.handleCreateAward))))
	mux.Handle("POST /training/v1/enrollments/{id}/documents", protect(h.requireStore(http.HandlerFunc(h.handleUploadEnrollmentDocument))))
	mux.Handle("POST /training/v1/awards/{id}/documents", protect(h.requireStore(http.HandlerFunc(h.handleUploadAwardDocument))))
	mux.Handle("GET /training/v1/documents/{id}/download", protect(h.requireStore(http.HandlerFunc(h.handleDownloadDocument))))
	mux.Handle("POST /training/v1/people", protect(h.requireStore(http.HandlerFunc(h.handleCreatePerson))))
	mux.Handle("PATCH /training/v1/people/{id}", protect(h.requireStore(http.HandlerFunc(h.handleUpdatePerson))))
	mux.Handle("POST /training/v1/documents/{id}/validate", protect(h.requireStore(http.HandlerFunc(h.handleValidateDocument))))
	mux.Handle("POST /training/v1/jobs/run", protect(h.requireStore(http.HandlerFunc(h.handleRunJobs))))
	mux.Handle("PUT /training/v1/awards/{id}", protect(h.requireStore(http.HandlerFunc(h.handleUpdateAward))))
	mux.Handle("POST /training/v1/vendors", protect(h.requireStore(http.HandlerFunc(h.handleUpsertVendor))))
	mux.Handle("PUT /training/v1/vendors/{id}", protect(h.requireStore(http.HandlerFunc(h.handleUpsertVendor))))
	mux.Handle("POST /training/v1/teams", protect(h.requireStore(http.HandlerFunc(h.handleUpsertTeam))))
	mux.Handle("PUT /training/v1/teams/{id}", protect(h.requireStore(http.HandlerFunc(h.handleUpsertTeam))))
	mux.Handle("POST /training/v1/skill-areas", protect(h.requireStore(http.HandlerFunc(h.handleUpsertSkillArea))))
	mux.Handle("PUT /training/v1/skill-areas/{id}", protect(h.requireStore(http.HandlerFunc(h.handleUpsertSkillArea))))
	mux.Handle("POST /training/v1/certifications", protect(h.requireStore(http.HandlerFunc(h.handleUpsertCertification))))
	mux.Handle("PUT /training/v1/certifications/{id}", protect(h.requireStore(http.HandlerFunc(h.handleUpsertCertification))))
	mux.Handle("POST /training/v1/courses", protect(h.requireStore(http.HandlerFunc(h.handleUpsertCourse))))
	mux.Handle("PUT /training/v1/courses/{id}", protect(h.requireStore(http.HandlerFunc(h.handleUpsertCourse))))
	mux.Handle("POST /training/v1/courses/{id}/archive", protect(h.requireStore(http.HandlerFunc(h.handleArchiveCourse))))

	mux.Handle("GET /training/v1/directory/sync/runs", protect(h.requireStore(http.HandlerFunc(h.handleListDirectorySyncRuns))))
	mux.Handle("POST /training/v1/directory/sync", protect(h.requireStore(http.HandlerFunc(h.handleRunDirectorySync))))

	// Factorial: interrogazione diagnostica read-only.
	mux.Handle("GET /training/v1/factorial/status", protect(http.HandlerFunc(h.handleFactorialStatus)))
	mux.Handle("GET /training/v1/factorial/employees", protect(http.HandlerFunc(h.handleFactorialEmployees)))
	mux.Handle("GET /training/v1/factorial/teams", protect(http.HandlerFunc(h.handleFactorialTeams)))
	mux.Handle("GET /training/v1/factorial/trainings", protect(http.HandlerFunc(h.handleFactorialTrainings)))
	mux.Handle("GET /training/v1/factorial/trainings/{id}/memberships", protect(http.HandlerFunc(h.handleFactorialTrainingMemberships)))
	mux.Handle("GET /training/v1/factorial/trainings/{id}/structure", protect(http.HandlerFunc(h.handleFactorialTrainingStructure)))
	mux.Handle("GET /training/v1/factorial/sessions/{id}/participants", protect(http.HandlerFunc(h.handleFactorialSessionParticipants)))

	// Sync formativo Factorial: letture delle run persistite (#154, task 6.3
	// di #151). Nessuna route di avvio qui.
	mux.Handle("GET /training/v1/factorial/sync/runs", protect(h.requireStore(http.HandlerFunc(h.handleListFactorialSyncRuns))))
	mux.Handle("GET /training/v1/factorial/sync/runs/{id}", protect(h.requireStore(http.HandlerFunc(h.handleGetFactorialSyncRun))))

	// Regole formative, richieste e code operative derivate (#140):
	// registrazione delegata ai file dedicati.
	h.registerRuleRoutes(mux, protect)
	h.registerRequestRoutes(mux, protect)
	h.registerQueueRoutes(mux, protect)

	// Letture di dominio e gruppi locali (#152, slice 1 del task 6):
	// registrazione delegata ai file dedicati.
	h.registerDomainReadRoutes(mux, protect)
	h.registerGroupRoutes(mux, protect)

	// Operazioni massive del workspace (#153, slice 2 del task 6):
	// registrazione delegata al file dedicato.
	h.registerBulkRoutes(mux, protect)

	// Percorsi formativi e valutazioni di competenza (#161, slice 2 del
	// task 7): registrazione delegata ai file dedicati.
	h.registerPathRoutes(mux, protect)
	h.registerAssessmentRoutes(mux, protect)

	// Report: consuntivo economico ed erogato (#163, slice 4 del task 7):
	// registrazione delegata al file dedicato.
	h.registerReportRoutes(mux, protect)
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
